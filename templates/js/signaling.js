let eventSrc;
let localStream;

const videoConstraints = {
    audio: true,
    video: {
        facingMode: { ideal: 'user' }
    }
}
const allowedCodecs = ['VP9', 'H264'];
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
let aliveUsers = {}
let pcs = {}
// ICE candidates are gathered per peer connection: a candidate belongs to the
// ufrag of the connection that produced it, so sharing one list across peers
// sends the wrong candidates to everybody once a second peer joins.
const peerCandidates = {};
// How long to keep gathering ICE before sending the offer/answer anyway.
const gatheringTimeoutSeconds = 5;
let localVideo = document.getElementById('localVideo');


function delay(ms) {
    return new Promise(resolve => setTimeout(resolve, ms));
}

async function createRemoteVideoStream(id) {
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
    const nameID = id.split("__")[0]
    videoOverlay.innerHTML = "<p>" + nameID + "</p>"

    // Append the video element to the container div
    containerDiv.appendChild(videoElement);
    containerDiv.appendChild(videoOverlay);

    // Append the container div to the main video container
    const videoContainer = document.getElementById('video-container');
    videoContainer.appendChild(containerDiv);

    // Set the ontrack event handler for the peer connection
    pcs[id].ontrack = (event) => {
        console.log("ontrack event:", id,event);
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
        const overlay = document.getElementById(id + '-video-overlay');
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

function updateContainerClass() {
    const videoContainer = document.getElementById('video-container');
    const childrenCount = videoContainer.children.length;
    videoContainer.classList.remove('one');
    videoContainer.classList.remove('two');

    if (childrenCount > 0) {
        videoContainer.classList.remove('hidden');
        if (window.innerWidth > 800) {
            videoContainer.classList.add("padding-bottom")
            showControls()
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
async function addLocalTracks(id) {
    localVideo = document.getElementById('localVideo');
    if (!localStream) {
        localStream = await navigator.mediaDevices.getUserMedia(videoConstraints);
        localVideo.srcObject = localStream;
    }
    localStream.getTracks().forEach((track) => pcs[id].addTrack(track, localStream));
}

// waitForCandidates must run after the local description is set: gathering
// belongs to the ufrag of that description, so candidates collected before it
// (or before a second createOffer) are discarded by the remote peer.
async function waitForCandidates(id) {
    statusText = "Gathering network information"
    updateStatusText(statusText)
    for (let i = 0; i < gatheringTimeoutSeconds; i++) {
        if (!pcs[id] || pcs[id].iceGatheringState === 'complete') {
            return
        }
        await delay(1000);
        statusText += "."
        updateStatusText(statusText)
    }
}

async function offerPeer(id) {
    updateStatusText("Attempting to connect to new user")
    const offer = await pcs[id].createOffer();
    offer.sdp = filterCodecs(offer.sdp, allowedCodecs);
    await pcs[id].setLocalDescription(offer);
    await waitForCandidates(id)
    if (!pcs[id]) {
        return
    }
    console.log("sending offer to ", id)
    sendEvent({
        eventType: "newOffer",
        userId: localUserId,
        forUser: id,
        offer: pcs[id].localDescription.sdp,
        candidates: JSON.stringify(peerCandidates[id] || []),
        code: "{{ .code }}",
    })
}

async function answerPeer(id, msg) {
    console.log("handling offer from", id)
    await pcs[id].setRemoteDescription(new RTCSessionDescription({ "type": "offer", "sdp": msg.offer }));
    await handleRemoteCandidates(msg)
    const answer = await pcs[id].createAnswer();
    answer.sdp = filterCodecs(answer.sdp, allowedCodecs);
    await pcs[id].setLocalDescription(answer);
    await waitForCandidates(id)
    if (!pcs[id]) {
        return
    }
    console.log("sending answer to ", id)
    sendEvent({
        eventType: "answer",
        userId: localUserId,
        forUser: id,
        answer: pcs[id].localDescription.sdp,
        candidates: JSON.stringify(peerCandidates[id] || []),
        code: "{{ .code }}",
    })
    hideLoadingModal()
}

function hideLoadingModal() {
    document.getElementById('loadingModal').classList.add("hidden")
}

function sendEvent(msg) {
    // Exchange the answer with the remote peer
    fetch("/event", {
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

async function newWebRTC(id, msg = {}) {
    console.log("adding new user to pcs: ", id)
    if (pcs[id]) {
        console.log("skipping, user exists,", id)
        return
    }
    pcs[id] = new RTCPeerConnection(configuration);
    peerCandidates[id] = [];
    pcs[id].onicecandidate = ({ candidate }) => handleCandidate(id, candidate);
    createRemoteVideoStream(id)
    await addLocalTracks(id)
    // The existing participants offer, the joiner answers; see
    // docs/signaling-protocol.md.
    if ('offer' in msg) {
        await answerPeer(id, msg)
    } else {
        await offerPeer(id)
    }
}

function startSSE() {
    const eventSrc = new EventSource(`/events?userId=${localUserId}&code={{ .code }}`);

    eventSrc.onopen = () => {
        console.log("SSE connection established.");
    };

    eventSrc.onerror = (err) => {
        console.log("SSE error:", err);
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
            newWebRTC(msg.userId)
            break
        case "acknowledge":
            startLoading(33, 100);
            updateStatusText("Waiting on others to join")
            break
        case "newOffer":
            console.log("newOffer:", msg.userId)
            newWebRTC(msg.userId, msg)
            break
        case "removedUser": handleClose(msg); break
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

async function handleClose(msg) {
    if (pcs[msg.userId]) {
        pcs[msg.userId].close();
        delete pcs[msg.userId];
    }
    delete peerCandidates[msg.userId];
    removeRemoteVideoStream(msg.userId)
    console.log("closed video of peer: ", msg.userId)
}

async function handleAnswer(msg) {
    console.log("handling answer from ", msg.userId)
    if (!pcs[msg.userId]) {
        console.warn("answer for unknown peer", msg.userId);
        return
    }
    await pcs[msg.userId].setRemoteDescription({ "type": "answer", "sdp": msg.answer });
    await handleRemoteCandidates(msg)
    hideLoadingModal()
    console.log("done handling answer")
}

async function handleRemoteCandidates(message) {
    const pc = pcs[message.userId]
    if (!pc) {
        return
    }
    let candidates
    try {
        candidates = JSON.parse(message.candidates || "[]")
    } catch (err) {
        console.error("unparseable candidates from", message.userId, err);
        return
    }
    console.log("candidates from ", message.userId)
    for (const candidate of candidates) {
        try {
            await pc.addIceCandidate(candidate)
        } catch (err) {
            console.error("rejected candidate from", message.userId, err);
        }
    }
}

function handleCandidate(id, candidate) {
    if (candidate != null && peerCandidates[id]) {
        peerCandidates[id].push(candidate)
    }
}
