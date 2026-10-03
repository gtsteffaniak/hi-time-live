const code = "{{ .code }}"
let username = ""

// Function to check for text content
function checkForTextContent() {
  // These only exist inside the room's privacy modal, not on the home page.
  const button = document.getElementById('start-button');
  const nameInput = document.getElementById('nameInput');
  if (!button || !nameInput) {
    return;
  }
  // Event listener for the input field
  nameInput.addEventListener('input', () => {
    username = nameInput.value
    // Check if the input field contains any non-whitespace characters
    if (nameInput.value.trim().length > 0) {
      button.style.display = 'block'; // Show the button container
    } else {
      button.style.display = 'none'; // Hide the button container
    }
  });
  // Event listener for Enter key press
  nameInput.addEventListener('keydown', (event) => {
    if (event.key == "Enter" && nameInput.value.trim().length > 0) {
      startSession()
    }
  });
}

// Call this function whenever the text might change
checkForTextContent();

function startSession() {
  const userIdCode = crypto.randomUUID().split("-")[0];
  const userId = `${username}__${userIdCode}`
  console.log(`local connection id ${userId}`)
  const privacyModal = document.getElementById('privacyModal');
  const loadingModal = document.getElementById('loadingModal');
  const localVideo = document.getElementById('localVideo');
  privacyModal.classList.add("hidden")
  loadingModal.classList.remove("hidden")
  localVideo.classList.remove("hidden")
  startLocalVideo(userId)
  startLoading(0, 33);
};

function copyToClipboard(text) {
  navigator.clipboard.writeText(text)
    .then(() => {
      console.log("Copied to clipboard: " + text);
    })
    .catch(err => {
      console.error("Failed to copy text: ", err);
    });
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

// setControlsVisible is the single source of truth for controls visibility:
// callers say what they want instead of toggling, so repeated calls from
// join/leave/resize cannot drift out of sync with what is on screen.
function setControlsVisible(visible) {
  const ctab = document.getElementById('ctab');
  const controls = document.getElementById('controls');
  const videocontainer = document.getElementById('video-container');
  videocontainer.classList.toggle("padding-bottom", visible)
  controls.classList.toggle("fly-in", visible)
  ctab.classList.toggle("fly-in", visible)
}

// The ctab is the only manual toggle and is only rendered on narrow screens.
function showControls() {
  const controls = document.getElementById('controls');
  setControlsVisible(!controls.classList.contains('fly-in'))
}