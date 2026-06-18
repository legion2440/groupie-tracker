document.addEventListener('DOMContentLoaded', () => {
  const form = document.querySelector('[data-filter-form]');
  const locationSearch = document.querySelector('[data-location-search]');
  const locationEmpty = document.querySelector('[data-location-empty]');

  function normalizeLocationQuery(value) {
    return String(value || '')
      .toLowerCase()
      .replace(/[,\.\-_\/\\]+/g, ' ')
      .trim()
      .replace(/\s+/g, ' ');
  }

  function setHiddenField(field, value) {
    if (!field || value === '') {
      if (field) {
        field.value = '';
        field.disabled = true;
      }
      return;
    }

    field.value = value;
    field.disabled = false;
  }

  function rangeValues(name) {
    const slider = document.querySelector(`[data-filter-range="${name}"]`);
    const minInput = slider?.querySelector('.dual-range__input--min');
    const maxInput = slider?.querySelector('.dual-range__input--max');

    if (!slider || !minInput || !maxInput) {
      return null;
    }

    const min = Number(minInput.min);
    const max = Number(maxInput.max);
    const selectedMin = Number(minInput.value);
    const selectedMax = Number(maxInput.value);

    if (![min, max, selectedMin, selectedMax].every(Number.isInteger) || min > max) {
      return null;
    }

    return {
      min,
      max,
      selectedMin: Math.min(Math.max(selectedMin, min), max),
      selectedMax: Math.min(Math.max(selectedMax, min), max),
    };
  }

  function yearStart(year) {
    return `${year}-01-01`;
  }

  function yearEnd(year) {
    return `${year}-12-31`;
  }

  function parseYearParam(value) {
    const match = String(value || '').match(/^(\d{4})(?:-\d{2}-\d{2})?$/);
    return match ? Number(match[1]) : NaN;
  }

  function syncCheckboxesFromURL(params, name) {
    const selected = new Set(params.getAll(name).filter((value) => value !== ''));

    document.querySelectorAll(`input[type="checkbox"][name="${name}"]`).forEach((input) => {
      input.checked = selected.has(input.value);
    });
  }

  function syncRangeFromURL(name, fromParam, toParam) {
    const slider = document.querySelector(`[data-filter-range="${name}"]`);
    const minInput = slider?.querySelector('.dual-range__input--min');
    const maxInput = slider?.querySelector('.dual-range__input--max');

    if (!slider || !minInput || !maxInput) {
      return;
    }

    const params = new URLSearchParams(window.location.search);
    const min = Number(minInput.min);
    const max = Number(maxInput.max);
    const fromYear = parseYearParam(params.get(fromParam));
    const toYear = parseYearParam(params.get(toParam));
    const selectedMin = Number.isInteger(fromYear) ? Math.min(Math.max(fromYear, min), max) : min;
    const selectedMax = Number.isInteger(toYear) ? Math.min(Math.max(toYear, min), max) : max;

    minInput.value = String(Math.min(selectedMin, selectedMax));
    maxInput.value = String(Math.max(selectedMin, selectedMax));
    minInput.dispatchEvent(new Event('input', { bubbles: true }));
    maxInput.dispatchEvent(new Event('input', { bubbles: true }));
  }

  function syncControlsFromURL() {
    const params = new URLSearchParams(window.location.search);

    syncCheckboxesFromURL(params, 'members');
    syncCheckboxesFromURL(params, 'locations');
    syncRangeFromURL('creation', 'creation_from', 'creation_to');
    syncRangeFromURL('first-album', 'album_from', 'album_to');
    syncFilterParams();
    applyLocationSearch();
  }

  function syncFilterParams() {
    if (!form) {
      return;
    }

    const creation = rangeValues('creation');
    const firstAlbum = rangeValues('first-album');

    if (creation) {
      setHiddenField(
        form.querySelector('[data-filter-param="creation-from"]'),
        creation.selectedMin === creation.min ? '' : String(creation.selectedMin)
      );
      setHiddenField(
        form.querySelector('[data-filter-param="creation-to"]'),
        creation.selectedMax === creation.max ? '' : String(creation.selectedMax)
      );
    }

    if (firstAlbum) {
      setHiddenField(
        form.querySelector('[data-filter-param="album-from"]'),
        firstAlbum.selectedMin === firstAlbum.min ? '' : yearStart(firstAlbum.selectedMin)
      );
      setHiddenField(
        form.querySelector('[data-filter-param="album-to"]'),
        firstAlbum.selectedMax === firstAlbum.max ? '' : yearEnd(firstAlbum.selectedMax)
      );
    }
  }

  function applyLocationSearch() {
    const query = normalizeLocationQuery(locationSearch?.value || '');
    const options = Array.from(document.querySelectorAll('[data-location-option]'));
    let visibleCount = 0;

    for (const option of options) {
      const key = option.dataset.locationSearchKey || normalizeLocationQuery(option.textContent);
      const matched = query === '' || key.includes(query);

      option.hidden = !matched;
      if (matched) {
        visibleCount++;
      }
    }

    if (locationEmpty) {
      locationEmpty.hidden = query === '' || visibleCount > 0;
    }
  }

  function isFilterCheckbox(target) {
    return target?.matches('input[type="checkbox"][name="members"], input[type="checkbox"][name="locations"]');
  }

  function isRangeInput(target) {
    return target?.matches('.dual-range__input');
  }

  function submitFilterForm() {
    if (!form) {
      return;
    }

    syncFilterParams();
    if (window.groupieResults?.submitFilterForm?.(form)) {
      return;
    }

    if (typeof form.requestSubmit === 'function') {
      form.requestSubmit();
    } else {
      form.submit();
    }
  }

  if (form) {
    form.addEventListener('input', (event) => {
      if (isRangeInput(event.target)) {
        syncFilterParams();
      }
    });
    form.addEventListener('change', (event) => {
      syncFilterParams();

      if (!event.isTrusted) {
        return;
      }
      if (isFilterCheckbox(event.target) || isRangeInput(event.target)) {
        submitFilterForm();
      }
    });
    form.addEventListener('submit', (event) => {
      syncFilterParams();
      if (window.groupieResults?.submitFilterForm?.(form)) {
        event.preventDefault();
      }
    });
  }

  document.querySelectorAll('[data-filter-range]').forEach((slider) => {
    slider.addEventListener('filter-range:update', () => {
      window.setTimeout(syncFilterParams, 0);
    });
  });

  if (locationSearch) {
    locationSearch.addEventListener('input', applyLocationSearch);
  }

  document.addEventListener('groupie:results-url-applied', syncControlsFromURL);
  window.addEventListener('pageshow', syncControlsFromURL);

  window.groupieFilters = {
    applyLocationSearch,
    normalizeLocationQuery,
    syncControlsFromURL,
    syncFilterParams,
  };

  syncControlsFromURL();
});
