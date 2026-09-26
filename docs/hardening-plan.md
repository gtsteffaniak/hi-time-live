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

## Done after the initial PR

- **Frontend state machine.** `templates/js/signaling.js` now keeps a `peers`
  map of per-peer records (`pc`, `state`, `pendingCandidates`, `mediaState`)
  with explicit `new -> offering|answering -> connected -> closed` transitions.
  Duplicate offers and answers for a peer already past that stage are dropped
  deliberately.
- **Trickle ICE.** A `candidate` event type carries each candidate as it is
  found; the gather wait is gone. Candidates that beat their offer are parked
  in `earlyCandidates`/`pendingCandidates` until the remote description lands.
- **SSE retry/backoff.** A `CLOSED` stream is restarted with exponential
  backoff (1s doubling to 30s); `CONNECTING` errors report "reconnecting".
  `mediaState` is re-broadcast on `onopen` so peers relearn mute state.
- **Explicit media state.** A room-scoped `mediaState` event broadcasts
  `{audio, video}`; remote overlays render "name (muted)" / "name (video off)".
  Existing peers broadcast when a `newUser` joins so the joiner sees the
  current state.
- **Controls stuck hidden.** `setControlsVisible(visible)` replaced the blind
  `fly-in` toggle; desktop shows controls unconditionally once a remote tile
  exists, and `#ctab` toggling only applies on narrow screens where it renders.
- **Tests.** Four additions: duplicate same-name tabs as a second participant;
  mute/video-off propagation to *every* remote peer (overlay label + flat
  audio energy + black tile); a Firefox<->Chromium smoke spec (fake media via
  `media.navigator.streams.fake` prefs — this caught that filtering the codec
  list to VP9/H264 left Firefox builds without OpenH264 sending no video, so
  VP8 is back on the allowed list); and a Go churn test asserting the room and
  connection maps return to empty after repeated join/leave cycles.

## Next steps (not in this change)

1. **Test matrix.** Simulated packet loss/renegotiation under throttle (e.g.
   Playwright's connection emulation or a proxy), and longer soak runs.
2. **Renegotiation.** Track add/remove mid-call (e.g. `switchMedia`) still has
   no signaling path; needs an offer/answer round guarded by the peer state
   machine.
