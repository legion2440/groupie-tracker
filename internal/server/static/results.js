(function () {
  const resultsSelector = '[data-results-container]';
  const filterKeys = new Set([
    'creation_from',
    'creation_to',
    'album_from',
    'album_to',
    'members',
    'locations',
  ]);

  let activeController = null;
  let latestRequestID = 0;

  function currentResultsContainer() {
    return document.querySelector(resultsSelector);
  }

  function canUpdate(url) {
    return Boolean(currentResultsContainer()) &&
      url.origin === window.location.origin &&
      url.pathname === '/';
  }

  function cleanParams(params) {
    const query = params.get('q');
    if (query === null || query.trim() === '') {
      params.delete('q');
      params.delete('search_type');
    }
    if ((params.get('search_type') || '') === '') {
      params.delete('search_type');
    }
  }

  function urlFromParams(params) {
    cleanParams(params);

    const target = new URL('/', window.location.origin);
    target.search = params.toString();
    target.hash = '';
    return target;
  }

  function searchURLFromForm(form, clearSearch) {
    const params = new URLSearchParams(window.location.search);
    params.delete('q');
    params.delete('search_type');

    if (!clearSearch) {
      const input = form.querySelector('input[name="q"], #artist-search');
      const typeInput = form.querySelector('#artist-search-type');
      const searchText = input?.value || '';

      if (searchText.trim() !== '') {
        params.set('q', searchText);
        if (typeInput?.value) {
          params.set('search_type', typeInput.value);
        }
      }
    }

    return urlFromParams(params);
  }

  function filterURLFromForm(form) {
    const params = new URLSearchParams(window.location.search);
    filterKeys.forEach((key) => params.delete(key));

    const data = new FormData(form);
    data.forEach((value, key) => {
      if (filterKeys.has(key) && String(value) !== '') {
        params.append(key, value);
      }
    });

    return urlFromParams(params);
  }

  function dispatchURLApplied(url) {
    document.dispatchEvent(new CustomEvent('groupie:results-url-applied', {
      detail: { url },
    }));
  }

  function replaceResultsFromDocument(doc) {
    const current = currentResultsContainer();
    const next = doc.querySelector(resultsSelector);

    if (!current || !next) {
      throw new Error('Results container not found in response');
    }

    const nodes = Array.from(next.childNodes).map((node) => document.importNode(node, true));
    current.replaceChildren(...nodes);
  }

  async function navigate(url, historyMode) {
    if (!canUpdate(url)) {
      return false;
    }

    if (activeController) {
      activeController.abort();
    }

    const requestID = latestRequestID + 1;
    const controller = new AbortController();
    activeController = controller;
    latestRequestID = requestID;

    try {
      const response = await fetch(url.href, {
        method: 'GET',
        credentials: 'same-origin',
        signal: controller.signal,
        headers: {
          Accept: 'text/html',
        },
      });

      if (!response.ok) {
        throw new Error(`Results request failed: ${response.status}`);
      }

      const html = await response.text();
      if (requestID !== latestRequestID) {
        return true;
      }

      const doc = new DOMParser().parseFromString(html, 'text/html');
      replaceResultsFromDocument(doc);

      if (historyMode === 'push') {
        window.history.pushState({ groupieResults: true }, '', url.href);
      } else if (historyMode === 'replace') {
        window.history.replaceState({ groupieResults: true }, '', url.href);
      }

      dispatchURLApplied(url);
      return true;
    } catch (error) {
      if (error.name !== 'AbortError') {
        console.error('Results update failed:', error);
      }
      return false;
    } finally {
      if (activeController === controller) {
        activeController = null;
      }
    }
  }

  function startNavigation(url, historyMode) {
    if (!canUpdate(url)) {
      return false;
    }

    void navigate(url, historyMode);
    return true;
  }

  window.addEventListener('popstate', () => {
    const url = new URL(window.location.href);
    if (!canUpdate(url)) {
      return;
    }

    void navigate(url, 'none');
  });

  window.groupieResults = {
    clearSearchForm(form) {
      return startNavigation(searchURLFromForm(form, true), 'push');
    },
    submitSearchForm(form) {
      return startNavigation(searchURLFromForm(form, false), 'push');
    },
    submitFilterForm(form) {
      return startNavigation(filterURLFromForm(form), 'push');
    },
    syncCurrentURL() {
      dispatchURLApplied(new URL(window.location.href));
    },
  };
})();
