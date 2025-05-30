// templates/js/room.js

// Ensure global variables like localStream, pcs, videoElements, localConnectionId, iceServers are defined
// For example:
// let localStream;
// const pcs = {}; // Peer connections
// const videoElements = {}; // Video elements for remote streams
// let localConnectionId = null; // Set after connecting to SSE
// const iceServers = { iceServers: [{ urls: 'stun:stun.l.google.com:19302' }] }; // Example STUN server

// Function to initialize local media (camera/microphone)
async function initLocalMedia() {
    console.log('Requesting local media access...');
    try {
        // Prioritize camera, then microphone. Adjust constraints as needed.
        const stream = await navigator.mediaDevices.getUserMedia({ video: true, audio: true });
        localStream = stream;
        const localVideo = document.getElementById('localVideo');
        if (localVideo) {
            localVideo.srcObject = stream;
            localVideo.muted = true; // Mute local video to prevent echo
            console.log('Local media stream attached to video element.');
        } else {
            console.error('Local video element not found.');
        }
        // After getting local media, you would typically connect to the signaling server (SSE)
        // and then be ready to add peers.
    } catch (e) {
        console.error('Error accessing local media:', e);
        // Display error to user, e.g., "Could not access camera/microphone."
        alert('Error accessing local media: ' + e.message + '\nPlease check permissions and try again.');
    }
}


// Function to add a new peer connection
function addNewPeer(remoteID, isInitiator) {
    if (pcs[remoteID]) {
        console.warn('addNewPeer: Peer connection for', remoteID, 'already exists. Skipping creation.');
        return;
    }
    console.log('addNewPeer: Creating new peer connection for', remoteID, 'isInitiator:', isInitiator);

    pcs[remoteID] = new RTCPeerConnection(iceServers);
    const pc = pcs[remoteID]; // Convenience reference

    // Log state changes for debugging
    pc.oniceconnectionstatechange = () => {
        console.log('ICE connection state change for', remoteID, ':', pc.iceConnectionState);
        if (pc.iceConnectionState === 'failed') {
            console.error('ICE connection failed for', remoteID, '. Restarting ICE might be needed.');
            // pc.restartIce(); // Consider this for more advanced recovery
        } else if (pc.iceConnectionState === 'disconnected' || pc.iceConnectionState === 'closed') {
            console.warn('ICE connection disconnected or closed for', remoteID);
            // Handle cleanup if necessary, e.g., remove video element
            // removeRemoteVideo(remoteID); // You'd need to implement this
        }
    };

    pc.onsignalingstatechange = () => {
        console.log('Signaling state change for', remoteID, ':', pc.signalingState);
    };

    pc.onicecandidate = event => {
        if (event.candidate) {
            console.log('New ICE candidate for', remoteID, ':', event.candidate.candidate ? event.candidate.candidate.substring(0, 70) + '...' : 'candidate object');
            // sendSignal is expected to be defined in signaling.js
            sendSignal('candidate', event.candidate, remoteID);
        } else {
            console.log('All ICE candidates gathered for', remoteID);
        }
    };

    pc.ontrack = event => {
        console.log('ontrack event for', remoteID, 'Track:', event.track, 'Streams:', event.streams);
        if (event.streams && event.streams[0]) {
            console.log('Attaching stream from track event for', remoteID, 'Stream ID:', event.streams[0].id);
            addRemoteVideo(remoteID, event.streams[0]);
        } else {
            // If track event doesn't have streams array, create a new stream for the track
            // This is a fallback, usually event.streams[0] is present.
            console.warn('ontrack event for', remoteID, 'received track without a stream object. Creating a new stream for it.');
            const inboundStream = new MediaStream([event.track]);
            addRemoteVideo(remoteID, inboundStream);
        }
    };

    // Add local stream tracks to the peer connection
    if (localStream) {
        localStream.getTracks().forEach(track => {
            console.log('Adding local track to pc for', remoteID, 'Track kind:', track.kind, 'ID:', track.id);
            try {
                pc.addTrack(track, localStream);
            } catch (e) {
                console.error('Error adding local track for', remoteID, ':', e);
            }
        });
    } else {
        console.warn('addNewPeer: Local stream not available when creating PC for', remoteID, '. Remote peer will not receive video/audio initially.');
        // You might want to handle this case, e.g., by renegotiating when localStream becomes available.
    }

    if (isInitiator) {
        console.log('Creating offer for', remoteID);
        pc.createOffer()
            .then(offer => {
                console.log('Created offer for', remoteID, 'Offer SDP:', offer.sdp ? offer.sdp.substring(0, 50) + '...' : 'offer object');
                return pc.setLocalDescription(offer);
            })
            .then(() => {
                console.log('Set local description success (offer) for', remoteID);
                // sendSignal is expected to be defined in signaling.js
                sendSignal('offer', pc.localDescription, remoteID);
                console.log('Sent offer to', remoteID);
            })
            .catch(e => console.error('Error creating or sending offer to', remoteID, ':', e));
    }
}

// Handle incoming offer
function handleOffer(remoteID, offer) {
    console.log('handleOffer: Received offer from', remoteID, 'Offer data:', offer);

    let pc = pcs[remoteID];
    if (!pc) {
        console.log('handleOffer: No existing PC for', remoteID, '. Creating new one (expecting to answer).');
        addNewPeer(remoteID, false); // isInitiator = false
        pc = pcs[remoteID]; // Re-fetch after creation
        if (!pc) {
            console.error("CRITICAL: PeerConnection could not be created or found for", remoteID, "in handleOffer.");
            return;
        }
    } else {
        // PC exists. Check its state before processing a new offer.
        console.warn('handleOffer: PC for', remoteID, 'already exists. Current signalingState:', pc.signalingState);
        // If an offer arrives for an existing PC, it's typically for renegotiation.
        // However, if the server broadcasts offers, this might be a misdirected offer.
        // A simple check: if we are not 'stable' or expecting an offer, it might be problematic.
        if (pc.signalingState !== 'stable' && pc.signalingState !== 'have-local-offer' && pc.signalingState !== 'have-remote-pranswer') {
            console.error('handleOffer: Received offer for existing PC for', remoteID,
                'in an unexpected state:', pc.signalingState,
                '. Offer ignored to prevent errors. This could be a misdirected broadcast from the server.');
            return; // Ignore the offer
        }
        console.log('handleOffer: Proceeding to set remote offer for existing PC for', remoteID, '(renegotiation or expected offer).');
    }

    pc.setRemoteDescription(new RTCSessionDescription(offer))
        .then(() => {
            console.log('Set remote description success (offer) for', remoteID);
            return pc.createAnswer();
        })
        .then(answer => {
            console.log('Created answer for', remoteID, 'Answer SDP:', answer.sdp ? answer.sdp.substring(0, 50) + '...' : 'answer object');
            return pc.setLocalDescription(answer);
        })
        .then(() => {
            console.log('Set local description success (answer) for', remoteID);
            // sendSignal is expected to be defined in signaling.js
            sendSignal('answer', pc.localDescription, remoteID);
            console.log('Sent answer to', remoteID);
        })
        .catch(e => console.error('Error in handleOffer for', remoteID, ':', e));
}

// Handle incoming answer
function handleAnswer(remoteID, answer) {
    console.log('handleAnswer: Received answer from', remoteID, 'Answer data:', answer);
    const pc = pcs[remoteID];
    if (pc) {
        if (pc.signalingState === 'have-local-offer' || pc.signalingState === 'stable') { // 'stable' if polite peer accepted our offer after a glare
            pc.setRemoteDescription(new RTCSessionDescription(answer))
                .then(() => {
                    console.log('Set remote description success (answer) for', remoteID);
                })
                .catch(e => console.error('Error setting remote description for answer from', remoteID, ':', e));
        } else {
            console.error('handleAnswer: Received answer from', remoteID, 'but PC is in unexpected state:', pc.signalingState, '. Answer ignored.');
        }
    } else {
        console.error('handleAnswer: No peer connection found for', remoteID);
    }
}

// Handle incoming ICE candidate
function handleCandidate(remoteID, candidate) {
    console.log('handleCandidate: Received ICE candidate from', remoteID, 'Candidate:', candidate ? (candidate.candidate ? candidate.candidate.substring(0,70)+'...' : 'candidate object') : 'null candidate');
    const pc = pcs[remoteID];
    if (pc) {
        if (candidate) { // Ensure candidate is not null/undefined
            pc.addIceCandidate(new RTCIceCandidate(candidate))
                .then(() => {
                    // console.log('Added ICE candidate successfully for', remoteID); // Can be too verbose
                })
                .catch(e => console.error('Error adding ICE candidate for', remoteID, ':', e, 'Candidate:', candidate));
        } else {
            console.warn('handleCandidate: Received null or undefined ICE candidate from', remoteID);
        }
    } else {
        console.error('handleCandidate: No peer connection found for', remoteID);
    }
}

// Function to add remote video stream to the UI
function addRemoteVideo(remoteID, stream) {
    console.log('addRemoteVideo: Attempting to attach remote stream for', remoteID, 'Stream ID:', stream.id, 'Tracks:', stream.getTracks());
    if (!stream.active) {
        console.warn('addRemoteVideo: Stream for', remoteID, 'is not active.');
        // return; // Optionally return if stream is not active
    }
    if (stream.getTracks().length === 0) {
        console.warn('addRemoteVideo: Stream for', remoteID, 'has no tracks.');
        // return; // Optionally return if stream has no tracks
    }


    let videoElement = videoElements[remoteID];
    const videoContainer = document.getElementById('videoContainer');

    if (!videoContainer) {
        console.error('addRemoteVideo: videoContainer element not found in DOM.');
        return;
    }

    if (!videoElement) {
        console.log('addRemoteVideo: Creating new video element for', remoteID);
        // Create a wrapper for each remote video for better styling and future controls
        const remoteVideoWrapper = document.createElement('div');
        remoteVideoWrapper.id = 'wrapper-' + remoteID;
        remoteVideoWrapper.className = 'remote-video-wrapper'; // Add a class for styling

        videoElement = document.createElement('video');
        videoElement.id = 'video-' + remoteID;
        videoElement.autoplay = true;
        videoElement.playsinline = true; // Important for mobile browsers
        // videoElement.muted = true; // Generally, remote streams should not be muted by default

        const nameTag = document.createElement('p');
        nameTag.className = 'video-name-tag';
        nameTag.textContent = remoteID; // Display the remote user's ID

        remoteVideoWrapper.appendChild(nameTag);
        remoteVideoWrapper.appendChild(videoElement);
        videoContainer.appendChild(remoteVideoWrapper);
        videoElements[remoteID] = videoElement;
    } else {
        console.log('addRemoteVideo: Reusing existing video element for', remoteID);
    }

    if (videoElement.srcObject !== stream) {
        videoElement.srcObject = stream;
        console.log('addRemoteVideo: Assigned srcObject for', remoteID);
    } else {
        console.log('addRemoteVideo: srcObject for', remoteID, 'is already set to this stream. No change made.');
    }
    

    videoElement.onloadedmetadata = () => {
        console.log('Remote video metadata loaded for:', remoteID);
        videoElement.play().catch(e => console.error('Error playing remote video for', remoteID, ':', e));
    };
    videoElement.onplaying = () => {
        console.log('Remote video is playing for:', remoteID);
    };
    videoElement.onerror = (e) => {
        console.error('Video element error for', remoteID, ':', e);
    };
     stream.onremovetrack = (event) => {
        console.warn('Remote track removed for', remoteID, 'Track:', event.track);
        // If all tracks are removed, you might want to remove the video element or show a placeholder
        if (videoElement.srcObject && videoElement.srcObject.getTracks().length === 0) {
            console.warn('All tracks removed for stream associated with', remoteID, '. Cleaning up video element.');
            // removeRemoteVideo(remoteID); // Implement this if needed
        }
    };
}

// Function to handle user disconnection
function handleUserDisconnected(remoteID) {
    console.log('handleUserDisconnected: User', remoteID, 'disconnected.');
    if (pcs[remoteID]) {
        pcs[remoteID].close();
        delete pcs[remoteID];
        console.log('Closed and deleted peer connection for', remoteID);
    } else {
        console.warn('handleUserDisconnected: No peer connection found for', remoteID, 'to close.');
    }

    const videoElement = videoElements[remoteID];
    if (videoElement) {
        const wrapper = document.getElementById('wrapper-' + remoteID) || videoElement.parentElement;
        if (wrapper && wrapper.parentElement === document.getElementById('videoContainer')) {
             document.getElementById('videoContainer').removeChild(wrapper);
        } else if (videoElement.parentElement) { // Fallback if wrapper not found as expected
            videoElement.parentElement.removeChild(videoElement);
        }
        delete videoElements[remoteID];
        console.log('Removed video element for', remoteID);
    } else {
         console.warn('handleUserDisconnected: No video element found for', remoteID, 'to remove.');
    }
}


// Call initLocalMedia when the page/script loads
// Ensure this is called after the DOM is ready if it interacts with DOM elements immediately
// document.addEventListener('DOMContentLoaded', initLocalMedia);
// Or if your signaling setup is separate:
// initLocalMedia().then(() => {
// connectToSignaling(); // Assuming you have a function to establish SSE
// });

// Make sure sendSignal is defined (usually in signaling.js)
// Example:
// function sendSignal(type, data, targetID) {
// console.error("sendSignal is not implemented in room.js, should be in signaling.js");
// }

// Example of how you might call initLocalMedia if it's not already handled
// This is just for context; integrate it with your existing page load logic.
if (document.readyState === 'loading') {
    document.addEventListener('DOMContentLoaded', initLocalMedia);
} else {
    // DOMContentLoaded has already fired
    // initLocalMedia(); // Call it if it hasn't been called yet by other means
}
