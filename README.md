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

A deployed version is available at https://hitime.live/.

you can also deploy your own server using the docker image provided at dockerhub with `gtstef/hitime` or building locally.

Note: The frontend features require an HTTPS connection, so any build should be done behind one. I have included the `generate_cert.go` standard library for quick mock certificate creation for local testing.

## Browser Support

WebRTC has broad browser support

![Chrome](https://raw.githubusercontent.com/alrra/browser-logos/master/src/chrome/chrome_48x48.png) | ![Firefox](https://raw.githubusercontent.com/alrra/browser-logos/master/src/firefox/firefox_48x48.png) | ![IE](https://raw.githubusercontent.com/alrra/browser-logos/master/src/edge/edge_48x48.png) | ![Opera](https://raw.githubusercontent.com/alrra/browser-logos/master/src/opera/opera_48x48.png) | ![Safari](https://raw.githubusercontent.com/alrra/browser-logos/master/src/safari/safari_48x48.png) 
--- | --- | --- | --- | --- |
 ✅ |  ✅ | ✅ | ✅ | ✅ |

