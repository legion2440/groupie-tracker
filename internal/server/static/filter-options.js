document.addEventListener('DOMContentLoaded', () => {
  const ranges = {
    creation: document.querySelector('[data-filter-range="creation"]'),
    firstAlbum: document.querySelector('[data-filter-range="first-album"]'),
  };
  const memberOptionsContainer = document.querySelector('[data-member-options]');
  const locationOptionsContainer = document.querySelector('[data-location-options]');

  let optionsController = null;
  let latestOptionsRequest = 0;

  function isIntegerRange(value) {
    return value &&
      Number.isInteger(value.min) &&
      Number.isInteger(value.max) &&
      value.min <= value.max;
  }

  function validateFilterOptions(data) {
    if (!data || typeof data !== 'object') {
      return null;
    }

    if (!isIntegerRange(data.creation_year) || !isIntegerRange(data.first_album_year)) {
      return null;
    }

    if (!Array.isArray(data.member_counts) ||
      !data.member_counts.every((count) => Number.isInteger(count) && count >= 1 && count <= 8)) {
      return null;
    }

    if (!Array.isArray(data.locations) ||
      !data.locations.every((location) =>
        location &&
        typeof location.value === 'string' &&
        typeof location.label === 'string'
      )) {
      return null;
    }

    return {
      creationYear: data.creation_year,
      firstAlbumYear: data.first_album_year,
      memberCounts: data.member_counts,
      locations: data.locations,
    };
  }

  function dispatchRangeUpdate(slider, range) {
    if (!slider) {
      return;
    }

    slider.dispatchEvent(new CustomEvent('filter-range:update', {
      detail: {
        min: range.min,
        max: range.max,
      },
    }));
  }

  function applyFilterOptions(options) {
    dispatchRangeUpdate(ranges.creation, options.creationYear);
    dispatchRangeUpdate(ranges.firstAlbum, options.firstAlbumYear);
    renderMemberOptions(options.memberCounts);
    renderLocationOptions(options.locations);
    window.groupieFilters?.syncFilterParams();
    window.groupieFilters?.applyLocationSearch();
  }

  function checkedValues(container, selector) {
    if (!container) {
      return new Set();
    }

    return new Set(
      Array.from(container.querySelectorAll(selector))
        .filter((input) => input.checked)
        .map((input) => input.value)
    );
  }

  function renderMemberOptions(memberCounts) {
    if (!memberOptionsContainer) {
      return;
    }

    const selected = checkedValues(memberOptionsContainer, 'input[name="members"]');
    memberOptionsContainer.textContent = '';

    for (const count of memberCounts) {
      const value = count === 8 ? '8+' : String(count);
      const label = document.createElement('label');
      label.className = 'member-button';

      const input = document.createElement('input');
      input.className = 'member-button__input visually-hidden';
      input.type = 'checkbox';
      input.name = 'members';
      input.value = value;
      input.checked = selected.has(value);

      const text = document.createElement('span');
      text.textContent = value;

      label.append(input, text);
      memberOptionsContainer.append(label);
    }
  }

  function renderLocationOptions(locations) {
    if (!locationOptionsContainer) {
      return;
    }

    const selected = checkedValues(locationOptionsContainer, 'input[name="locations"]');
    locationOptionsContainer.textContent = '';

    for (const option of locations) {
      const label = document.createElement('label');
      label.className = 'location-option';
      label.dataset.locationOption = '';
      label.dataset.locationSearchKey = window.groupieFilters?.normalizeLocationQuery(option.label) || option.label.toLowerCase();

      const input = document.createElement('input');
      input.type = 'checkbox';
      input.name = 'locations';
      input.value = option.value;
      input.checked = selected.has(option.value);

      const text = document.createElement('span');
      text.textContent = option.label;

      label.append(input, text);
      locationOptionsContainer.append(label);
    }
  }

  async function loadFilterOptions() {
    if (optionsController) {
      optionsController.abort();
    }

    const controller = new AbortController();
    const requestID = latestOptionsRequest + 1;

    optionsController = controller;
    latestOptionsRequest = requestID;

    try {
      const response = await fetch('/api/filter-options', {
        cache: 'no-store',
        signal: controller.signal,
      });

      if (!response.ok) {
        throw new Error(`Filter options request failed: ${response.status}`);
      }

      const options = validateFilterOptions(await response.json());

      if (!options || requestID !== latestOptionsRequest) {
        return;
      }

      applyFilterOptions(options);
    } catch (error) {
      if (error.name !== 'AbortError') {
        console.error('Filter options unavailable:', error);
      }
    } finally {
      if (optionsController === controller) {
        optionsController = null;
      }
    }
  }

  document.addEventListener('groupie:catalog-refreshed', loadFilterOptions);
  loadFilterOptions();
});
