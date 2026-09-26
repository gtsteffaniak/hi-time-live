# Signaling protocol

The server never touches media. It is a room-scoped message router with two
endpoints, and everything below is enforced server side and covered by tests in
`routes/*_test.go`.

## Transport

| Direction | Endpoint | Notes |
| --- | --- | --- |
| server -> client | `GET /events?userId=<id>&code=<uuid>` | SSE stream, one per tab |
| client -> server | `POST /event` | JSON body, max 256 KiB |

`code` must be a UUID (`validCode`); `userId` must be non-empty. Either missing
or malformed gives `400` before any room state is touched.

## Message envelope

Every message in both directions is one JSON object:

```jsonc
{
  "eventType": "newOffer",   // required
  "userId":    "alice",      // sender
  "code":      "<room uuid>",// room the sender claims
  "forUser":   "bob",        // required for newOffer/answer/candidate
  "offer":     "<sdp>",
  "answer":    "<sdp>",
  "candidate": "<json ICE candidate>",   // trickled, one per message
  "mediaState": "{\"audio\":true,\"video\":false}" // presence, room-scoped
}
```

## Event types

Client -> server (anything else is `400`, so the stream cannot be used as a
generic room chat bus):

| eventType | Routed to | Requirements |
| --- | --- | --- |
| `newOffer` | every stream of `forUser` in `code` | `forUser` set, sender live in `code` |
| `answer` | every stream of `forUser` in `code` | `forUser` set, sender live in `code` |
| `candidate` | every stream of `forUser` in `code` | `forUser` set, sender live in `code` |
| `mediaState` | every *other* participant in `code` | sender live in `code` |

Offers are *addressed*, never broadcast: if a broadcast offer were answered by a
participant it was not meant for, that answer would corrupt the intended
connection's remote description (observed: a peer answering another pair's
offer leaves both ends stuck at `connectionState: connecting`).

Server -> client:

| eventType | Meaning |
| --- | --- |
| `acknowledge` | stream established; sent before any room state changes |
| `newUser` | a new participant joined; existing peers create the offer |
| `removedUser` | the last stream of a participant closed |
| `newOffer` / `answer` / `candidate` / `mediaState` | forwarded verbatim from another participant |

## ICE

Candidates trickle: each `onicecandidate` result is posted as its own
`candidate` event instead of waiting for `iceGatheringState === 'complete'` and
bundling a list into the SDP. A candidate can beat the offer that created it,
so receivers queue candidates until their peer connection has a remote
description (and until the peer exists at all). The legacy `candidates` array
field on `offer`/`answer` is still parsed for compatibility but no longer sent.

A `: keepalive` comment is written every 20s so idle streams survive proxies.

## Rules

1. **Room isolation.** A message is only ever delivered to connections whose
   `roomId` matches the sender's. Cross-room delivery is impossible by
   construction, not by convention.
2. **Sender authorization.** `POST /event` requires an open SSE stream for that
   `userId` in that room; otherwise `403`. A third party cannot inject offers
   into a call it is not in.
3. **Offer direction.** The *existing* participants offer to the joiner. The
   joiner only answers. This keeps glare out of the mesh: for N participants a
   joiner receives N-1 offers and sends N-1 answers.
4. **Reconnects and duplicate tabs.** Connections are keyed by connection id,
   not user id, so a reload briefly has two streams for one user. `removedUser`
   is announced only when the *last* stream for that user closes, and a repeat
   join is treated as a reconnect rather than a new participant.
5. **Slow clients.** Each stream has a 32-message buffer; a client that cannot
   keep up loses messages instead of stalling every other participant behind the
   connection lock.
6. **Room lifecycle.** Rooms are created on first join and deleted when the last
   participant leaves, so the room map cannot grow without bound.

## Join sequence

```
alice: GET /events  -> acknowledge
bob:   GET /events  -> acknowledge
alice:              <- newUser(bob)
alice: POST /event newOffer(forUser=bob)   -> bob receives newOffer(alice)
alice: POST /event candidate(forUser=bob)  -> bob receives candidate(alice)   (repeats)
bob:   POST /event answer(forUser=alice)   -> alice receives answer
bob:   POST /event candidate(forUser=alice)-> alice receives candidate(bob)   (repeats)
bob closes stream
alice:              <- removedUser(bob)
```
