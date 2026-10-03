/**
 * Participant window layout: grid, spotlight (main + filmstrip), and free-floating
 * draggable/resizable tiles with minimize and maximize (viewport-filling stage).
 */
(function () {
  const LAYOUT_MODES = ['grid', 'spotlight', 'free'];
  const MODE_LABELS = {
    grid: 'Grid',
    spotlight: 'Spotlight',
    free: 'Free layout',
  };
  const MINIMIZED_WIDTH = 176;
  const MINIMIZED_HEIGHT = 99;
  const MIN_WINDOW_WIDTH = 200;
  const MIN_WINDOW_HEIGHT = 140;
  const CHROME_HEIGHT = 36;
  const CONTROLS_RESERVE_PX = 88;
  const LOCAL_VIDEO_RESERVE_PX = 120;

  let layoutMode = 'grid';
  let spotlightMainId = null;
  let maximizedId = null;
  let zIndexCounter = 10;
  const windowState = {};
  let dragSession = null;
  let resizeSession = null;

  function getStage() {
    return document.getElementById('video-stage');
  }

  function getContainer() {
    return document.getElementById('video-container');
  }

  function peerDisplayName(id) {
    return id.split('__')[0] || id;
  }

  function controlsReserve() {
    const controls = document.getElementById('controls');
    if (!controls || controls.classList.contains('hidden')) {
      return 24;
    }
    return controls.classList.contains('fly-in') ? CONTROLS_RESERVE_PX : 24;
  }

  function viewportBounds() {
    return {
      width: window.innerWidth,
      height: window.innerHeight,
      bottom: controlsReserve(),
      right: LOCAL_VIDEO_RESERVE_PX,
    };
  }

  function clampRect(rect) {
    const bounds = viewportBounds();
    const minVisible = 48;
    const maxW = bounds.width - 16;
    const maxH = bounds.height - bounds.bottom - 16;
    const width = Math.min(Math.max(rect.width, MIN_WINDOW_WIDTH), maxW);
    const height = Math.min(Math.max(rect.height, MIN_WINDOW_HEIGHT), maxH);
    let left = rect.left;
    let top = rect.top;
    left = Math.min(Math.max(left, -width + minVisible), bounds.width - minVisible);
    top = Math.min(Math.max(top, 0), bounds.height - bounds.bottom - minVisible);
    return { left, top, width, height };
  }

  function defaultGridRects(count) {
    const bounds = viewportBounds();
    const pad = 12;
    const usableW = Math.min(bounds.width - pad * 2, 1200);
    const usableH = bounds.height - bounds.bottom - pad * 2;
    const originX = (bounds.width - usableW) / 2;
    const originY = pad;
    if (count <= 0) {
      return [];
    }
    let cols = 1;
    if (count === 2) {
      cols = bounds.width > 800 ? 2 : 1;
    } else if (count >= 3) {
      cols = 2;
    }
    const rows = Math.ceil(count / cols);
    const gap = 12;
    const cellW = (usableW - gap * (cols - 1)) / cols;
    const cellH = (usableH - gap * (rows - 1)) / rows;
    const rects = [];
    for (let i = 0; i < count; i++) {
      const col = i % cols;
      const row = Math.floor(i / cols);
      rects.push({
        left: originX + col * (cellW + gap),
        top: originY + row * (cellH + gap),
        width: cellW,
        height: cellH,
      });
    }
    return rects;
  }

  function applyRect(el, rect) {
    el.style.left = `${rect.left}px`;
    el.style.top = `${rect.top}px`;
    el.style.width = `${rect.width}px`;
    el.style.height = `${rect.height}px`;
  }

  function bringToFront(el) {
    zIndexCounter += 1;
    el.style.zIndex = String(zIndexCounter);
  }

  function ensureWindowState(id) {
    if (!windowState[id]) {
      windowState[id] = {
        minimized: false,
        savedRect: null,
      };
    }
    return windowState[id];
  }

  function setModeClass() {
    const container = getContainer();
    if (!container) {
      return;
    }
    container.classList.remove('layout-grid', 'layout-spotlight', 'layout-free');
    container.classList.add(`layout-${layoutMode}`);
    container.dataset.layoutMode = layoutMode;
    if (layoutMode === 'spotlight' && spotlightMainId) {
      container.dataset.spotlightMain = spotlightMainId;
    } else {
      delete container.dataset.spotlightMain;
    }
    const label = document.getElementById('layoutModeLabel');
    if (label) {
      label.textContent = MODE_LABELS[layoutMode];
    }
  }

  function stackMinimizedWindows() {
    const container = getContainer();
    if (!container) {
      return;
    }
    const minimized = [...container.querySelectorAll('.participant-window.is-minimized')];
    const bounds = viewportBounds();
    let x = 12;
    const y = bounds.height - bounds.bottom - MINIMIZED_HEIGHT - 12;
    minimized.forEach((el) => {
      applyRect(el, {
        left: x,
        top: y,
        width: MINIMIZED_WIDTH,
        height: MINIMIZED_HEIGHT,
      });
      x += MINIMIZED_WIDTH + 8;
    });
  }

  function applySpotlightLayout() {
    const container = getContainer();
    if (!container) {
      return;
    }
    const windows = [...container.querySelectorAll('.participant-window')];
    if (windows.length === 0) {
      return;
    }
    if (!spotlightMainId || !document.getElementById(`${spotlightMainId}-container`)) {
      spotlightMainId = windows[0].id.replace('-container', '');
    }
    const bounds = viewportBounds();
    const pad = 12;
    const filmstripH = Math.min(132, Math.max(96, bounds.height * 0.16));
    const mainRect = {
      left: pad,
      top: pad,
      width: bounds.width - pad * 2,
      height: bounds.height - bounds.bottom - filmstripH - pad * 3,
    };
    const others = windows.filter((w) => w.id !== `${spotlightMainId}-container`);
    const mainEl = document.getElementById(`${spotlightMainId}-container`);
    if (mainEl) {
      mainEl.classList.add('is-spotlight-main');
      applyRect(mainEl, mainRect);
      bringToFront(mainEl);
    }
    windows.forEach((w) => w.classList.remove('is-spotlight-main', 'is-spotlight-strip'));
    if (mainEl) {
      mainEl.classList.add('is-spotlight-main');
    }
    const stripCount = others.length;
    if (stripCount === 0) {
      return;
    }
    const gap = 8;
    const stripW = bounds.width - pad * 2;
    const tileW = Math.min(200, (stripW - gap * (stripCount - 1)) / stripCount);
    const tileH = filmstripH;
    const stripY = bounds.height - bounds.bottom - tileH - pad;
    let x = (bounds.width - (tileW * stripCount + gap * (stripCount - 1))) / 2;
    others.forEach((el) => {
      el.classList.add('is-spotlight-strip');
      applyRect(el, { left: x, top: stripY, width: tileW, height: tileH });
      x += tileW + gap;
    });
  }

  function applyFreeLayout() {
    const container = getContainer();
    if (!container) {
      return;
    }
    const windows = [...container.querySelectorAll('.participant-window:not(.is-minimized)')];
    if (maximizedId) {
      const maxEl = document.getElementById(`${maximizedId}-container`);
      if (maxEl) {
        const bounds = viewportBounds();
        applyRect(maxEl, {
          left: 8,
          top: 8,
          width: bounds.width - 16,
          height: bounds.height - bounds.bottom - 16,
        });
        bringToFront(maxEl);
      }
    }
    windows.forEach((el) => {
      if (maximizedId && el.id === `${maximizedId}-container`) {
        return;
      }
      const id = el.id.replace('-container', '');
      const state = ensureWindowState(id);
      if (!el.style.left) {
        const idx = windows.indexOf(el);
        const defaults = defaultGridRects(windows.length);
        const rect = defaults[idx] || defaultGridRects(1)[0];
        applyRect(el, rect);
        state.savedRect = { ...rect };
      }
    });
    stackMinimizedWindows();
  }

  function applyGridLayout() {
    const container = getContainer();
    if (!container) {
      return;
    }
    maximizedId = null;
    const windows = [...container.querySelectorAll('.participant-window')];
    const rects = defaultGridRects(windows.length);
    windows.forEach((el, i) => {
      el.classList.remove('is-minimized', 'is-maximized', 'is-spotlight-main', 'is-spotlight-strip');
      el.style.zIndex = '';
      if (rects[i]) {
        applyRect(el, rects[i]);
      }
      const id = el.id.replace('-container', '');
      const state = ensureWindowState(id);
      state.minimized = false;
      state.savedRect = null;
    });
  }

  function applyLayout() {
    setModeClass();
    const container = getContainer();
    if (!container) {
      return;
    }
    container.classList.remove('one', 'two', 'padding-bottom');
    const count = container.querySelectorAll('.participant-window').length;
    if (count === 0) {
      container.classList.add('hidden');
      return;
    }
    container.classList.remove('hidden');
    if (layoutMode === 'grid') {
      applyGridLayout();
    } else if (layoutMode === 'spotlight') {
      container.querySelectorAll('.participant-window').forEach((el) => {
        el.classList.remove('is-minimized', 'is-maximized');
      });
      applySpotlightLayout();
    } else {
      applyFreeLayout();
    }
  }

  function updateLayoutChrome(el, id) {
    let chrome = el.querySelector('.window-chrome');
    if (chrome) {
      return;
    }
    chrome = document.createElement('div');
    chrome.className = 'window-chrome';
    chrome.innerHTML = `
      <div class="window-drag-handle" title="Drag to move">
        <span class="window-title"></span>
      </div>
      <div class="window-actions">
        <button type="button" class="window-btn window-minimize" title="Minimize" aria-label="Minimize participant window">
          <span class="material-symbols-outlined">minimize</span>
        </button>
        <button type="button" class="window-btn window-maximize" title="Maximize to stage" aria-label="Maximize participant window">
          <span class="material-symbols-outlined">crop_square</span>
        </button>
        <button type="button" class="window-btn window-spotlight" title="Set as spotlight main" aria-label="Set as spotlight main view">
          <span class="material-symbols-outlined">star</span>
        </button>
      </div>`;
    el.insertBefore(chrome, el.firstChild);
    chrome.querySelector('.window-title').textContent = peerDisplayName(id);
    chrome.querySelector('.window-minimize').addEventListener('click', (e) => {
      e.stopPropagation();
      toggleMinimize(id);
    });
    chrome.querySelector('.window-maximize').addEventListener('click', (e) => {
      e.stopPropagation();
      toggleMaximize(id);
    });
    chrome.querySelector('.window-spotlight').addEventListener('click', (e) => {
      e.stopPropagation();
      setSpotlightMain(id);
    });
    const handle = chrome.querySelector('.window-drag-handle');
    handle.addEventListener('mousedown', (e) => startDrag(e, el, id));
    handle.addEventListener('dblclick', () => toggleMaximize(id));

    const resizeHandle = document.createElement('div');
    resizeHandle.className = 'window-resize-handle';
    resizeHandle.title = 'Resize';
    resizeHandle.addEventListener('mousedown', (e) => startResize(e, el, id));
    el.appendChild(resizeHandle);

    el.addEventListener('mousedown', () => bringToFront(el));
    el.addEventListener('click', () => {
      const state = ensureWindowState(id);
      if (state.minimized) {
        toggleMinimize(id);
        return;
      }
      if (layoutMode === 'spotlight' && el.classList.contains('is-spotlight-strip')) {
        setSpotlightMain(id);
      }
    });
  }

  function toggleMinimize(id) {
    const el = document.getElementById(`${id}-container`);
    if (!el) {
      return;
    }
    const state = ensureWindowState(id);
    if (layoutMode !== 'free') {
      layoutMode = 'free';
      maximizedId = null;
    }
    if (state.minimized) {
      state.minimized = false;
      el.classList.remove('is-minimized');
      if (state.savedRect) {
        applyRect(el, clampRect(state.savedRect));
      }
      updateMaximizeIcon(el, false);
    } else {
      if (!state.savedRect) {
        state.savedRect = readRect(el);
      }
      state.minimized = true;
      el.classList.remove('is-maximized');
      el.classList.add('is-minimized');
      if (maximizedId === id) {
        maximizedId = null;
      }
    }
    applyLayout();
  }

  function readRect(el) {
    return {
      left: parseFloat(el.style.left) || 0,
      top: parseFloat(el.style.top) || 0,
      width: parseFloat(el.style.width) || el.offsetWidth,
      height: parseFloat(el.style.height) || el.offsetHeight,
    };
  }

  function updateMaximizeIcon(el, maximized) {
    const icon = el.querySelector('.window-maximize .material-symbols-outlined');
    if (icon) {
      icon.textContent = maximized ? 'close_fullscreen' : 'crop_square';
    }
    el.classList.toggle('is-maximized', maximized);
  }

  function toggleMaximize(id) {
    const el = document.getElementById(`${id}-container`);
    if (!el) {
      return;
    }
    const state = ensureWindowState(id);
    if (layoutMode === 'spotlight') {
      setSpotlightMain(id);
      return;
    }
    if (layoutMode === 'grid') {
      layoutMode = 'free';
    }
    if (state.minimized) {
      toggleMinimize(id);
    }
    if (maximizedId === id) {
      maximizedId = null;
      updateMaximizeIcon(el, false);
      if (state.savedRect) {
        applyRect(el, clampRect(state.savedRect));
      }
    } else {
      if (maximizedId) {
        const prev = document.getElementById(`${maximizedId}-container`);
        if (prev) {
          updateMaximizeIcon(prev, false);
        }
      }
      state.savedRect = readRect(el);
      maximizedId = id;
      updateMaximizeIcon(el, true);
    }
    applyLayout();
  }

  function setSpotlightMain(id) {
    spotlightMainId = id;
    layoutMode = 'spotlight';
    maximizedId = null;
    const container = getContainer();
    if (container) {
      container.querySelectorAll('.participant-window').forEach((w) => {
        updateMaximizeIcon(w, false);
      });
    }
    applyLayout();
  }

  function cycleLayoutMode() {
    const idx = LAYOUT_MODES.indexOf(layoutMode);
    layoutMode = LAYOUT_MODES[(idx + 1) % LAYOUT_MODES.length];
    maximizedId = null;
    const container = getContainer();
    if (container) {
      container.querySelectorAll('.participant-window').forEach((w) => {
        w.classList.remove('is-minimized', 'is-maximized');
        updateMaximizeIcon(w, false);
        const pid = w.id.replace('-container', '');
        if (windowState[pid]) {
          windowState[pid].minimized = false;
        }
      });
    }
    if (layoutMode === 'spotlight') {
      const first = container && container.querySelector('.participant-window');
      spotlightMainId = first ? first.id.replace('-container', '') : null;
    }
    applyLayout();
  }

  function resetLayout() {
    layoutMode = 'grid';
    spotlightMainId = null;
    maximizedId = null;
    Object.keys(windowState).forEach((k) => delete windowState[k]);
    const container = getContainer();
    if (container) {
      container.querySelectorAll('.participant-window').forEach((w) => {
        w.classList.remove('is-minimized', 'is-maximized', 'is-spotlight-main', 'is-spotlight-strip');
        w.style.left = '';
        w.style.top = '';
        w.style.width = '';
        w.style.height = '';
        w.style.zIndex = '';
        updateMaximizeIcon(w, false);
      });
    }
    applyLayout();
    if (typeof updateContainerClass === 'function') {
      updateContainerClass();
    }
  }

  function startDrag(e, el, id) {
    if (layoutMode !== 'free' || e.button !== 0) {
      if (layoutMode === 'spotlight' && el.classList.contains('is-spotlight-strip')) {
        setSpotlightMain(id);
      }
      return;
    }
    const state = ensureWindowState(id);
    if (state.minimized || maximizedId === id) {
      return;
    }
    e.preventDefault();
    const rect = readRect(el);
    dragSession = {
      el,
      id,
      startX: e.clientX,
      startY: e.clientY,
      origin: rect,
    };
    bringToFront(el);
    document.addEventListener('mousemove', onDragMove);
    document.addEventListener('mouseup', onDragEnd);
  }

  function onDragMove(e) {
    if (!dragSession) {
      return;
    }
    const dx = e.clientX - dragSession.startX;
    const dy = e.clientY - dragSession.startY;
    const next = clampRect({
      ...dragSession.origin,
      left: dragSession.origin.left + dx,
      top: dragSession.origin.top + dy,
    });
    applyRect(dragSession.el, next);
  }

  function onDragEnd() {
    if (dragSession) {
      const state = ensureWindowState(dragSession.id);
      state.savedRect = readRect(dragSession.el);
    }
    dragSession = null;
    document.removeEventListener('mousemove', onDragMove);
    document.removeEventListener('mouseup', onDragEnd);
  }

  function startResize(e, el, id) {
    if (layoutMode !== 'free' || e.button !== 0) {
      return;
    }
    const state = ensureWindowState(id);
    if (state.minimized || maximizedId === id) {
      return;
    }
    e.preventDefault();
    e.stopPropagation();
    const rect = readRect(el);
    resizeSession = {
      el,
      id,
      startX: e.clientX,
      startY: e.clientY,
      origin: rect,
    };
    bringToFront(el);
    document.addEventListener('mousemove', onResizeMove);
    document.addEventListener('mouseup', onResizeEnd);
  }

  function onResizeMove(e) {
    if (!resizeSession) {
      return;
    }
    const dx = e.clientX - resizeSession.startX;
    const dy = e.clientY - resizeSession.startY;
    const next = clampRect({
      left: resizeSession.origin.left,
      top: resizeSession.origin.top,
      width: resizeSession.origin.width + dx,
      height: resizeSession.origin.height + dy,
    });
    applyRect(resizeSession.el, next);
  }

  function onResizeEnd() {
    if (resizeSession) {
      const state = ensureWindowState(resizeSession.id);
      state.savedRect = readRect(resizeSession.el);
    }
    resizeSession = null;
    document.removeEventListener('mousemove', onResizeMove);
    document.removeEventListener('mouseup', onResizeEnd);
  }

  function registerParticipantWindow(containerDiv, id) {
    containerDiv.classList.add('participant-window');
    updateLayoutChrome(containerDiv, id);
    ensureWindowState(id);
    if (!spotlightMainId) {
      spotlightMainId = id;
    }
    applyLayout();
    if (typeof updateContainerClass === 'function') {
      updateContainerClass();
    }
  }

  function unregisterParticipantWindow(id) {
    delete windowState[id];
    if (spotlightMainId === id) {
      spotlightMainId = null;
    }
    if (maximizedId === id) {
      maximizedId = null;
    }
    applyLayout();
  }

  function onResize() {
    applyLayout();
    if (typeof updateContainerClass === 'function') {
      updateContainerClass();
    }
  }

  window.registerParticipantWindow = registerParticipantWindow;
  window.unregisterParticipantWindow = unregisterParticipantWindow;
  window.cycleLayoutMode = cycleLayoutMode;
  window.resetParticipantLayout = resetLayout;
  window.applyParticipantLayout = applyLayout;

  window.addEventListener('resize', onResize);
  document.addEventListener('DOMContentLoaded', () => {
    const cycleBtn = document.getElementById('layoutCycleButton');
    const resetBtn = document.getElementById('resetLayoutButton');
    if (cycleBtn) {
      cycleBtn.addEventListener('click', cycleLayoutMode);
    }
    if (resetBtn) {
      resetBtn.addEventListener('click', resetLayout);
    }
  });
})();
