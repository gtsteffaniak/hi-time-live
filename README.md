# Hi time live

<p align="center">
  <img width="500" src="./static/img/hitime.png" title="Main logo">
</p>
<p align="center">
    A free peer-2-peer WebRTC based video conferencing.
</p>

## What is it?

This video conferencing software combines three technologies to create an all-in-one web application:

1. WebRTC with html/JavaScript in the client-side browser to handle the user interface.
2. Continuous Server Sent Event communication channel for realtime updates.
3. Signaling server to facilitate the connection information, written in go.

<p align="center">
  <img width="500" src="https://lh3.googleusercontent.com/tn1h7nq5-ANzEyuwISMNLqFngijegUKAAfIkqoy76lg3ewxnI2wDGBtA29vIgp96CyivhVOEuh_OkX7jjAc_e4r-_m5LpZStO8Bxc3VFvOL-XVEB51mnOJSzrnXwzpHGE-DFsq6w" title="WebRTC">
</p>

Advantages of peer-2-peer connection-oriented video conferencing:

- Privacy for the users since none of the video or connection information is stored on a server.
- Latency on videos will be ideal because video routing and processing does not use an intermediary server.
- Reduced management expense for the server, since the server merely forwards messages and does not handle live video or store information in memory.

Disadvantages:

- Higher bandwidth and CPU usage, since the client must process more video streams from each peer.
- Fewer centralized features, such as effects or controls.
- Requires complicated and somewhat error-prone communication between clients, each other, and the backend. 

## How to use it

A deployed version is available at https://gportal.link/hitime.

you can also deploy your own server using the docker image provided at dockerhub with `gtstef/hitime` or building locally.

### Serving under a URL prefix

To run behind a reverse proxy at a sub path (e.g. `https://example.com/hitime/`),
set `BASE_PATH=/hitime` (or pass `--base-path /hitime`). The app then serves
everything under `/hitime/`, redirects `/hitime` to `/hitime/`, and returns 404
for paths outside the prefix. Proxy the location through unchanged, without
stripping the prefix, and keep buffering off for the `/events` SSE stream:

```nginx
location /hitime/ {
    proxy_pass http://hitime:9012;
    proxy_http_version 1.1;
    proxy_buffering off;
    proxy_read_timeout 1h;
}
```

Note: The frontend features require an HTTPS connection, so any build should be done behind one. I have included the `generate_cert.go` standard library for quick mock certificate creation for local testing.

### ICE and TURN configuration

Browsers need STUN to discover public addresses and TURN to relay media when a direct peer path is blocked (symmetric NAT, corporate firewalls, and similar). By default the app serves a small set of public Google STUN servers. To reach callers on restrictive networks, point the app at your own TURN relay.

| Setting | Flag | Description |
| --- | --- | --- |
| `ICE_SERVERS` | `--ice-servers` | **Dev only.** JSON array of `RTCIceServer` objects; replaces the built-in list entirely. |
| `TURN_URL` | `--turn-url` | Comma-separated TURN URL(s), e.g. `turn:turn.example.com:3478?transport=udp,turns:turn.example.com:5349`. Appended alongside the default STUN servers unless `ICE_SERVERS` is set. |
| `TURN_USERNAME` / `TURN_CREDENTIAL` | `--turn-username`, `--turn-credential` | **Dev only.** Static relay credentials embedded in each room page (anyone who can open a room URL can reuse them). |
| `TURN_REST_SECRET` | `--turn-rest-secret` | Shared secret for coturn-style time-limited credentials. Safe for production: the secret stays on the server and each room page gets a fresh username/password pair. Requires `TURN_URL`. |
| `TURN_REST_TTL` | `--turn-rest-ttl` | Lifetime of REST-minted credentials (default `24h`). |
| `TURN_REST_USER` | `--turn-rest-user` | Optional suffix in REST usernames (`<expiry>:<suffix>`). |

Example with a local coturn instance in development:

```bash
./hi-time-live --dev \
  --turn-url 'turn:localhost:3478' \
  --turn-username devuser \
  --turn-credential devpass
```

Example for production using REST credentials (configure the same shared secret in coturn with `use-auth-secret`):

```bash
export TURN_URL='turn:turn.example.com:3478?transport=udp'
export TURN_REST_SECRET='your-shared-secret'
./hi-time-live
```

The room page embeds the resulting ICE server list in a `#ice-config` JSON block; the client builds its `RTCPeerConnection` from that list.

## Testing

- `make test` runs the Go unit/integration tests (room lifecycle, signaling
  routing, isolation, disconnects) under the race detector.
- `make test-e2e` runs the Playwright suite in `tests/e2e`, which drives one
  Chromium process per peer with deterministic fake camera/microphone input and
  asserts real media over `getStats` and pixel/tone identity.
- `make test-e2e-base-path` runs the same suite against the app mounted under
  `/hitime` (`HITIME_BASE_PATH`), which is how it is deployed behind gportal.

The signaling contract is documented in [docs/signaling-protocol.md](docs/signaling-protocol.md),
and the remaining hardening work in [docs/hardening-plan.md](docs/hardening-plan.md).

## Browser Support

WebRTC has broad browser support

![Chrome](https://raw.githubusercontent.com/alrra/browser-logos/master/src/chrome/chrome_48x48.png) | ![Firefox](https://raw.githubusercontent.com/alrra/browser-logos/master/src/firefox/firefox_48x48.png) | ![IE](https://raw.githubusercontent.com/alrra/browser-logos/master/src/edge/edge_48x48.png) | ![Opera](https://raw.githubusercontent.com/alrra/browser-logos/master/src/opera/opera_48x48.png) | ![Safari](https://raw.githubusercontent.com/alrra/browser-logos/master/src/safari/safari_48x48.png) 
--- | --- | --- | --- | --- |
 ✅ |  ✅ | ✅ | ✅ | ✅ |

