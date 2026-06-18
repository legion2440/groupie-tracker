package server

import (
	"strings"
	"testing"
)

func TestSearchScriptNativeClearSubmitsWithoutEmptySearchParams(t *testing.T) {
	content, err := staticFS.ReadFile("static/search.js")
	if err != nil {
		t.Fatalf("read search script: %v", err)
	}
	script := string(content)

	for _, want := range []string{
		"input.addEventListener('search'",
		"clearEmptySearchState();",
		"submitSearchForm();",
		"submitEmptySearchForm();",
		"input.removeAttribute('name');",
		"typeInput.removeAttribute('name');",
		"typeInput.setAttribute('name', 'search_type');",
		"data.delete('q');",
		"data.delete('search_type');",
		"window.groupieResults?.clearSearchForm?.(form)",
		"window.groupieResults?.submitSearchForm?.(form)",
		"document.addEventListener('groupie:results-url-applied', syncControlsFromURL);",
	} {
		if !strings.Contains(script, want) {
			t.Fatalf("expected search.js to contain %q", want)
		}
	}
	for _, forbidden := range []string{
		"innerHTML",
		"insertAdjacentHTML",
		".artist-card",
		"artist-grid",
		"window.location.assign",
		"history.pushState",
		"history.replaceState",
	} {
		if strings.Contains(script, forbidden) {
			t.Fatalf("search.js must not contain %q", forbidden)
		}
	}
}

func TestResultsScriptPartialUpdateContracts(t *testing.T) {
	content, err := staticFS.ReadFile("static/results.js")
	if err != nil {
		t.Fatalf("read results script: %v", err)
	}
	script := string(content)

	for _, want := range []string{
		"[data-results-container]",
		"fetch(url.href",
		"Accept: 'text/html'",
		"new DOMParser().parseFromString(html, 'text/html')",
		"current.replaceChildren(...nodes);",
		"window.history.pushState",
		"window.addEventListener('popstate'",
		"groupie:results-url-applied",
		"clearSearchForm(form)",
		"submitSearchForm(form)",
		"submitFilterForm(form)",
		"params.delete('q');",
		"params.delete('search_type');",
		"params.append(key, value);",
	} {
		if !strings.Contains(script, want) {
			t.Fatalf("expected results.js to contain %q", want)
		}
	}
	for _, forbidden := range []string{
		"innerHTML",
		"insertAdjacentHTML",
		".artist-card",
		"artist-grid",
		"window.location.assign",
	} {
		if strings.Contains(script, forbidden) {
			t.Fatalf("results.js must not contain %q", forbidden)
		}
	}
}

func TestPreviewsScriptHoverAudioContracts(t *testing.T) {
	content, err := staticFS.ReadFile("static/previews.js")
	if err != nil {
		t.Fatalf("read previews script: %v", err)
	}
	script := string(content)

	for _, want := range []string{
		".artist-card",
		".record[data-preview-artist]",
		".artist-vinyl[data-preview-artist]",
		"document.addEventListener('pointerover'",
		"document.addEventListener('pointerout'",
		"document.addEventListener('mouseenter'",
		"document.addEventListener('mouseleave'",
		"new URL('/api/deezer-preview'",
		"endpoint.searchParams.set('artist', artist);",
		"previewCache.set(key, request);",
		"normalizePreview(data)",
		"console.info('[Deezer preview] ' + preview.artist + ' — ' + preview.title);",
		"let activeAudio = null;",
		"let activeToken = 0;",
		"stopActivePreview();",
		"target.matches(previewSourceSelector)",
		"target.querySelector(previewSourceSelector)",
		"if (activeTarget === target)",
		"if (activeToken !== token || activeTarget !== target || !preview.preview) return;",
		"target instanceof Node && element.contains(target)",
		"new Audio(preview.preview)",
		"audio.play();",
		"audio.pause();",
		"audio.currentTime = 0;",
		"}).catch(() =>",
	} {
		if !strings.Contains(script, want) {
			t.Fatalf("expected previews.js to contain %q", want)
		}
	}
	for _, forbidden := range []string{
		"preventDefault",
		"stopPropagation",
		"innerHTML",
		"insertAdjacentHTML",
		"classList.add",
		"classList.remove",
		"textContent",
	} {
		if strings.Contains(script, forbidden) {
			t.Fatalf("previews.js must not contain %q", forbidden)
		}
	}
}
