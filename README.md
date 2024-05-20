# Hi time live

![Logo](./static/img/hitime.png)

A free peer-2-peer WebRTC based video conferencing.

## What is it?

This video conferencing software combines three technologies to create an all-in-one web application:

1. WebRTC with html/javascript in the client side browser to handle the user interface.
2. Websocket communication channel between browser and signaling server.
3. Signaling server to facilitate the connection information, written in go.

Advantages of a peer-2-peer connection oriented video conferencing:

- Privacy for the users since none of the video or connection information is stored on a server.
- Latency on videos will be ideal because video routing and processing does not use intermediary server.
- Reduced management expense for the server, since the server merely forwards messages and does not handle live video or store information in memory.

Disadvantages:

- Higher bandwidth usage, since the client must process more video streams from each peer.
- Fewer centralized features, such as effects or controls.

## How to use it

A deployed version is available at https://hitime.live/.

you can also deploy your own server using the docker image provided at dockerub with `gtstef/hitime` or building locally.

Note: The features on the frontend require HTTPS connection, so any build should be done behind a HTTPS connection. I have included `generate_cert.go` standard library for quick mock certificate creation for local testing.
