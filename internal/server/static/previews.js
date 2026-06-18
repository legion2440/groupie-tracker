(function () {
  const previewTargetSelector = '.artist-card, .artist-vinyl[data-preview-artist]';
  const previewSourceSelector = '.record[data-preview-artist], .artist-vinyl[data-preview-artist]';
  const previewCache = new Map();

  let activeTarget = null;
  let activeAudio = null;
  let activeToken = 0;

  function previewKey(artist) {
    return artist.trim().toLowerCase().replace(/[\s,._/\\-]+/g, ' ');
  }

  function emptyPreview() {
    return {
      preview: '',
      title: '',
      artist: '',
      track_id: 0,
    };
  }

  function normalizePreview(data) {
    if (!data || typeof data.preview !== 'string') {
      return emptyPreview();
    }
    return {
      preview: data.preview,
      title: typeof data.title === 'string' ? data.title : '',
      artist: typeof data.artist === 'string' ? data.artist : '',
      track_id: Number.isFinite(data.track_id) ? data.track_id : 0,
    };
  }

  function logStartedPreview(preview) {
    if (!preview.preview) return;
    console.info('[Deezer preview] ' + preview.artist + ' — ' + preview.title);
  }

  function closestPreviewTarget(target) {
    if (!target || typeof target.closest !== 'function') {
      return null;
    }
    return target.closest(previewTargetSelector);
  }

  function previewSource(target) {
    if (!target) {
      return null;
    }
    if (typeof target.matches === 'function' && target.matches(previewSourceSelector)) {
      return target;
    }
    if (typeof target.querySelector !== 'function') {
      return null;
    }
    return target.querySelector(previewSourceSelector);
  }

  function movedWithin(element, target) {
    return target instanceof Node && element.contains(target);
  }

  function resetAudio(audio) {
    if (!audio) return;
    audio.pause();
    try {
      audio.currentTime = 0;
    } catch (_) {
      // Some browsers reject seeking before metadata is available.
    }
  }

  function stopActivePreview() {
    activeToken += 1;
    resetAudio(activeAudio);
    activeAudio = null;
    activeTarget = null;
  }

  function loadPreview(artist) {
    const key = previewKey(artist);
    if (!key) {
      return Promise.resolve(emptyPreview());
    }
    if (previewCache.has(key)) {
      return previewCache.get(key);
    }

    const endpoint = new URL('/api/deezer-preview', window.location.origin);
    endpoint.searchParams.set('artist', artist);

    const request = fetch(endpoint.href, {
      method: 'GET',
      credentials: 'same-origin',
      headers: {
        Accept: 'application/json',
      },
    })
      .then((response) => {
        if (!response.ok) return null;
        return response.json();
      })
      .then((data) => normalizePreview(data))
      .catch(() => emptyPreview());

    previewCache.set(key, request);
    return request;
  }

  function playPreview(target) {
    const source = previewSource(target);
    if (!source) return;

    const artist = source.dataset.previewArtist || '';
    if (activeTarget === target) {
      return;
    }

    stopActivePreview();
    activeTarget = target;
    const token = activeToken;

    loadPreview(artist).then((preview) => {
      if (activeToken !== token || activeTarget !== target || !preview.preview) return;

      const audio = new Audio(preview.preview);
      activeAudio = audio;

      const playResult = audio.play();
      if (playResult && typeof playResult.then === 'function') {
        playResult.then(() => {
          if (activeAudio === audio) {
            logStartedPreview(preview);
          }
        }).catch(() => {
          if (activeAudio === audio) {
            resetAudio(audio);
            activeAudio = null;
          }
        });
      } else {
        logStartedPreview(preview);
      }
    });
  }

  document.addEventListener('pointerover', (event) => {
    const target = closestPreviewTarget(event.target);
    if (!target || movedWithin(target, event.relatedTarget)) return;
    playPreview(target);
  }, true);

  document.addEventListener('pointerout', (event) => {
    const target = closestPreviewTarget(event.target);
    if (!target || movedWithin(target, event.relatedTarget)) return;
    if (activeTarget === target) stopActivePreview();
  }, true);

  document.addEventListener('mouseenter', (event) => {
    const target = closestPreviewTarget(event.target);
    if (!target || movedWithin(target, event.relatedTarget)) return;
    playPreview(target);
  }, true);

  document.addEventListener('mouseleave', (event) => {
    const target = closestPreviewTarget(event.target);
    if (!target || movedWithin(target, event.relatedTarget)) return;
    if (activeTarget !== target) return;
    stopActivePreview();
  }, true);
})();
