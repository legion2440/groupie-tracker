document.addEventListener('DOMContentLoaded', () => {
  const form = document.querySelector('[data-search-form]');
  const input = document.getElementById('artist-search');
  const typeInput = document.getElementById('artist-search-type');
  const list = document.getElementById('artist-search-suggestions');
  const status = document.getElementById('artist-search-status');

  if (!form || !input || !typeInput || !list || !status) {
    return;
  }

  const debounceMs = 175;
  const allowedTypes = new Set([
    'artist',
    'member',
    'location',
    'first_album',
    'creation_date',
  ]);

  let debounceTimer = null;
  let activeController = null;
  let activeRequestToken = 0;
  let suggestions = [];
  let activeIndex = -1;

  function trimmedInput() {
    return input.value.trim();
  }

  function clearTypedSearchState() {
    typeInput.value = '';
    typeInput.removeAttribute('name');
  }

  function clearStatus() {
    status.textContent = '';
  }

  function setStatus(message) {
    status.textContent = message;
  }

  function abortActiveRequest() {
    if (activeController) {
      activeController.abort();
      activeController = null;
    }
  }

  function invalidatePendingSuggestions() {
    window.clearTimeout(debounceTimer);
    debounceTimer = null;
    activeRequestToken += 1;
    abortActiveRequest();
  }

  function clearEmptySearchState() {
    invalidatePendingSuggestions();
    clearTypedSearchState();
    clearSuggestions();
    clearStatus();
  }

  function omitEmptySearchFieldsForSubmit() {
    if (trimmedInput() === '') {
      input.removeAttribute('name');
    }
    if (typeInput.value === '') {
      typeInput.removeAttribute('name');
    } else {
      typeInput.setAttribute('name', 'search_type');
    }
  }

  function emptySearchActionURL() {
    const target = new URL(form.getAttribute('action') || window.location.href, window.location.origin);
    const params = new URLSearchParams();
    const data = new FormData(form);

    data.delete('q');
    data.delete('search_type');
    data.forEach((value, key) => {
      params.append(key, value);
    });

    target.search = params.toString();
    target.hash = '';
    return target.href;
  }

  function submitEmptySearchForm() {
    omitEmptySearchFieldsForSubmit();
    if (window.groupieResults?.clearSearchForm?.(form)) {
      return;
    }
    window.location.href = emptySearchActionURL();
  }

  function submitSearchForm() {
    omitEmptySearchFieldsForSubmit();
    if (window.groupieResults?.submitSearchForm?.(form)) {
      return;
    }
    if (typeof form.requestSubmit === 'function') {
      form.requestSubmit();
    } else {
      form.submit();
    }
  }

  function syncControlsFromURL() {
    const params = new URLSearchParams(window.location.search);
    const searchText = params.get('q') || '';
    const searchType = params.get('search_type') || '';

    input.value = searchText;
    typeInput.value = searchText.trim() !== '' ? searchType : '';
    if (typeInput.value === '') {
      typeInput.removeAttribute('name');
    } else {
      typeInput.setAttribute('name', 'search_type');
    }
    clearSuggestions();
    clearStatus();
  }

  function setExpanded(expanded) {
    input.setAttribute('aria-expanded', String(expanded));
  }

  function clearActiveOption() {
    activeIndex = -1;
    input.removeAttribute('aria-activedescendant');
    Array.from(list.children).forEach((option) => {
      option.setAttribute('aria-selected', 'false');
      option.classList.remove('is-active');
    });
  }

  function closeSuggestions() {
    list.hidden = true;
    setExpanded(false);
    clearActiveOption();
  }

  function clearSuggestions() {
    suggestions = [];
    list.replaceChildren();
    closeSuggestions();
  }

  function isOpen() {
    return !list.hidden;
  }

  function renderSuggestions(items) {
    suggestions = items;
    list.replaceChildren();
    clearActiveOption();

    if (items.length === 0) {
      closeSuggestions();
      setStatus('No suggestions');
      return;
    }

    const fragment = document.createDocumentFragment();
    items.forEach((item, index) => {
      const option = document.createElement('li');
      option.id = `artist-search-option-${index}`;
      option.className = 'search-suggestion';
      option.setAttribute('role', 'option');
      option.setAttribute('aria-selected', 'false');
      option.dataset.index = String(index);
      option.dataset.value = item.value;
      option.dataset.type = item.type;
      option.textContent = item.label;

      option.addEventListener('pointerenter', () => {
        setActiveOption(index);
      });

      option.addEventListener('pointerdown', (event) => {
        event.preventDefault();
        selectSuggestion(index);
      });

      fragment.appendChild(option);
    });

    list.appendChild(fragment);
    list.hidden = false;
    setExpanded(true);
    setStatus(`${items.length} suggestions available`);
  }

  function setActiveOption(index) {
    if (suggestions.length === 0) {
      clearActiveOption();
      return;
    }

    activeIndex = (index + suggestions.length) % suggestions.length;
    Array.from(list.children).forEach((option, optionIndex) => {
      const active = optionIndex === activeIndex;
      option.setAttribute('aria-selected', String(active));
      option.classList.toggle('is-active', active);
      if (active) {
        input.setAttribute('aria-activedescendant', option.id);
        option.scrollIntoView({ block: 'nearest' });
      }
    });
  }

  function selectSuggestion(index) {
    const suggestion = suggestions[index];
    if (!suggestion) {
      return;
    }

    input.value = suggestion.value;
    typeInput.value = suggestion.type;
    typeInput.setAttribute('name', 'search_type');
    closeSuggestions();

    submitSearchForm();
  }

  function validSuggestion(item) {
    if (!item || typeof item !== 'object') {
      return null;
    }
    if (
      typeof item.value !== 'string' ||
      typeof item.label !== 'string' ||
      typeof item.type !== 'string' ||
      !allowedTypes.has(item.type)
    ) {
      return null;
    }
    return {
      value: item.value,
      label: item.label,
      type: item.type,
    };
  }

  async function loadSuggestions(query, token) {
    abortActiveRequest();
    activeController = new AbortController();

    const endpoint = new URL('/api/search/suggestions', window.location.origin);
    endpoint.searchParams.set('q', query);

    try {
      const response = await fetch(endpoint, {
        method: 'GET',
        signal: activeController.signal,
        headers: {
          Accept: 'application/json',
        },
      });

      if (token !== activeRequestToken) {
        return;
      }

      if (!response.ok) {
        throw new Error(`Suggestions failed: ${response.status}`);
      }

      const data = await response.json();
      if (!Array.isArray(data)) {
        throw new Error('Invalid suggestions response');
      }

      const items = data
        .map(validSuggestion)
        .filter((item) => item !== null);

      renderSuggestions(items);
    } catch (error) {
      if (error.name === 'AbortError') {
        return;
      }
      if (token !== activeRequestToken) {
        return;
      }
      clearSuggestions();
      setStatus('Suggestions unavailable');
    } finally {
      if (token === activeRequestToken) {
        activeController = null;
      }
    }
  }

  function scheduleSuggestions() {
    window.clearTimeout(debounceTimer);
    debounceTimer = null;

    const query = trimmedInput();
    if (query === '') {
      clearEmptySearchState();
      return;
    }

    const token = ++activeRequestToken;
    debounceTimer = window.setTimeout(() => {
      loadSuggestions(query, token);
    }, debounceMs);
  }

  input.addEventListener('input', () => {
    clearTypedSearchState();
    clearActiveOption();
    scheduleSuggestions();
  });

  input.addEventListener('search', () => {
    if (trimmedInput() === '') {
      clearEmptySearchState();
      submitEmptySearchForm();
      return;
    }
    clearTypedSearchState();
    clearActiveOption();
    scheduleSuggestions();
  });

  input.addEventListener('focus', () => {
    if (trimmedInput() === '') {
      return;
    }
    if (suggestions.length > 0) {
      list.hidden = false;
      setExpanded(true);
      return;
    }
    scheduleSuggestions();
  });

  input.addEventListener('keydown', (event) => {
    if (event.key === 'ArrowDown' && suggestions.length > 0) {
      event.preventDefault();
      if (!isOpen()) {
        list.hidden = false;
        setExpanded(true);
      }
      setActiveOption(activeIndex < 0 ? 0 : activeIndex + 1);
      return;
    }

    if (event.key === 'ArrowUp' && suggestions.length > 0) {
      event.preventDefault();
      if (!isOpen()) {
        list.hidden = false;
        setExpanded(true);
      }
      setActiveOption(activeIndex < 0 ? suggestions.length - 1 : activeIndex - 1);
      return;
    }

    if (event.key === 'Enter' && activeIndex >= 0 && isOpen()) {
      event.preventDefault();
      selectSuggestion(activeIndex);
      return;
    }

    if (event.key === 'Enter') {
      event.preventDefault();
      closeSuggestions();
      submitSearchForm();
      return;
    }

    if (event.key === 'Escape' && isOpen()) {
      event.preventDefault();
      closeSuggestions();
      return;
    }

    if (event.key === 'Tab') {
      closeSuggestions();
    }
  });

  form.addEventListener('submit', (event) => {
    if (trimmedInput() === '') {
      event.preventDefault();
      clearTypedSearchState();
      closeSuggestions();
      submitEmptySearchForm();
      return;
    }
    omitEmptySearchFieldsForSubmit();
    closeSuggestions();
    if (window.groupieResults?.submitSearchForm?.(form)) {
      event.preventDefault();
    }
  });

  document.addEventListener('pointerdown', (event) => {
    if (!form.contains(event.target)) {
      closeSuggestions();
    }
  });

  document.addEventListener('groupie:results-url-applied', syncControlsFromURL);
  window.addEventListener('pageshow', syncControlsFromURL);

  window.groupieSearch = {
    syncControlsFromURL,
  };
});
