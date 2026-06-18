(function () {
  const icon = document.querySelector('link[rel~="icon"][type="image/svg+xml"]');
  if (!icon) return;

  const originalHref = icon.getAttribute('href') || icon.href;
  const motionQuery = window.matchMedia('(prefers-reduced-motion: reduce)');
  const frameMs = 125;
  const rotationMs = 3000;

  let frameTemplate = null;
  let timer = 0;
  let loading = null;

  const restoreStaticIcon = () => {
    icon.setAttribute('href', originalHref);
  };

  const setMotionListener = (callback) => {
    if (typeof motionQuery.addEventListener === 'function') {
      motionQuery.addEventListener('change', callback);
      return;
    }
    motionQuery.addListener(callback);
  };

  const stop = () => {
    if (timer) {
      window.clearInterval(timer);
    }
    timer = 0;
    restoreStaticIcon();
  };

  const createFrame = (angle) => {
    const doc = frameTemplate.cloneNode(true);
    const vinyl = doc.getElementById('vinyl');
    if (!vinyl) return originalHref;

    for (const animation of Array.from(vinyl.querySelectorAll('animateTransform'))) {
      animation.remove();
    }

    vinyl.setAttribute('transform', `rotate(${angle.toFixed(2)} 200 200)`);

    const svg = new XMLSerializer().serializeToString(doc.documentElement);
    return `data:image/svg+xml,${encodeURIComponent(svg)}`;
  };

  const tick = () => {
    const progress = (window.performance.now() % rotationMs) / rotationMs;
    icon.setAttribute('href', createFrame(progress * 360));
  };

  const loadTemplate = () => {
    if (frameTemplate) return Promise.resolve(frameTemplate);
    if (loading) return loading;

    loading = fetch(originalHref)
      .then((response) => {
        if (!response.ok) throw new Error('favicon failed to load');
        return response.text();
      })
      .then((svg) => {
        const doc = new DOMParser().parseFromString(svg, 'image/svg+xml');
        if (doc.querySelector('parsererror')) throw new Error('favicon svg is invalid');
        frameTemplate = doc;
        return frameTemplate;
      })
      .catch(() => {
        restoreStaticIcon();
        return null;
      });

    return loading;
  };

  const start = () => {
    if (motionQuery.matches) {
      stop();
      return;
    }
    if (timer) {
      return;
    }

    loadTemplate().then((template) => {
      if (!template || timer || motionQuery.matches) return;
      tick();
      timer = window.setInterval(tick, frameMs);
    });
  };

  setMotionListener(start);
  start();
})();
