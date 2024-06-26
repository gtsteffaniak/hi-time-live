// Switch media function
async function switchMedia() {
    let newConstraints = videoConstraints;
    const currentMode = newConstraints.video.facingMode.ideal
    // Toggle facing mode
    newConstraints.video.facingMode.ideal = (currentMode === 'user') ? 'environment' : 'user';

    // Stop all current tracks
    localStream.getTracks().forEach(track => track.stop());

    // Get new media stream with updated constraints
    try {
        const newStream = await navigator.mediaDevices.getUserMedia(newConstraints);
        localStream = newStream;
        localVideo.srcObject = newStream;

        // Replace tracks in peer connections
        for (let socket_id in peers) {
            const peerConnection = peers[socket_id];
            if (peerConnection) {
                const senders = peerConnection.getSenders();
                senders.forEach(sender => {
                    const newTrack = newStream.getTracks().find(track => track.kind === sender.track.kind);
                    if (newTrack) {
                        sender.replaceTrack(newTrack);
                    }
                });
            }
        }
    } catch (error) {
        console.error('Error switching media.', error);
    }
    updateButtons()
}
/**
 * Enable/disable microphone
 */
function toggleMute() {
    for (let index in localStream.getAudioTracks()) {
        let enabled = localStream.getAudioTracks()[index].enabled
        localStream.getAudioTracks()[index].enabled = !enabled
        muteButton.innerText = enabled ? "Unmute" : "Mute"
        muteButton.style.backgroundColor = enabled ? "red" : "";
    }
}
/**
 * Enable/disable video
 */
function toggleVid() {
    for (let index in localStream.getVideoTracks()) {
        let enabled = localStream.getVideoTracks()[index].enabled
        localStream.getVideoTracks()[index].enabled = !enabled
        vidButton.innerText = enabled ? "Enable Video" : "Disable Video"
        vidButton.style.backgroundColor = enabled ? "red" : "";
    }
}
/**
 * updating text of buttons
 */
function updateButtons() {
    for (let index in localStream.getVideoTracks()) {
        vidButton.innerText = localStream.getVideoTracks()[index].enabled ? "Video Enabled" : "Video Disabled"
    }
    for (let index in localStream.getAudioTracks()) {
        muteButton.innerText = localStream.getAudioTracks()[index].enabled ? "Unmuted" : "Muted"
    }
}
