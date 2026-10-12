let eventSrc;
let localStream;

const videoConstraints = {
    audio: true,
    video: {
        facingMode: { ideal: 'user' }
    }
}
// VP8 stays on the list for interop: Firefox builds without OpenH264 cannot
// encode H264, and removing it would leave those peers sending no video at all.
const allowedCodecs = ['VP9', 'H264', 'VP8'];

// Rendered server-side from --ice-servers / --turn-url (see routes/ice.go) into
// the #ice-config element, so a deployment can add a TURN relay without
// rebuilding the frontend. STUN alone cannot traverse symmetric NAT or a
// restrictive firewall, which is why peers that "never connect" need a relay.
function readRenderedIceServers() {
    const element = document.getElementById('ice-config');
    if (!element) return [];
    try {
        const parsed = JSON.parse(element.textContent);
        return Array.isArray(parsed) ? parsed : [];
    } catch (err) {
        console.error("unparseable ICE configuration:", err);
        return [];
    }
}

const renderedIceServers = readRenderedIceServers();
const defaultIceServers = [
    { 'urls': 'stun:stun.l.google.com:19302' },
    { 'urls': 'stun:stun1.l.google.com:19302' },
    { 'urls': 'stun:stun2.l.google.com:19302' },
    { 'urls': 'stun:stun3.l.google.com:19302' },
    { 'urls': 'stun:stun4.l.google.com:19302' },
];
const configuration = {
    'iceServers': (Array.isArray(renderedIceServers) && renderedIceServers.length) ? renderedIceServers : defaultIceServers,
};

function iceServerUrls(server) {
    const urls = server && server.urls;
    if (Array.isArray(urls)) return urls;
    return typeof urls === 'string' ? [urls] : [];
}

// ?relay=1 forces relay-only ICE and ?noturn=1 drops TURN entries. Together they
// reproduce "this user's NAT admits no usable path" deterministically, which is
// how the e2e suite covers the never-connects failure and how a human can tell a
// TURN misconfiguration apart from a codec problem.
// ?connecttimeout=<ms> shortens the connect watchdog so tests fail fast.
const debugParams = new URLSearchParams(window.location.search);
if (debugParams.get('relay')) {
    configuration.iceTransportPolicy = 'relay';
}
if (debugParams.get('noturn')) {
    configuration.iceServers = configuration.iceServers.filter(
        (server) => !iceServerUrls(server).some((url) => /^turns?:/i.test(url))
    );
}
const parsedConnectTimeout = parseInt(debugParams.get('connecttimeout'), 10);
const connectTimeoutMs = Number.isFinite(parsedConnectTimeout)
    ? Math.min(Math.max(parsedConnectTimeout, 2000), 120000)
    : 20000;
// "disconnected" is often a Wi-Fi blip rather than a dead peer, so give ICE a
// chance to heal before restarting it, and give the restart a chance to land
// before declaring the peer failed.
const iceDisconnectGraceMs = 5000;
const maxIceRestarts = 2;

let localUserId = "notset"

// Each remote participant gets one peer record: the RTCPeerConnection plus where
// it is in the handshake, so late or duplicate signaling messages are dropped
// deliberately instead of corrupting a live connection.
// state: new -> offering|answering -> connected -> reconnecting -> failed|closed
const peers = {};
// A trickled candidate or mediaState can beat the offer that creates the peer;
// they are parked here and consumed when the peer record is created.
const earlyCandidates = {};
const earlyMediaState = {};
// SSE auto-reconnects while readyState is CONNECTING; a CLOSED stream is
// restarted manually with this backoff.
let sseRetryDelayMs = 1000;
const sseMaxRetryDelayMs = 30000;
// Signaling POSTs are queued while the event stream is down and flushed on
// reconnect, so an offer, answer or candidate is not lost to a transient outage.
let sseOpen = false;
const outboundQueue = [];
const outboundQueueLimit = 128;
let localVideo = document.getElementById('localVideo');


// The status bar is the room's live roster: one row per participant, pulsing
// while they are still connecting and steady once they settle. Every peer stays
// listed, so a join, a reconnect, a failure or a departure is visible as it
// happens instead of only in a spinner that disappears the moment media arrives.
//
// Collapse the internal handshake states into the four things a user cares
// about: `new`, `offering` and `answering` are all just "connecting".
function peerVisualState(peer) {
    switch (peer.state) {
        case "failed": return "failed";
        case "reconnecting": return "reconnecting";
        case "connected": return "connected";
        default: return "connecting";
    }
}

function peerStateText(state) {
    return state === "failed" ? "connection failed" : state;
}

function connectionRow(peer) {
    const row = document.createElement('div');
    row.id = peer.id + '-status-row';
    row.classList.add('conn-row');

    const state = peerVisualState(peer);

    const icon = document.createElement('span');
    icon.classList.add('conn-icon', 'conn-' + state);

    const name = document.createElement('span');
    name.classList.add('conn-name');
    name.textContent = peer.id.split("__")[0];

    const label = document.createElement('span');
    label.classList.add('conn-state');
    label.textContent = peerStateText(state);

    row.appendChild(icon);
    row.appendChild(name);
    row.appendChild(label);

    if (peer.state === "failed") {
        const retry = document.createElement('button');
        retry.classList.add('conn-retry');
        retry.textContent = 'Retry';
        retry.onclick = () => retryPeer(peer.id);
        row.appendChild(retry);
    }
    return row;
}

function renderConnectionBar() {
    const bar = document.getElementById('connection-bar');
    if (!bar) return;
    bar.innerHTML = '';
    for (const peer of Object.values(peers)) {
        bar.appendChild(connectionRow(peer));
    }
    bar.classList.toggle("hidden", bar.children.length === 0);
}

function createRemoteVideoStream(id) {
    // Create the container div element
    const containerDiv = document.createElement('div');
    containerDiv.id = id + '-container';
    containerDiv.classList.add("remote-views")

    // Create the video element
    const videoElement = document.createElement('video');
    videoElement.id = id + '-remoteVideo';
    videoElement.muted = true; // for safari to autoplay
    videoElement.autoplay = true;
    videoElement.playsinline = true;

    const videoOverlay = document.createElement('div');
    videoOverlay.id = id + '-video-overlay';
    videoOverlay.classList.add("video-overlay")

    // Append the video element to the container div
    containerDiv.appendChild(videoElement);
    containerDiv.appendChild(videoOverlay);

    // Append the container div to the main video container
    const videoContainer = document.getElementById('video-container');
    videoContainer.appendChild(containerDiv);
}

// Helper function to attach media stream to the video element
async function attachMediaStream(video, stream, id) {
    try {
        // Use Safari-friendly attachment logic
        video.srcObject = stream;
        try {
            await video.play(); // Ensure video playback starts
            console.log("Remote video is playing for:", id);
            video.muted = false; // Unmute after video starts
        } catch (playError) {
            console.error("Error playing the remote video:", playError);
        }
        updateContainerClass();
    } catch (error) {
        console.error("Error attaching media stream:", error);
    }
}

function updatePeerOverlay(id) {
    const overlay = document.getElementById(id + '-video-overlay');
    if (!overlay) return;
    const name = id.split("__")[0];
    const peer = peers[id];
    const state = peer && peer.mediaState;
    const flags = [];
    if (state && !state.audio) flags.push("muted");
    if (state && !state.video) flags.push("video off");
    if (peer && peer.state === "failed") flags.push("connection failed");
    if (peer && peer.state === "reconnecting") flags.push("reconnecting");
    overlay.innerHTML = "<p>" + name + (flags.length ? " (" + flags.join(", ") + ")" : "") + "</p>";
}

function updateContainerClass() {
    const videoContainer = document.getElementById('video-container');
    const childrenCount = videoContainer.children.length;
    videoContainer.classList.remove('one');
    videoContainer.classList.remove('two');

    if (childrenCount > 0) {
        videoContainer.classList.remove('hidden');
        if (window.innerWidth > 800) {
            videoContainer.classList.add("padding-bottom")
            setControlsVisible(true)
        }
    } else {
        videoContainer.classList.add('hidden');
    }
    if (childrenCount === 1) {
        videoContainer.classList.add('one');
        return;
    }
    if (childrenCount === 2) {
        if (window.innerWidth > 800) {
            videoContainer.classList.add('two');
        } else {
            videoContainer.classList.add('one');
        }
    }
    if (childrenCount === 4 || childrenCount === 3) {
        videoContainer.classList.add('two');
        return;
    }
}


window.addEventListener("resize", updateContainerClass);

function hasRemoteVideos() {
    const videoContainer = document.getElementById('video-container');
    return videoContainer.getElementsByTagName('video').length > 0;
}

function removeRemoteVideoStream(id) {
    const containerDiv = document.getElementById(id + '-container');
    if (containerDiv) {
        containerDiv.remove(); // Removes the container div from the DOM
    }
    updateContainerClass();
    // "Waiting on others to join" is only true when nobody else is here. A peer
    // that failed keeps its status row (with a retry) instead of silently
    // collapsing back to the empty-room prompt.
    if (!hasRemoteVideos() && Object.keys(peers).length === 0) {
        updateStatusText("Waiting on others to join");
        const loadingModal = document.getElementById('loadingModal');
        loadingModal.classList.remove("hidden");
    }
}

// Function to filter the codecs in the SDP
function filterCodecs(sdp, allowedCodecs) {
    const sdpLines = sdp.split('\r\n');
    let isVideoSection = false;
    const videoMLineIndex = sdpLines.findIndex(line => line.startsWith('m=video'));

    if (videoMLineIndex === -1) return sdp; // No video section found

    let mLineParts = sdpLines[videoMLineIndex].split(' ');
    let filteredPayloadTypes = [];

    // Regex to match the allowed codecs
    const codecRegex = new RegExp(`^a=rtpmap:(\\d+) (${allowedCodecs.join('|')})\\/\\d+`, 'i');

    // Iterate over the SDP lines to find the allowed payload types
    for (let i = videoMLineIndex + 1; i < sdpLines.length; i++) {
        if (sdpLines[i].startsWith('m=')) {
            break; // End of the video section
        }

        const match = sdpLines[i].match(codecRegex);
        if (match) {
            filteredPayloadTypes.push(match[1]); // Capture the payload type
        }
    }

    if (filteredPayloadTypes.length === 0) return sdp; // No matching codecs found

    // Update the m= line with the filtered payload types
    mLineParts = mLineParts.slice(0, 3).concat(filteredPayloadTypes);
    sdpLines[videoMLineIndex] = mLineParts.join(' ');

    // Filter out irrelevant lines
    const filteredSdpLines = sdpLines.filter(line => {
        if (line.startsWith('m=') || line.startsWith('c=') || line.startsWith('a=sendrecv') || line.startsWith('a=recvonly') || line.startsWith('a=sendonly') || line.startsWith('a=inactive')) {
            return true;
        }

        const fmtpMatch = line.match(/^a=fmtp:(\d+)/);
        const rtcpFbMatch = line.match(/^a=rtcp-fb:(\d+)/);

        if (fmtpMatch && filteredPayloadTypes.includes(fmtpMatch[1])) {
            return true;
        }

        if (rtcpFbMatch && filteredPayloadTypes.includes(rtcpFbMatch[1])) {
            return true;
        }

        if (line.startsWith('a=rtpmap:')) {
            const parts = line.split(' ');
            const payloadType = parts[0].split(':')[1];
            return filteredPayloadTypes.includes(payloadType);
        }

        return !line.startsWith('a=rtpmap:') && !line.startsWith('a=rtcp-fb:') && !line.startsWith('a=fmtp:');
    });

    return filteredSdpLines.join('\r\n');
}


// addLocalTracks reuses a single capture for every peer connection, so the mute
// and video controls act on the tracks all peers actually receive.
async function addLocalTracks(peer) {
    localVideo = document.getElementById('localVideo');
    if (!localStream) {
        localStream = await navigator.mediaDevices.getUserMedia(videoConstraints);
        localVideo.srcObject = localStream;
    }
    // A reset or retried peer reuses its record; re-adding tracks to a
    // connection that already has senders would duplicate every transceiver.
    if (peer.pc.getSenders().length > 0) return;
    localStream.getTracks().forEach((track) => peer.pc.addTrack(track, localStream));
}

// wirePeer attaches the event handlers to a peer's current RTCPeerConnection.
// Split out so an ICE restart or retry can swap in a fresh connection without
// rebuilding the peer record or its tile.
function wirePeer(peer) {
    peer.pc.onicecandidate = ({ candidate }) => {
        if (candidate && peers[peer.id]) {
            sendEvent({
                eventType: "candidate",
                userId: localUserId,
                forUser: peer.id,
                candidate: JSON.stringify(candidate),
                code: "{{ .code }}",
            })
        }
    };
    peer.pc.onconnectionstatechange = () => handleConnectionState(peer);
    peer.pc.oniceconnectionstatechange = () => handleIceState(peer);
    peer.pc.ontrack = (event) => {
        console.log("ontrack event:", peer.id, event);
        const remoteStream = event.streams[0]; // Get the remote stream
        const remoteVideo = document.getElementById(peer.id + '-remoteVideo');
        if (!remoteVideo) return;

        if (remoteVideo.srcObject) return; // Prevent redundant attachment

        console.log("Attaching remote view: ", peer.id, "to stream:", remoteStream);
        attachMediaStream(remoteVideo, remoteStream, peer.id);
    };
}

// newPeer registers a peer record and creates its remote tile. Candidates
// trickle out one event at a time as they are found rather than waiting for
// gathering to complete and bundling them into the SDP.
function newPeer(id) {
    if (peers[id]) {
        return peers[id]
    }
    console.log("adding new peer:", id)
    const peer = {
        id: id,
        pc: new RTCPeerConnection(configuration),
        state: "new",
        // Exactly one side of a pair is impolite, so a colliding renegotiation
        // (both sides restarting ICE at once) has a deterministic winner.
        polite: localUserId > id,
        pendingCandidates: earlyCandidates[id] || [],
        mediaState: earlyMediaState[id],
        restartCount: 0,
        connectTimer: undefined,
    };
    peers[id] = peer;
    delete earlyCandidates[id];
    delete earlyMediaState[id];
    wirePeer(peer);
    createRemoteVideoStream(id)
    startConnectWatchdog(peer)
    updatePeerOverlay(id)
    renderConnectionBar()
    return peer
}

// resetPeer replaces a peer's dead RTCPeerConnection while keeping its record
// and tile, so a retry or a remote reconnect does not remove the user's view.
function resetPeer(peer) {
    clearConnectWatchdog(peer);
    try {
        peer.pc.close();
    } catch (err) {
        console.warn("closing peer connection:", peer.id, err);
    }
    const remoteVideo = document.getElementById(peer.id + '-remoteVideo');
    if (remoteVideo) remoteVideo.srcObject = null;
    peer.pc = new RTCPeerConnection(configuration);
    peer.state = "new";
    peer.restartCount = 0;
    peer.pendingCandidates = [];
    wirePeer(peer);
    updatePeerOverlay(peer.id);
    renderConnectionBar();
}

function closePeer(id) {
    const peer = peers[id];
    if (peer) {
        clearConnectWatchdog(peer);
        peer.state = "closed";
        peer.pc.close();
        delete peers[id];
    }
    removeRemoteVideoStream(id)
    renderConnectionBar()
    console.log("closed video of peer: ", id)
}

// failPeer marks a peer unusable but keeps its tile and status row so the user
// sees what happened and can retry, rather than a silent return to the
// empty-room prompt.
function failPeer(peer, reason) {
    if (peer.state === "closed" || peer.state === "failed") return;
    console.warn("peer failed:", peer.id, reason);
    peer.state = "failed";
    peer.lastError = reason;
    clearConnectWatchdog(peer);
    updatePeerOverlay(peer.id);
    renderConnectionBar();
    logIceDiagnostics(peer);
}

function startConnectWatchdog(peer) {
    clearConnectWatchdog(peer);
    peer.connectTimer = setTimeout(() => {
        if (peers[peer.id] !== peer) return;
        if (peer.state === "connected" || peer.state === "failed") return;
        failPeer(peer, "timed out connecting");
    }, connectTimeoutMs);
}

function clearConnectWatchdog(peer) {
    if (peer.connectTimer !== undefined) {
        clearTimeout(peer.connectTimer);
        peer.connectTimer = undefined;
    }
}

function handleConnectionState(peer) {
    const state = peer.pc.connectionState;
    console.log("peer connection state:", peer.id, state);
    if (state === "connected") {
        peer.state = "connected";
        peer.restartCount = 0;
        clearConnectWatchdog(peer);
        updatePeerOverlay(peer.id);
        renderConnectionBar();
        logIceDiagnostics(peer);
        return;
    }
    if (state === "failed") {
        // A failed pair can still be rescued by an ICE restart (fresh ufrag and
        // pwd re-open connectivity checks), so try before giving up on it.
        attemptIceRestart(peer, "connection failed");
    }
}

function handleIceState(peer) {
    const state = peer.pc.iceConnectionState;
    console.log("ice connection state:", peer.id, state);
    if (state === "disconnected" && peer.state === "connected") {
        peer.state = "reconnecting";
        updatePeerOverlay(peer.id);
        renderConnectionBar();
        setTimeout(() => {
            if (peers[peer.id] !== peer) return;
            if (peer.pc.iceConnectionState === "disconnected" || peer.pc.connectionState === "failed") {
                attemptIceRestart(peer, "ice disconnected");
            }
        }, iceDisconnectGraceMs);
    }
    if ((state === "connected" || state === "completed") && peer.state === "reconnecting") {
        peer.state = "connected";
        updatePeerOverlay(peer.id);
        renderConnectionBar();
    }
}

// attemptIceRestart re-offers with a fresh ICE ufrag/pwd. Both sides may try at
// once; the polite side yields in answerPeer so the two never glare.
async function attemptIceRestart(peer, reason) {
    if (peer.state === "closed" || peer.state === "failed") return;
    if (peer.restartCount >= maxIceRestarts) {
        failPeer(peer, reason);
        return;
    }
    peer.restartCount += 1;
    peer.state = "reconnecting";
    updatePeerOverlay(peer.id);
    renderConnectionBar();
    console.log("restarting ICE with", peer.id, "attempt", peer.restartCount, "reason:", reason);
    try {
        await offerPeer(peer, { iceRestart: true });
    } catch (err) {
        console.error("ICE restart failed for", peer.id, err);
        failPeer(peer, "ice restart failed");
    }
}

async function retryPeer(id) {
    const peer = peers[id];
    if (!peer) return;
    console.log("retrying peer", id);
    resetPeer(peer);
    startConnectWatchdog(peer);
    try {
        await addLocalTracks(peer);
        await offerPeer(peer);
    } catch (err) {
        console.error("retry failed for", id, err);
        failPeer(peer, "retry failed");
    }
}

// logIceDiagnostics prints the nominated candidate pair and its candidate types,
// which is what distinguishes "no relay configured" from "codec mismatch" when
// a peer never connects.
async function logIceDiagnostics(peer) {
    try {
        const stats = await peer.pc.getStats();
        let pair = null;
        const candidates = new Map();
        stats.forEach((report) => {
            if (report.type === "local-candidate" || report.type === "remote-candidate") {
                candidates.set(report.id, report);
            }
            if (report.type === "candidate-pair" && (report.selected || report.nominated)) {
                pair = report;
            }
        });
        if (!pair) {
            console.log("ice diagnostics:", peer.id, "no nominated candidate pair");
            return;
        }
        const local = candidates.get(pair.localCandidateId);
        const remote = candidates.get(pair.remoteCandidateId);
        console.log("ice diagnostics:", peer.id, {
            pairState: pair.state,
            local: local ? `${local.candidateType} ${local.protocol} ${local.address}:${local.port}` : "unknown",
            remote: remote ? `${remote.candidateType} ${remote.protocol} ${remote.address}:${remote.port}` : "unknown",
        });
    } catch (err) {
        console.warn("ice diagnostics failed for", peer.id, err);
    }
}

async function offerPeer(peer, options = {}) {
    if (!options.iceRestart) {
        updateStatusText("Attempting to connect to new user")
    }
    peer.state = "offering";
    startConnectWatchdog(peer);
    renderConnectionBar();
    const offer = await peer.pc.createOffer(options.iceRestart ? { iceRestart: true } : undefined);
    offer.sdp = filterCodecs(offer.sdp, allowedCodecs);
    await peer.pc.setLocalDescription(offer);
    if (peer.state === "closed") {
        return
    }
    console.log("sending offer to ", peer.id, options.iceRestart ? "(ice restart)" : "")
    sendEvent({
        eventType: "newOffer",
        userId: localUserId,
        forUser: peer.id,
        offer: peer.pc.localDescription.sdp,
        renegotiate: options.iceRestart ? true : undefined,
        code: "{{ .code }}",
    })
}

async function answerPeer(peer, msg) {
    console.log("handling offer from", peer.id)
    // Glare: both sides restarted at once. The polite side yields and accepts
    // the remote offer; the impolite side keeps its own and ignores this one.
    if (peer.pc.signalingState === "have-local-offer") {
        if (!peer.polite) {
            console.log("ignoring colliding offer from", peer.id)
            return
        }
        console.log("rolling back local offer to accept", peer.id)
        await peer.pc.setLocalDescription({ type: "rollback" });
    }
    peer.state = "answering";
    startConnectWatchdog(peer);
    renderConnectionBar();
    await peer.pc.setRemoteDescription(new RTCSessionDescription({ "type": "offer", "sdp": msg.offer }));
    for (const candidate of bundledCandidates(msg)) {
        await addCandidate(peer, candidate)
    }
    await flushPendingCandidates(peer)
    const answer = await peer.pc.createAnswer();
    answer.sdp = filterCodecs(answer.sdp, allowedCodecs);
    await peer.pc.setLocalDescription(answer);
    if (peer.state === "closed") {
        return
    }
    // A renegotiation can complete while the connection is already up (the
    // remote restarted ICE, but our own connectionState never changed), so no
    // state event will arrive to clear "answering". Settle it now instead of
    // letting the connect watchdog declare a healthy peer failed.
    if (peer.pc.connectionState === "connected") {
        handleConnectionState(peer);
    }
    console.log("sending answer to ", peer.id)
    sendEvent({
        eventType: "answer",
        userId: localUserId,
        forUser: peer.id,
        answer: peer.pc.localDescription.sdp,
        code: "{{ .code }}",
    })
    hideLoadingModal()
}

function hideLoadingModal() {
    document.getElementById('loadingModal').classList.add("hidden")
}

function sendEvent(msg) {
    // The event stream carries every offer, answer and candidate; while it is
    // down, queue instead of dropping so a transient outage does not silently
    // lose a connection's signaling.
    if (!sseOpen) {
        if (outboundQueue.length >= outboundQueueLimit) {
            outboundQueue.shift();
            console.warn("outbound signaling queue full, dropping oldest event");
        }
        outboundQueue.push(msg);
        return;
    }
    postEvent(msg);
}

function postEvent(msg) {
    fetch("event", {
        method: "POST",
        headers: {
            "Content-Type": "application/json"
        },
        body: JSON.stringify(msg)
    }).then(response => {
        if (!response.ok) {
            console.error('signaling rejected', msg.eventType, response.status);
            updateStatusText("Connection problem, try rejoining");
        }
    }).catch(error => console.error('Fetch error:', error));
}

function flushOutboundQueue() {
    const queued = outboundQueue.splice(0, outboundQueue.length);
    for (const msg of queued) {
        postEvent(msg);
    }
}

// startOffer is the existing-participant side of a join: create a peer record
// and offer. startAnswer is the joiner side: the joiner only ever answers.
// See docs/signaling-protocol.md.
async function startOffer(id) {
    const existing = peers[id];
    if (existing && (existing.state === "offering" || existing.state === "answering")) {
        console.log("skipping offer, peer already negotiating:", id, existing.state)
        return
    }
    if (existing && existing.state === "connected") {
        // A newUser for a peer we are happily talking to is a stale reconnect
        // artifact; renegotiating a healthy connection would only disturb it.
        console.log("skipping offer, peer already connected:", id)
        return
    }
    let peer = existing;
    if (!peer) {
        peer = newPeer(id)
    } else if (peer.state === "failed" || peer.state === "reconnecting") {
        resetPeer(peer)
    }
    await addLocalTracks(peer)
    await offerPeer(peer)
}

async function startAnswer(id, msg) {
    let peer = peers[id];
    if (peer) {
        if (!msg.renegotiate) {
            if (peer.state === "offering" || peer.state === "answering") {
                // A duplicate or late offer must not renegotiate a live connection.
                console.log("dropping offer, peer already", peer.state, ":", id)
                return
            }
            if (peer.state !== "new") {
                // The remote side rebuilt its connection (it reconnected or was
                // retried) while ours still held the old one. A fresh offer
                // means the old connection is gone, so replace it rather than
                // deadlock on a stale record.
                console.log("remote restarted negotiation, resetting peer", id)
                resetPeer(peer)
            }
        }
    } else {
        peer = newPeer(id)
    }
    await addLocalTracks(peer)
    await answerPeer(peer, msg)
}

function startSSE() {
    eventSrc = new EventSource(`events?userId=${localUserId}&code={{ .code }}`);

    eventSrc.onopen = () => {
        console.log("SSE connection established.");
        sseOpen = true;
        sseRetryDelayMs = 1000;
        flushOutboundQueue();
        // A reconnect is invisible to the room (the server only announces a
        // user when their last stream closes), but peers may have missed our
        // media state while the stream was down.
        broadcastMediaState();
    };

    eventSrc.onerror = (err) => {
        console.log("SSE error:", err);
        sseOpen = false;
        if (eventSrc.readyState === EventSource.CLOSED) {
            eventSrc.close();
            updateStatusText("Connection lost, retrying");
            setTimeout(startSSE, sseRetryDelayMs);
            sseRetryDelayMs = Math.min(sseRetryDelayMs * 2, sseMaxRetryDelayMs);
        } else {
            // readyState CONNECTING: the browser is already retrying.
            updateStatusText("Connection lost, reconnecting");
        }
    };

    eventSrc.onmessage = (event) => {
        try {
            const msg = JSON.parse(event.data);
            eventRouter(msg);
        } catch (err) {
            console.log("Error parsing event data:", err);
        }
    };
}

async function eventRouter(msg) {
    switch (msg.eventType) {
        case "newUser":
            broadcastMediaState()
            startOffer(msg.userId)
            break
        case "acknowledge":
            startLoading(33, 100);
            updateStatusText("Waiting on others to join")
            break
        case "newOffer":
            console.log("newOffer:", msg.userId)
            startAnswer(msg.userId, msg)
            break
        case "candidate": handleRemoteCandidate(msg); break
        case "mediaState": handleMediaState(msg); break
        case "removedUser": closePeer(msg.userId); break
        case "answer": handleAnswer(msg); break
        default: console.log("something happened but don't know what", msg); break
    }
}

async function startLocalVideo(userId) {
    localUserId = userId
    localVideo = document.getElementById('localVideo');
    localStream = await navigator.mediaDevices.getUserMedia(videoConstraints);
    localVideo.srcObject = localStream;
    const controls = document.getElementById('controls')
    controls.classList.remove("hidden")
    startSSE()
}

async function handleAnswer(msg) {
    console.log("handling answer from ", msg.userId)
    const peer = peers[msg.userId];
    if (!peer) {
        console.warn("answer for unknown peer", msg.userId);
        return
    }
    if (peer.pc.signalingState !== "have-local-offer") {
        // A stale answer (e.g. to an offer we rolled back during glare) must not
        // be applied, or it corrupts the connection's remote description.
        console.warn("ignoring answer, peer signaling is", peer.pc.signalingState, ":", msg.userId);
        return
    }
    await peer.pc.setRemoteDescription({ "type": "answer", "sdp": msg.answer });
    for (const candidate of bundledCandidates(msg)) {
        await addCandidate(peer, candidate)
    }
    await flushPendingCandidates(peer)
    // Same gap as answerPeer: an ICE-restart answer can land on a connection
    // that is already "connected", so no state event follows to leave
    // "offering" behind.
    if (peer.pc.connectionState === "connected") {
        handleConnectionState(peer);
    }
    hideLoadingModal()
    console.log("done handling answer")
}

// bundledCandidates parses the legacy `candidates` field so peers that still
// bundle candidates into their offer/answer keep working.
function bundledCandidates(message) {
    try {
        return JSON.parse(message.candidates || "[]")
    } catch (err) {
        console.error("unparseable candidates from", message.userId, err);
        return []
    }
}

async function addCandidate(peer, candidate) {
    try {
        await peer.pc.addIceCandidate(candidate)
    } catch (err) {
        console.error("rejected candidate from", peer.id, err);
    }
}

async function flushPendingCandidates(peer) {
    const queued = peer.pendingCandidates;
    peer.pendingCandidates = [];
    for (const candidate of queued) {
        await addCandidate(peer, candidate)
    }
}

function handleRemoteCandidate(msg) {
    let candidate
    try {
        candidate = JSON.parse(msg.candidate)
    } catch (err) {
        console.error("unparseable candidate from", msg.userId, err);
        return
    }
    const peer = peers[msg.userId]
    if (!peer) {
        (earlyCandidates[msg.userId] = earlyCandidates[msg.userId] || []).push(candidate)
        return
    }
    if (!peer.pc.remoteDescription) {
        peer.pendingCandidates.push(candidate)
        return
    }
    addCandidate(peer, candidate)
}

function handleMediaState(msg) {
    let state
    try {
        state = JSON.parse(msg.mediaState)
    } catch (err) {
        console.error("unparseable mediaState from", msg.userId, err);
        return
    }
    const peer = peers[msg.userId]
    if (!peer) {
        earlyMediaState[msg.userId] = state
        return
    }
    peer.mediaState = state
    updatePeerOverlay(msg.userId)
}

function localMediaState() {
    const audio = localStream ? localStream.getAudioTracks().some(t => t.enabled) : true
    const video = localStream ? localStream.getVideoTracks().some(t => t.enabled) : true
    return { audio: audio, video: video }
}

function broadcastMediaState() {
    if (!localUserId || localUserId === "notset") {
        return
    }
    sendEvent({
        eventType: "mediaState",
        userId: localUserId,
        mediaState: JSON.stringify(localMediaState()),
        code: "{{ .code }}",
    })
}
