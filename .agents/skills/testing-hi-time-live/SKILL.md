---
name: testing-hi-time-live
description: How to run UI-driven end-to-end tests of the hi-time-live WebRTC app on this machine (fake media, multi-process browsers, diagnostics, known quirks).
---

# Testing hi-time-live (WebRTC signaling + mesh calls)

## Running the app
- `PATH=/usr/local/go/bin:$PATH go run . --dev --port <PORT>`; without `cert.pem`/`key.pem` it falls back to HTTP. `http://localhost:<PORT>` is a secure context so `getUserMedia` works.
- `--dev` hot-reloads templates: if the file tree is being edited mid-run, open pages keep stale JS. Reload all peers after code changes.
- Pick a port nobody else uses; the Playwright suite in `tests/e2e` may already be running its own server.

## Multi-peer browsers with identifiable fake media
- Chrome's fake-media flags are process-wide: launch a separate Chromium process per peer with Playwright `headless:false`, args:
  `--use-fake-device-for-media-stream --use-fake-ui-for-media-stream --autoplay-policy=no-user-gesture-required --disable-features=WebRtcHideLocalIpsWithMdns`
  plus `--use-file-for-fake-video-capture=<color>.y4m --use-file-for-fake-audio-capture=<tone>.wav` from `tests/e2e/.media/` (red/green/blue/yellow + distinct tones, see `tests/e2e/lib/media.ts`).
- Wire `page.on('console')`, `page.on('pageerror')`, and `page.on('response')` to JSONL files — every pageerror matters (e.g. a `TypeError: null.addEventListener` fires on the home page because `checkForTextContent` binds `#nameInput` which only exists on the room page).
- Switch between peer windows with `wmctrl -ia <winid>` (list via `wmctrl -lp`); each peer browser is a separate process.
- Read-only diagnostics via `page.evaluate`: remote tiles are `#video-container video` (id `<peer>-remoteVideo`), per-peer connections are in the top-level `pcs` object (connectionState + `getStats()` inbound-rtp `framesDecoded`/`totalAudioEnergy`), and a canvas drawImage gives the tile's center pixel to verify *which* peer's color is rendered.

## App quirks to know
- Join flow: home → "Create a new room" → `/room?id=<uuid>` → privacy modal → type name in `#nameInput` → Start (`#start-button` appears only after non-empty input). Same flow for join via URL.
- `startSession()` generates a fresh `name__<uuid>` userId each time — two tabs can share a *name* but never an exact userId via the UI.
- The bottom controls (Mute / Disable Video / Leave) are toggled by `showControls()` from `updateContainerClass` on every join/leave/resize and the only manual toggle (`#ctab`) is `display:none` above 800px — they can be stuck hidden on desktop. If hidden, evaluate `showControls()` once, then click the real buttons.
- Video tiles have a black letterbox band at top; sample the *center* pixel for the peer color.
- Signaling latency is ~5s per join (ICE gather timeout); allow ~15-25s settle before asserting.

## Devin Secrets Needed
- None — the app has no auth.
