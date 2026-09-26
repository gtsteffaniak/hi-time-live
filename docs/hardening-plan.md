# Hardening plan

Goal: a signaling contract that is explicit enough to test, and tests that fail
for real product reasons rather than DOM coincidences.

## Done in this change

### Server

- Protocol written down (`docs/signaling-protocol.md`) and enforced: unknown
  event types, malformed room codes and missing user ids are rejected at the
  edge.
- Room isolation and sender authorization are structural (`403` for a sender
  with no stream in the room it posts to) rather than implicit.
- SSE streams are keyed per connection, bounded (32 messages, non-blocking
  send), heartbeated every 20s, and announce `removedUser` only when a user's
  last stream closes — so a reload no longer evicts the reloading user from the
  other participants' UIs.
- Room map entries are deleted when empty; `attemptJoin` distinguishes a first
  join from a reconnect so the mesh is not re-offered on every tab refresh.
- `invalidRoom.html` rendered a misspelled partial and therefore served an empty
  200. It now renders and returns `404`.
- Routes are constructed by `NewRouter()` so tests can serve the real mux.

### Tests

- Go: room lifecycle, concurrent join/leave under `-race`, input validation,
  offer/answer routing, cross-room leakage, duplicate tabs, disconnect
  announcements, slow clients, oversized payloads, page rendering.
- Playwright (`tests/e2e`): one Chromium *process* per peer (Chrome's fake-media
  flags are process-wide), each fed a deterministic solid-colour Y4M video and a
  distinct sine-wave WAV. That makes identity assertable: the remote tile's
  centre pixel must be the right colour and its dominant FFT frequency the right
  tone, so a mis-routed stream fails instead of passing as "a video element
  exists".
- Liveness comes from `getStats` (`framesDecoded`, `bytesReceived`,
  `totalAudioEnergy`) and `RTCPeerConnection.connectionState`, not from
  timeouts.

Covered scenarios: two-peer call with media identity, peer leave, three-peer
full mesh, leave from a three-way call, room isolation, invalid room, reload and
rejoin, aborted signaling requests, unauthorized sender, mute, video disable.

## Next steps (not in this change)

1. **Frontend state machine.** `templates/js/signaling.js` now keeps per-peer
   candidate lists and an ordered offer/answer path, but state is still implicit
   in module-level `pcs`. A per-peer object
   (`new` -> `offering`/`answering` -> `connected` -> `closed`) would make
   late/duplicate messages droppable on purpose rather than by accident.
2. **Trickle ICE.** Candidates are still batched into the offer/answer after a
   gather wait. A `candidate` event type would cut setup latency and remove the
   sleep.
3. **Error surfacing.** `sendEvent` now logs non-2xx responses and updates the
   status line, but there is no SSE retry/backoff — a dropped stream still
   leaves the user silently disconnected.
4. **Explicit media state.** Mute/disable only flips `track.enabled`, so the
   remote side sees silence and black rather than a labelled state. A
   `mediaState` event would let the UI show who is muted.
5. **Controls can get stuck hidden on desktop.** `updateContainerClass` toggles
   `fly-in` on every join/leave/resize, and `#ctab` is `display:none` above
   800px, so once the controls toggle off there is no clickable way back —
   observed during manual UI testing. Show them unconditionally on desktop or
   make the toggle state a single source of truth.
6. **Test matrix.** Firefox peers (Playwright supports its own fake-media
   prefs), simulated packet loss/renegotiation, and a soak test for room-map
   growth. Two further UI-level items already verified manually are good
   candidates to automate: duplicate same-name tabs, and mute/video-off
   propagation to a *second* remote peer.
