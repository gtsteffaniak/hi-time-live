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
const configuration = {
    'iceServers': [
        { 'urls': 'stun:stun.l.google.com:19302' },
        { 'urls': 'stun:stun1.l.google.com:19302' },
        { 'urls': 'stun:stun2.l.google.com:19302' },
        { 'urls': 'stun:stun3.l.google.com:19302' },
        { 'urls': 'stun:stun4.l.google.com:19302' },
        { 'urls': 'stun:stun.ekiga.net' },
        { 'urls': 'stun:stun.ideasip.com' },
        { 'urls': 'stun:stun.stunprotocol.org:3478' },
        { 'urls': 'stun:stun.voiparound.com' },
        { 'urls': 'stun:stun.voipbuster.com' },
        { 'urls': 'stun:stun.voipstunt.com' },
    ]
};
let localUserId = "notset"

// Each remote participant gets one peer record: the RTCPeerConnection plus
// where it is in the offer/answer handshake, so late or duplicate signaling
// messages are dropped deliberately instead of corrupting a live connection.
// state: new -> offering|answering -> connected -> closed
const peers = {};
// A trickled candidate or mediaState can beat the offer that creates the peer;
// they are parked here and consumed when the peer record is created.
const earlyCandidates = {};
const earlyMediaState = {};
// SSE auto-reconnects while readyState is CONNECTING; a CLOSED stream is
// restarted manually with this backoff.
let sseRetryDelayMs = 1000;
const sseMaxRetryDelayMs = 30000;
let localVideo = document.getElementById('localVideo');


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

    // Set the ontrack event handler for the peer connection
    peers[id].pc.ontrack = (event) => {
        console.log("ontrack event:", id, event);
        const remoteStream = event.streams[0]; // Get the remote stream
        const remoteVideo = document.getElementById(videoElement.id);

        if (remoteVideo.srcObject) return; // Prevent redundant attachment

        console.log("Attaching remote view: ", id, "to stream:", remoteStream);

        // Attach the stream to the video element
        attachMediaStream(remoteVideo, remoteStream, id);
    };

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
    const state = peers[id] && peers[id].mediaState;
    const flags = [];
    if (state && !state.audio) flags.push("muted");
    if (state && !state.video) flags.push("video off");
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

function removeRemoteVideoStream(id) {
    const containerDiv = document.getElementById(id + '-container');
    if (containerDiv) {
        containerDiv.remove(); // Removes the container div from the DOM
    }
    const videoContainer = document.getElementById('video-container');
    updateContainerClass();
    const count = videoContainer.getElementsByTagName('video').length;
    if (count <= 0) {
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
    localStream.getTracks().forEach((track) => peer.pc.addTrack(track, localStream));
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
        pendingCandidates: earlyCandidates[id] || [],
        mediaState: earlyMediaState[id],
    };
    peers[id] = peer;
    delete earlyCandidates[id];
    delete earlyMediaState[id];
    peer.pc.onicecandidate = ({ candidate }) => {
        if (candidate && peers[id]) {
            sendEvent({
                eventType: "candidate",
                userId: localUserId,
                forUser: id,
                candidate: JSON.stringify(candidate),
                code: "{{ .code }}",
            })
        }
    };
    peer.pc.onconnectionstatechange = () => {
        if (peer.pc.connectionState === "connected") {
            peer.state = "connected";
        }
        if (peer.pc.connectionState === "failed") {
            console.warn("peer connection failed:", id);
            closePeer(id);
        }
    };
    createRemoteVideoStream(id)
    updatePeerOverlay(id)
    return peer
}

function closePeer(id) {
    const peer = peers[id];
    if (peer) {
        peer.state = "closed";
        peer.pc.close();
        delete peers[id];
    }
    removeRemoteVideoStream(id)
    console.log("closed video of peer: ", id)
}

async function offerPeer(peer) {
    updateStatusText("Attempting to connect to new user")
    peer.state = "offering";
    const offer = await peer.pc.createOffer();
    offer.sdp = filterCodecs(offer.sdp, allowedCodecs);
    await peer.pc.setLocalDescription(offer);
    if (peer.state === "closed") {
        return
    }
    console.log("sending offer to ", peer.id)
    sendEvent({
        eventType: "newOffer",
        userId: localUserId,
        forUser: peer.id,
        offer: peer.pc.localDescription.sdp,
        code: "{{ .code }}",
    })
}

async function answerPeer(peer, msg) {
    console.log("handling offer from", peer.id)
    peer.state = "answering";
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
    // Exchange the answer with the remote peer
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

// startOffer is the existing-participant side of a join: create a peer record
// and offer. startAnswer is the joiner side: the joiner only ever answers.
// See docs/signaling-protocol.md.
async function startOffer(id) {
    const existing = peers[id];
    if (existing && existing.state !== "new") {
        console.log("skipping offer, peer already negotiating:", id, existing.state)
        return
    }
    const peer = newPeer(id)
    await addLocalTracks(peer)
    await offerPeer(peer)
}

async function startAnswer(id, msg) {
    const existing = peers[id];
    if (existing && existing.state !== "new") {
        // A duplicate or late offer must not renegotiate a live connection.
        console.log("dropping offer, peer already", existing.state, ":", id)
        return
    }
    const peer = newPeer(id)
    await addLocalTracks(peer)
    await answerPeer(peer, msg)
}

function startSSE() {
    eventSrc = new EventSource(`events?userId=${localUserId}&code={{ .code }}`);

    eventSrc.onopen = () => {
        console.log("SSE connection established.");
        sseRetryDelayMs = 1000;
        // A reconnect is invisible to the room (the server only announces a
        // user when their last stream closes), but peers may have missed our
        // media state while the stream was down.
        broadcastMediaState();
    };

    eventSrc.onerror = (err) => {
        console.log("SSE error:", err);
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
    if (peer.state !== "offering") {
        console.warn("unexpected answer, peer is", peer.state, ":", msg.userId);
        return
    }
    await peer.pc.setRemoteDescription({ "type": "answer", "sdp": msg.answer });
    for (const candidate of bundledCandidates(msg)) {
        await addCandidate(peer, candidate)
    }
    await flushPendingCandidates(peer)
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
