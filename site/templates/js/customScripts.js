const code = "{{ .code }}"

function startSession() {
  const privacyModal = document.getElementById('privacyModal');
  const loadingModal = document.getElementById('loadingModal');

  const startButton = privacyModal.querySelector('.button.is-primary.start');
  privacyModal.classList.add("hidden")
  loadingModal.classList.remove("hidden")

  const videos = document.getElementById('videos');
  startLocalVideo()
  startLoading(0, 33);
};

function copyToClipboard() {
  var copyText = document.getElementById("copyCode");
  copyText.select();
  document.execCommand("copy");
  console.log("copied code: " + copyText.value);
}

function goToRoom() {
  var code = document.getElementById("copyCode").value;
  const uuidPattern = /[0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}/i;
  // Use the match method to find the UUID in the string
  const match = code.match(uuidPattern);
  // If a match is found, return the matched UUID, otherwise return null
  if (match) {
    window.location.href = "/room?id=" + match[0];
  } else {
    alert("invalid code")
  }
}

function loadJoinModal() {
  let menu = document.getElementById('mainMenu');
  let joinModal = document.getElementById('join-modal');
  menu.classList.add("hidden")
  joinModal.classList.remove("hidden")
}

function hideJoinModal() {
  let menu = document.getElementById('mainMenu');
  let joinModal = document.getElementById('join-modal');
  menu.classList.remove("hidden")
  joinModal.classList.add("hidden")
}

function updateStatusText(message) {
  let status = document.getElementById('status-text');
  status.innerText = message
}

function showControls() {
  const ctab = document.getElementById('ctab');
  const controls = document.getElementById('controls');
  controls.classList.toggle("fly-in")
  ctab.classList.toggle("fly-in")
}