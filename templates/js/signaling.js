// Frontend WebRTC Signaling Logic (signaling.js)

let eventSrc;
let localStream; // Holds the single local media stream
let localUserId = "notset-" + Math.random().toString(36).substring(2, 7); // Default, should be set by startLocalVideo
const pcs = {}; // Stores RTCPeerConnection objects, keyed by remote user ID

const videoConstraints = {
    audio: true,
    video: {
        facingMode: { ideal: 'user' }
    }
};
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

let localVideo = document.getElementById('localVideo');

function delay(ms) {
    return new Promise(resolve => setTimeout(resolve, ms));
}

async function createRemoteVideoStream(remoteUserId) {
    if (document.getElementById(remoteUserId + '-container')) {
        console.log("Remote video container already exists for", remoteUserId);
        return;
    }

    const containerDiv = document.createElement('div');
    containerDiv.id = remoteUserId + '-container';
    containerDiv.classList.add("remote-views");

    const videoElement = document.createElement('video');
    videoElement.id = remoteUserId + '-remoteVideo';
    videoElement.autoplay = true;
    videoElement.playsinline = true;

    const videoOverlay = document.createElement('div');
    videoOverlay.id = remoteUserId + '-video-overlay';
    videoOverlay.classList.add("video-overlay");
    const nameID = remoteUserId.split("__")[0];
    videoOverlay.innerHTML = `<p>${nameID}</p>`;

    containerDiv.appendChild(videoElement);
    containerDiv.appendChild(videoOverlay);

    const videoContainer = document.getElementById('video-container');
    videoContainer.appendChild(containerDiv);

    if (pcs[remoteUserId]) {
        pcs[remoteUserId].ontrack = (event) => {
            console.log("ontrack event for:", remoteUserId, event);
            if (event.streams && event.streams[0]) {
                const remoteStream = event.streams[0];
                const remoteVideoElement = document.getElementById(videoElement.id);
                if (remoteVideoElement && remoteVideoElement.srcObject !== remoteStream) {
                    console.log("Attaching remote stream for", remoteUserId, "to video element.");
                    attachMediaStream(remoteVideoElement, remoteStream, remoteUserId);
                }
            } else if (event.track) {
                const newStream = new MediaStream([event.track]);
                const remoteVideoElement = document.getElementById(videoElement.id);
                if (remoteVideoElement && remoteVideoElement.srcObject !== newStream) {
                    console.log("Attaching remote track (new stream) for", remoteUserId, "to video element.");
                    attachMediaStream(remoteVideoElement, newStream, remoteUserId);
                }
            }
        };
    } else {
        console.error("PeerConnection not found for ontrack setup:", remoteUserId);
    }
    updateContainerClass();
}

async function attachMediaStream(videoElement, stream, remoteUserId) {
    try {
        videoElement.srcObject = stream;
        videoElement.muted = true;
        await videoElement.play();
        console.log("Remote video started playing for:", remoteUserId);
        videoElement.muted = false;
    } catch (playError) {
        console.error("Error playing remote video for user", remoteUserId, ":", playError);
    }
}

function updateContainerClass() {
    const videoContainer = document.getElementById('video-container');
    if (!videoContainer) return;
    const childrenCount = videoContainer.querySelectorAll('.remote-views').length;
    videoContainer.classList.remove('one', 'two', 'hidden', 'padding-bottom');

    if (childrenCount > 0) {
        if (window.innerWidth > 800) {
            videoContainer.classList.add("padding-bottom");
            if (typeof showControls === 'function') showControls();
        }
    } else {
        videoContainer.classList.add('hidden');
    }

    if (childrenCount === 1) videoContainer.classList.add('one');
    else if (childrenCount === 2) videoContainer.classList.add(window.innerWidth > 800 ? 'two' : 'one');
    else if (childrenCount === 3 || childrenCount === 4) videoContainer.classList.add('two');
    else if (childrenCount > 4) videoContainer.classList.add('grid-layout');
}
window.addEventListener("resize", updateContainerClass);

function removeRemoteVideoStream(remoteUserId) {
    const containerDiv = document.getElementById(remoteUserId + '-container');
    if (containerDiv) containerDiv.remove();
    updateContainerClass();
    const videoContainer = document.getElementById('video-container');
    if (videoContainer && videoContainer.querySelectorAll('.remote-views').length === 0) {
        if (typeof updateStatusText === 'function') updateStatusText("Waiting on others to join");
        const loadingModal = document.getElementById('loadingModal');
        if (loadingModal) loadingModal.classList.remove("hidden");
    }
}

function filterCodecs(sdp, allowedCodecs) {
    const sdpLines = sdp.split('\r\n');
    const videoMLineIndex = sdpLines.findIndex(line => line.startsWith('m=video'));
    if (videoMLineIndex === -1) return sdp;

    let mLineParts = sdpLines[videoMLineIndex].split(' ');
    let filteredPayloadTypes = [];
    const codecRegex = new RegExp(`^a=rtpmap:(\\d+) (${allowedCodecs.join('|')})\\/\\d+`, 'i');

    for (let i = videoMLineIndex + 1; i < sdpLines.length; i++) {
        if (sdpLines[i].startsWith('m=')) break;
        const match = sdpLines[i].match(codecRegex);
        if (match) filteredPayloadTypes.push(match[1]);
    }

    if (filteredPayloadTypes.length === 0) return sdp;
    mLineParts = mLineParts.slice(0, 3).concat(filteredPayloadTypes);
    sdpLines[videoMLineIndex] = mLineParts.join(' ');

    return sdpLines.filter(line => {
        if (line.startsWith('m=') || line.startsWith('c=') || line.startsWith('a=sendrecv') || line.startsWith('a=recvonly') || line.startsWith('a=sendonly') || line.startsWith('a=inactive')) return true;
        const fmtpMatch = line.match(/^a=fmtp:(\d+)/);
        const rtcpFbMatch = line.match(/^a=rtcp-fb:(\d+)/);
        if (fmtpMatch && filteredPayloadTypes.includes(fmtpMatch[1])) return true;
        if (rtcpFbMatch && filteredPayloadTypes.includes(rtcpFbMatch[1])) return true;
        if (line.startsWith('a=rtpmap:')) {
            const payloadType = line.split(' ')[0].split(':')[1];
            return filteredPayloadTypes.includes(payloadType);
        }
        return !line.startsWith('a=rtpmap:') && !line.startsWith('a=rtcp-fb:') && !line.startsWith('a=fmtp:');
    }).join('\r\n');
}

function setupIceCandidateGathering(pc, remoteUserId, specificLocalCandidates) {
    console.log(`Setting up ICE candidate gathering for connection to ${remoteUserId}. Candidates will be stored for this peer.`);
    pc.onicecandidate = ({ candidate }) => {
        if (candidate) {
            specificLocalCandidates.push(candidate);
        } else {
            console.log(`ICE gathering complete signal (null candidate) for ${remoteUserId}. Total candidates in array: ${specificLocalCandidates.length}`);
        }
    };
    pc.onicegatheringstatechange = () => {
        if (pcs[remoteUserId]) {
             console.log(`ICE gathering state for ${remoteUserId}: ${pcs[remoteUserId].iceGatheringState}. Candidates so far: ${specificLocalCandidates.length}`);
        }
    };
}

async function waitForIceCandidates(pc, specificLocalCandidates, remoteUserId) {
    const maxWaitTime = 8000; 
    const checkInterval = 250;
    let waitedTime = 0;
    console.log(`Starting ICE candidate gathering for ${remoteUserId} (local candidates for this peer). Max wait: ${maxWaitTime/1000}s. PC signalingState: ${pc.signalingState}, ICE gathering state: ${pc.iceGatheringState}`);

    while (waitedTime < maxWaitTime) {
        if (pc.iceGatheringState === 'complete') {
            console.log(`ICE gathering reported 'complete' for ${remoteUserId} after ${waitedTime}ms. Candidates collected: ${specificLocalCandidates.length}`);
            break; 
        }
        await delay(checkInterval);
        waitedTime += checkInterval;
    }

    if (pc.iceGatheringState !== 'complete') {
        console.log(`Timed out waiting for ICE gathering to complete for ${remoteUserId} (state: ${pc.iceGatheringState}). Found ${specificLocalCandidates.length} candidates during wait. Proceeding.`);
    }
    await delay(500); 
    console.log(`Finished ICE candidate gathering phase for ${remoteUserId}. Final candidate count for this peer: ${specificLocalCandidates.length}`);
}

async function createPeerConnection(remoteUserId) {
    console.log("Creating new PeerConnection for:", remoteUserId);
    if (pcs[remoteUserId]) {
        console.log("Closing existing stale PeerConnection for:", remoteUserId);
        pcs[remoteUserId].close();
        delete pcs[remoteUserId];
    }

    pcs[remoteUserId] = new RTCPeerConnection(configuration);
    const pc = pcs[remoteUserId];

    if (localStream) {
        localStream.getTracks().forEach(track => {
            try { pc.addTrack(track, localStream); } catch (e) { console.error(`Error adding track to PC for ${remoteUserId}:`, e); }
        });
        console.log("Local tracks added to PeerConnection for:", remoteUserId);
    } else {
        console.error("Local stream not available for PeerConnection for:", remoteUserId);
        return null;
    }
    
    await createRemoteVideoStream(remoteUserId);

    pc.onconnectionstatechange = () => {
        if (pcs[remoteUserId]) {
            console.log(`Connection state for ${remoteUserId}: ${pcs[remoteUserId].connectionState}`);
            if (['failed', 'disconnected', 'closed'].includes(pcs[remoteUserId].connectionState)) {
                console.log(`PC for ${remoteUserId} is ${pcs[remoteUserId].connectionState}. Cleaning up.`);
                handleClose({ userId: remoteUserId });
            }
        }
    };
    return pc;
}

async function initializeWebRTCForUser(remoteUserId, incomingMessage = null) {
    if (remoteUserId === localUserId) {
        console.log("Attempting to connect to self, skipping:", remoteUserId);
        return;
    }
    console.log(`Initializing WebRTC for user: ${remoteUserId}. Incoming message type:`, incomingMessage ? incomingMessage.eventType : 'N/A (offering)');

    const pc = await createPeerConnection(remoteUserId);
    if (!pc) {
        console.error("Failed to create PeerConnection for", remoteUserId);
        return;
    }
    
    const specificLocalCandidates = [];
    setupIceCandidateGathering(pc, remoteUserId, specificLocalCandidates);

    // Use "{{ .code }}" directly as it will be processed by Go templates if this .js file is a template
    const currentRoomCode = "{{ .code }}"; 

    if (incomingMessage && incomingMessage.offer) {
        console.log(`Handling incoming offer from ${remoteUserId}`);
        try {
            await pc.setRemoteDescription(new RTCSessionDescription({ type: 'offer', sdp: incomingMessage.offer }));
            if (incomingMessage.candidates) await handleRemoteCandidates(remoteUserId, incomingMessage.candidates);

            const answer = await pc.createAnswer();
            answer.sdp = filterCodecs(answer.sdp, allowedCodecs);
            await pc.setLocalDescription(answer);
            await waitForIceCandidates(pc, specificLocalCandidates, remoteUserId); 

            const answerMessage = {
                eventType: "answer", userId: localUserId, forUser: remoteUserId,
                answer: answer.sdp, candidates: JSON.stringify(specificLocalCandidates),
                code: currentRoomCode, 
            };
            console.log(`Sending answer to ${remoteUserId} with ${specificLocalCandidates.length} candidates.`);
            sendEvent(answerMessage);
        } catch (error) { console.error(`Error handling offer/creating answer for ${remoteUserId}:`, error); }
    } else {
        console.log(`Creating offer for ${remoteUserId}`);
        try {
            const offer = await pc.createOffer();
            offer.sdp = filterCodecs(offer.sdp, allowedCodecs);
            await pc.setLocalDescription(offer);
            await waitForIceCandidates(pc, specificLocalCandidates, remoteUserId); 

            const offerMessage = {
                eventType: "newOffer", userId: localUserId, forUser: remoteUserId,
                offer: offer.sdp, candidates: JSON.stringify(specificLocalCandidates),
                code: currentRoomCode,
            };
            console.log(`Sending newOffer to ${remoteUserId} with ${specificLocalCandidates.length} candidates.`);
            sendEvent(offerMessage);
        } catch (error) { console.error(`Error creating offer for ${remoteUserId}:`, error); }
    }
    const loadingModal = document.getElementById('loadingModal');
    if (loadingModal) loadingModal.classList.add("hidden");
}

function sendEvent(msg) {
    fetch("/event", {
        method: "POST", headers: { "Content-Type": "application/json" },
        body: JSON.stringify(msg)
    })
    .then(response => { if (!response.ok) console.error(`Error sending event ${msg.eventType}: ${response.status}`); })
    .catch(error => console.error(`Fetch error sending event ${msg.eventType}:`, error));
}

function startSSE() {
    // Use "{{ .code }}" directly as it will be processed by Go templates if this .js file is a template
    const roomCode = "{{ .code }}"; 

    if (!roomCode ) { // Check if it was NOT replaced
        console.error("Room code was not replaced by template. Cannot start SSE. Ensure JS is served as Go template.");
        alert("Error: Application configuration issue (room ID). Cannot connect.");
        return;
    }
    
    if (localUserId.startsWith("notset-")) {
         console.warn("localUserId not explicitly set by startLocalVideo, using default:", localUserId);
    }

    eventSrc = new EventSource(`/events?userId=${localUserId}&code=${roomCode}`);
    eventSrc.onopen = () => console.log("SSE connection established with userId:", localUserId, "in room:", roomCode);
    eventSrc.onerror = (err) => console.error("SSE error:", err);
    eventSrc.onmessage = (event) => {
        try {
            const msg = JSON.parse(event.data);
            eventRouter(msg);
        } catch (err) { console.error("Error parsing SSE event data:", err, "Data:", event.data); }
    };
}

async function eventRouter(msg) {
    switch (msg.eventType) {
        case "newUser": 
            if (msg.userId !== localUserId) {
                console.log(`New user ${msg.userId} joined. Initializing WebRTC (sending offer).`);
                initializeWebRTCForUser(msg.userId);
            }
            break;
        case "acknowledge": 
            console.log("SSE connection acknowledged by server.");
            if (typeof startLoading === 'function') startLoading(33,100); // From original code
            if (typeof updateStatusText === 'function') updateStatusText("Waiting on others to join"); // From original
            break;
        case "newOffer": 
            if (msg.userId !== localUserId) {
                console.log(`Received newOffer from ${msg.userId}. Initializing WebRTC (sending answer).`);
                initializeWebRTCForUser(msg.userId, msg);
            }
            break;
        case "answer": 
            if (msg.userId !== localUserId && pcs[msg.userId]) {
                console.log(`Received answer from ${msg.userId}.`);
                try {
                    await pcs[msg.userId].setRemoteDescription(new RTCSessionDescription({ type: 'answer', sdp: msg.answer }));
                    if (msg.candidates) await handleRemoteCandidates(msg.userId, msg.candidates);
                } catch (error) { console.error(`Error setting remote desc for answer from ${msg.userId}:`, error); }
                 const loadingModal = document.getElementById('loadingModal'); // From original handleAnswer
                 if (loadingModal) loadingModal.classList.add("hidden");       // From original handleAnswer
                 console.log("Done handling answer for", msg.userId);             // From original handleAnswer
            } else { console.warn("Received answer for unknown/missing PC for user:", msg.userId); }
            break;
        case "removedUser":
            console.log(`User ${msg.userId} left the room.`);
            handleClose(msg);
            break;
        default: console.log("Unknown SSE event type or unhandled:", msg); break;
    }
}

async function startLocalVideo(providedUserId) {
    if (!providedUserId) {
        console.error("startLocalVideo called without a providedUserId.");
        alert("User ID is missing. Cannot start video session.");
        return;
    }
    localUserId = providedUserId; 
    console.log("Starting local video. Local user ID set to:", localUserId);

    localVideo = document.getElementById('localVideo');
    if (!localVideo) {
        console.error("Local video element (#localVideo) not found.");
        return;
    }

    try {
        if (localStream) { 
            localStream.getTracks().forEach(track => track.stop());
            console.log("Stopped existing local stream tracks.");
        }
        localStream = await navigator.mediaDevices.getUserMedia(videoConstraints);
        localVideo.srcObject = localStream;
        localVideo.muted = true; 
        await localVideo.play(); 
        console.log("Local video stream started and attached.");

        const controls = document.getElementById('controls');
        if (controls) controls.classList.remove("hidden"); // From original
        
        startSSE(); 
    } catch (error) {
        console.error("Error accessing local media (getUserMedia):", error);
        if (error.name === "NotFoundError" || error.name === "DevicesNotFoundError") alert("No camera/microphone found.");
        else if (error.name === "NotAllowedError" || error.name === "PermissionDeniedError") alert("Permission to use camera/microphone was denied.");
        else alert(`Error accessing media: ${error.message}`);
    }
}

function handleClose(msg) { 
    const remoteUserId = msg.userId;
    if (pcs[remoteUserId]) {
        console.log(`Closing PeerConnection for user: ${remoteUserId}`);
        pcs[remoteUserId].ontrack = null;
        pcs[remoteUserId].onicecandidate = null;
        pcs[remoteUserId].onconnectionstatechange = null;
        pcs[remoteUserId].close();
        delete pcs[remoteUserId];
    }
    removeRemoteVideoStream(remoteUserId);
    console.log(`Cleaned up resources for disconnected peer: ${remoteUserId}. Original msg.Id was ${msg.Id}`); // msg.Id from original
}

async function handleRemoteCandidates(remoteUserId, candidatesString) {
    if (!pcs[remoteUserId]) {
        console.warn(`PC for ${remoteUserId} not found. Cannot add remote candidates.`);
        return;
    }
    if (!candidatesString || candidatesString.trim() === "" || candidatesString.trim() === "[]") {
        console.log(`No remote candidates provided or empty list from ${remoteUserId}.`);
        return;
    }

    try {
        const candidates = JSON.parse(candidatesString);
        if (candidates && Array.isArray(candidates)) {
            console.log(`Processing ${candidates.length} remote candidates from ${remoteUserId}.`);
            for (const candidate of candidates) { // Changed from for...in to for...of for arrays
                if (candidate) { 
                    try { await pcs[remoteUserId].addIceCandidate(new RTCIceCandidate(candidate)); } 
                    catch (e) { console.error(`Error adding remote ICE for ${remoteUserId}:`, e, "Cand:", candidate); }
                }
            }
        } else { console.warn(`Parsed candidates from ${remoteUserId} not an array. String:`, candidatesString); }
    } catch (e) { console.error(`Error parsing remote candidates from ${remoteUserId}:`, e, "String:", candidatesString); }
}

