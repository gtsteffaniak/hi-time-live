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
const userId = crypto.randomUUID().split("-")[0];
let aliveUsers = {}
let pcs = {}
const localCandidates = [];
let localVideo = document.getElementById('localVideo');

console.log("local connection id:", userId)

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
    videoOverlay.innerHTML = "<p>" + id + "</p>"

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
        attachMediaStream(remoteVideo, remoteStream);
    };

}

// Helper function to attach media stream to the video element
function attachMediaStream(video, stream,id) {
    try {
        // Use Safari-friendly attachment logic
        video.srcObject = stream;
        video.addEventListener("loadedmetadata", async () => {
            try {
                await video.play(); // Ensure video playback starts
                console.log("Remote video is playing for:", id);
                video.muted = false; // Unmute after video starts
            } catch (playError) {
                console.error("Error playing the remote video:", playError);

                // Optional: Add a fallback for user interaction
                const overlay = document.getElementById(id + '-video-overlay');
                overlay.innerHTML += "<p>Click to play</p>";
                overlay.style.cursor = "pointer";

                overlay.addEventListener("click", async () => {
                    try {
                        overlay.style.display = "none"; // Hide the overlay
                        await video.play();
                        console.log("Remote video resumed after user interaction:", id);
                        video.muted = false;
                    } catch (interactionError) {
                        console.error("Error playing the video after user interaction:", interactionError);
                    }
                });
            }
        });
    } catch (error) {
        console.error("Error attaching media stream:", error);
    }
}

function removeRemoteVideoStream(id) {
    const containerDiv = document.getElementById(id + '-container');
    if (containerDiv) {
        containerDiv.remove(); // Removes the container div from the DOM
    }
    const videoContainer = document.getElementById('video-container');
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


async function waitForCandidates(id) {
    pcs[id].onicecandidate = ({ candidate }) => handleCandidate(candidate);
    createRemoteVideoStream(id)
    localVideo = document.getElementById('localVideo');
    if (localVideo.srcObject) {
        localStream = await navigator.mediaDevices.getUserMedia(videoConstraints);
        localVideo.srcObject = localStream;
        localStream.getTracks().forEach((track) => pcs[id].addTrack(track, localStream));

    }
    let offer = await pcs[id].createOffer();
    const filteredSDP = filterCodecs(offer.sdp, allowedCodecs);
    offer.sdp = filteredSDP;
    await pcs[id].setLocalDescription(offer);
    statusText = "Gathering network information"
    updateStatusText(statusText)
    for (let i = 0; i < 5; i++) {
        await delay(1000);
        if (localCandidates.length > 10) {
            await delay(1000)
            return
        }
        statusText += "."
        updateStatusText(statusText)
    }
}

async function handleOffer(msg) {
    let id = msg.userId
    if 
    console.log("handling offer from", msg.userId)
    const offerDescription = new RTCSessionDescription({ "type": "offer", "sdp": msg.offer })
    await pcs[id].setRemoteDescription(offerDescription);
    handleRemoteCandidates(msg)

    // Create an answer
    const answer = await pcs[id].createAnswer();
    const filteredSDP = filterCodecs(answer.sdp, allowedCodecs);

    // Set local description with the answer
    const responseMessage = {
        eventType: "answer",
        userId: userId,
        forUser: id,
        answer: filteredSDP,
        candidates: JSON.stringify(localCandidates),
        code: "{{ .code }}",
    }
    console.log("sending answer to ", id)
    // Exchange the answer with the remote peer
    sendEvent(responseMessage)
    loadingModal = document.getElementById('loadingModal');
    loadingModal.classList.add("hidden")
    await pcs[id].setLocalDescription(answer);
}

async function handleCreateOffer(id) {
    updateStatusText("Attempting to connect to new user")
    let myoffer = await pcs[id].createOffer();
    myoffer.sdp = filterCodecs(myoffer.sdp, allowedCodecs);;
    await pcs[id].setLocalDescription(myoffer);
    // Set local description with the answer
    const responseMessage = {
        eventType: "newOffer",
        userId: userId,
        offer: myoffer.sdp,
        candidates: JSON.stringify(localCandidates),
        code: "{{ .code }}",
    }
    console.log("sending offer to ", id)
    sendEvent(responseMessage)
}

function sendEvent(msg) {
    // Exchange the answer with the remote peer
    fetch("/event", {
        method: "POST",
        headers: {
            "Content-Type": "application/json"
        },
        body: JSON.stringify(msg)
    }).then(response => console.log(response))
        .catch(error => console.error('Fetch error:', error));

}

async function newWebRTC(id, msg = {}) {
    console.log("adding new user to pcs: ", id)
    if (id in pcs) {
        console.log("skipping, user exists,", id)
        return
    }
    if (pcs[id]) {
        pcs[id] = null
    }
    pcs[id] = await new RTCPeerConnection(configuration);
    console.log("new user pcs: ", pcs[id])

    await waitForCandidates(id)
    if ('offer' in msg) {
        handleOffer(msg)
    } else {
        handleCreateOffer(id)
    }
}

function startSSE() {
    const eventSrc = new EventSource(`/events?userId=${userId}&code={{ .code }}`);

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
        case "newUser","newOffer":
            newWebRTC(msg.userId,msg)
            break
        case "acknowledge":
            startLoading(33, 100);
            updateStatusText("Waiting on others to join")
            break
        case "removedUser": handleClose(msg); break
        case "answer": handleAnswer(msg); break
        default: console.log("something happened but don't know what", msg); break
    }
}

async function startLocalVideo() {
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
        pcs[msg.userId] = null;
    }
    removeRemoteVideoStream(msg.userId)
    console.log("closed video of peer: ", msg.Id)
}

async function handleAnswer(msg) {
    console.log("handling answer from ", msg.userId)
    await pcs[msg.userId].setRemoteDescription({ "type": "answer", "sdp": msg.answer });
    handleRemoteCandidates(msg)

    loadingModal = document.getElementById('loadingModal');
    loadingModal.classList.add("hidden")
    console.log("done handling answer")
}

async function handleRemoteCandidates(message) {
    let candidates = JSON.parse(message.candidates)
    console.log("candidates from ", message.userId)
    for (c in candidates) {
        await pcs[message["userId"]].addIceCandidate(candidates[c])
    }
}

async function handleCandidate(candidate) {
    if (candidate != null) {
        console.log("new candidate")
        localCandidates.push(candidate)
    }
}
