const code = "{{ .code }}"
let username = ""

// Function to check for text content
function checkForTextContent() {
  // Replace 'your-text-element-id' with the ID of the element containing the text
  const button = document.getElementById('start-button');

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
}

// Call this function whenever the text might change
checkForTextContent();

function startSession() {
  const userIdCode = crypto.randomUUID().split("-")[0];
  const userId = `${username}-${userIdCode}`
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

function showControls() {
  const ctab = document.getElementById('ctab');
  const controls = document.getElementById('controls');
  controls.classList.toggle("fly-in")
  ctab.classList.toggle("fly-in")
}