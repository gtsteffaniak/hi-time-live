function isSafari() {
    var ua = navigator.userAgent.toLowerCase();
    return ua.indexOf('safari') != -1 && ua.indexOf('chrome') == -1 && ua.indexOf('android') == -1;
}

function safariContinue() {
    const privacyModal = document.getElementById('privacyModal');
    privacyModal.classList.remove("hidden")
    let modal = document.getElementById('safariModal');
    modal.classList.add("hidden")
}
