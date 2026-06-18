document.addEventListener('DOMContentLoaded', () => {
  const refreshButton = document.getElementById('refresh-btn');
  const refreshTooltip = document.getElementById('refresh-tooltip');
  let refreshResetTimer = null;

  function setRefreshState(state) {
    if (!refreshButton || !refreshTooltip) {
      return;
    }

    window.clearTimeout(refreshResetTimer);
    refreshResetTimer = null;

    refreshButton.dataset.state = state;

    const states = {
      idle: {
        tooltip: 'Обновить информацию',
        busy: false,
        disabled: false,
      },
      loading: {
        tooltip: 'Обновление информации…',
        busy: true,
        disabled: true,
      },
      success: {
        tooltip: 'Информация обновлена',
        busy: false,
        disabled: false,
      },
      error: {
        tooltip: 'Обновление недоступно',
        busy: false,
        disabled: false,
      },
    };

    const current = states[state] || states.idle;

    refreshTooltip.textContent = current.tooltip;
    refreshButton.setAttribute('aria-label', current.tooltip);
    refreshButton.setAttribute('aria-busy', String(current.busy));
    refreshButton.disabled = current.disabled;
  }

  async function responseReportsUnavailable(response) {
    const contentType = response.headers.get('content-type') || '';

    if (!contentType.includes('application/json')) {
      return false;
    }

    try {
      const result = await response.clone().json();

      return result?.unavailable === true ||
        result?.available === false ||
        result?.ok === false ||
        result?.success === false;
    } catch {
      return false;
    }
  }

  if (refreshButton) {
    setRefreshState('idle');

    refreshButton.addEventListener('click', async () => {
      setRefreshState('loading');

      try {
        const res = await fetch('/api/refresh', { method:'POST' });

        if (!res.ok || await responseReportsUnavailable(res)) {
          throw new Error(`Refresh failed: ${res.status}`);
        }

        setRefreshState('success');
        document.dispatchEvent(new CustomEvent('groupie:catalog-refreshed'));

        refreshResetTimer = window.setTimeout(() => {
          setRefreshState('idle');
        }, 5000);
      } catch (error) {
        console.error('Refresh failed:', error);
        setRefreshState('error');
      }
    });

    window.addEventListener('offline', () => { setRefreshState('error'); });
  }

  function initializeDualRange(slider) {
    const minInput = slider.querySelector('.dual-range__input--min');
    const maxInput = slider.querySelector('.dual-range__input--max');
    const track = slider.querySelector('.dual-range__track');
    const selected = slider.querySelector('.dual-range__selected');
    const outputMin = slider.closest('.filter-group')?.querySelector('[data-dual-range-output-min]');
    const outputMax = slider.closest('.filter-group')?.querySelector('[data-dual-range-output-max]');

    if (!minInput || !maxInput || !track || !selected || !outputMin || !outputMax) return;

    const thumbSize = 18;
    let activeInput = maxInput;

    function clamp(value, min, max) {
      return Math.min(Math.max(value, min), max);
    }

    function readBounds() {
      const minLimit = Number(minInput.min);
      const maxLimit = Number(maxInput.max);

      if (!Number.isFinite(minLimit) || !Number.isFinite(maxLimit) || minLimit > maxLimit) {
        return null;
      }

      return {
        minLimit,
        maxLimit,
        valueSpan: maxLimit - minLimit,
      };
    }

    function getMinimumGap(bounds) {
      if (bounds.valueSpan <= 0) {
        return 0;
      }

      const trackWidth = track.getBoundingClientRect().width;

      return Math.min(
        bounds.valueSpan,
        Math.ceil((thumbSize / Math.max(trackWidth, 1)) * bounds.valueSpan)
      );
    }

    function setActive(input) {
      activeInput = input;
      minInput.classList.toggle('is-active', input === minInput);
      maxInput.classList.toggle('is-active', input === maxInput);
    }

    function render(changedInput = activeInput) {
      const bounds = readBounds();

      if (!bounds) {
        return;
      }

      const minimumGap = getMinimumGap(bounds);
      let minValue = clamp(Number(minInput.value), bounds.minLimit, bounds.maxLimit);
      let maxValue = clamp(Number(maxInput.value), bounds.minLimit, bounds.maxLimit);

      if (changedInput === minInput) {
        minValue = Math.min(minValue, maxValue - minimumGap);
        minValue = clamp(minValue, bounds.minLimit, bounds.maxLimit);
      } else if (changedInput === maxInput) {
        maxValue = Math.max(maxValue, minValue + minimumGap);
        maxValue = clamp(maxValue, bounds.minLimit, bounds.maxLimit);
      } else if (maxValue - minValue < minimumGap) {
        maxValue = Math.min(bounds.maxLimit, minValue + minimumGap);
        minValue = Math.max(bounds.minLimit, maxValue - minimumGap);
      }

      if (maxValue - minValue < minimumGap) {
        if (changedInput === minInput) {
          minValue = Math.max(bounds.minLimit, maxValue - minimumGap);
        } else {
          maxValue = Math.min(bounds.maxLimit, minValue + minimumGap);
        }
      }

      if (minValue > maxValue) {
        maxValue = minValue;
      }

      minInput.value = String(minValue);
      maxInput.value = String(maxValue);
      outputMin.textContent = String(minValue);
      outputMax.textContent = String(maxValue);

      const minPercent = bounds.valueSpan > 0
        ? ((minValue - bounds.minLimit) / bounds.valueSpan) * 100
        : 0;
      const maxPercent = bounds.valueSpan > 0
        ? ((maxValue - bounds.minLimit) / bounds.valueSpan) * 100
        : 100;

      selected.style.left = `${minPercent}%`;
      selected.style.right = `${100 - maxPercent}%`;
      setActive(changedInput);
    }

    [minInput, maxInput].forEach((input) => {
      input.addEventListener('pointerdown', () => setActive(input));
      input.addEventListener('focus', () => setActive(input));
      input.addEventListener('input', () => {
        setActive(input);
        render(input);
      });
    });

    slider.addEventListener('filter-range:update', (event) => {
      const nextMin = Number(event.detail?.min);
      const nextMax = Number(event.detail?.max);

      if (!Number.isInteger(nextMin) || !Number.isInteger(nextMax) || nextMin > nextMax) {
        return;
      }

      const oldBounds = readBounds();

      if (!oldBounds) {
        return;
      }

      const currentMin = Number(minInput.value);
      const currentMax = Number(maxInput.value);
      const wasFullRange = currentMin === oldBounds.minLimit && currentMax === oldBounds.maxLimit;

      minInput.min = String(nextMin);
      minInput.max = String(nextMax);
      maxInput.min = String(nextMin);
      maxInput.max = String(nextMax);

      if (wasFullRange) {
        minInput.value = String(nextMin);
        maxInput.value = String(nextMax);
      } else {
        let preservedMin = clamp(currentMin, nextMin, nextMax);
        let preservedMax = clamp(currentMax, nextMin, nextMax);

        if (preservedMin > preservedMax) {
          preservedMax = preservedMin;
        }

        minInput.value = String(preservedMin);
        maxInput.value = String(preservedMax);
      }

      render(activeInput);
    });

    setActive(maxInput);
    render(maxInput);

    if ('ResizeObserver' in window) {
      const observer = new ResizeObserver(() => render(activeInput));
      observer.observe(slider);
    } else {
      window.addEventListener('resize', () => render(activeInput));
    }
  }

  document.querySelectorAll('[data-dual-range]').forEach(initializeDualRange);
});
