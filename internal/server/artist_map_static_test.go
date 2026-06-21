package server

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func TestArtistMapStaticUsesRealCoordinates(t *testing.T) {
	content, err := staticFS.ReadFile("static/artist-map.js")
	if err != nil {
		t.Fatalf("read artist-map.js: %v", err)
	}
	js := string(content)
	for _, forbidden := range []string{
		"TEMPORARY_MAP_POSITIONS",
		"getTemporaryPosition",
	} {
		if strings.Contains(js, forbidden) {
			t.Fatalf("artist-map.js must not contain %q", forbidden)
		}
	}
	for _, want := range []string{
		"computeTourCamera",
		"dataset.latitude",
		"dataset.longitude",
		"projectGeoWithCamera",
		"projectGeoToViewport",
	} {
		if !strings.Contains(js, want) {
			t.Fatalf("artist-map.js should contain %q", want)
		}
	}
}

func TestArtistMapHasAutoFitWithoutManualZoomControls(t *testing.T) {
	jsContent, err := staticFS.ReadFile("static/artist-map.js")
	if err != nil {
		t.Fatalf("read artist-map.js: %v", err)
	}
	cssContent, err := staticFS.ReadFile("static/style.css")
	if err != nil {
		t.Fatalf("read style.css: %v", err)
	}
	templateContent, err := tmplFS.ReadFile("templates/artist.html")
	if err != nil {
		t.Fatalf("read artist.html: %v", err)
	}

	combined := string(jsContent) + "\n" +
		string(cssContent) + "\n" +
		string(templateContent)
	for _, forbidden := range []string{
		"tour-map__zoom",
		"tour-map__zoom-in",
		"tour-map__zoom-out",
		"zoomControl",
		"resetCamera",
		"addEventListener(\"wheel\"",
		"addEventListener('wheel'",
	} {
		if strings.Contains(combined, forbidden) {
			t.Fatalf("Tour Map must not add manual control %q", forbidden)
		}
	}

	js := string(jsContent)
	for _, want := range []string{
		"computeTourCamera(",
		"projectGeoWithCamera(",
		"applyLandCamera(landLayer, camera, viewport);",
		"const resizeObserver = new ResizeObserver",
	} {
		if !strings.Contains(js, want) {
			t.Fatalf("artist-map.js should contain %q", want)
		}
	}

	selectLocationIndex := strings.Index(js, "function selectLocation")
	if selectLocationIndex < 0 {
		t.Fatal("artist-map.js should contain selectLocation")
	}
	cardsListenerIndex := strings.Index(js[selectLocationIndex:], "  cards.forEach")
	if cardsListenerIndex < 0 {
		t.Fatal("artist-map.js should wire card listeners after selectLocation")
	}
	selectLocationBody := js[selectLocationIndex : selectLocationIndex+cardsListenerIndex]
	if strings.Contains(selectLocationBody, "computeTourCamera") {
		t.Fatal("selecting a card must not recompute the Tour Map camera")
	}
}

func windowsPathFromWSL(path string) string {
	if !strings.HasPrefix(path, "/mnt/") || len(path) < len("/mnt/d/") {
		return path
	}

	drive := strings.ToUpper(path[5:6])
	rest := strings.ReplaceAll(path[len("/mnt/d/"):], "/", `\`)

	return drive + `:\` + rest
}

func TestArtistMapProjection(t *testing.T) {
	nodePath := os.Getenv("NODE_BINARY")
	if nodePath == "" {
		var err error
		for _, name := range []string{"node", "node.exe"} {
			nodePath, err = exec.LookPath(name)
			if err == nil {
				break
			}
		}
		if nodePath == "" {
			t.Fatalf("node is required for artist-map.js projection test: %v", err)
		}
	}
	jsPath, err := filepath.Abs(filepath.Join("static", "artist-map.js"))
	if err != nil {
		t.Fatalf("resolve artist-map.js path: %v", err)
	}
	if strings.HasSuffix(strings.ToLower(nodePath), ".exe") {
		jsPath = windowsPathFromWSL(jsPath)
	}
	script := `
const assert = require("assert");
const map = require(process.argv[1]);

function close(actual, expected) {
  assert.ok(Math.abs(actual - expected) < 0.000001, actual + " != " + expected);
}

function assertFiniteCamera(camera) {
  assert.ok(Number.isFinite(camera.centerX), "centerX should be finite");
  assert.ok(Number.isFinite(camera.centerY), "centerY should be finite");
  assert.ok(Number.isFinite(camera.zoom), "zoom should be finite");
}

function assertInsidePadding(locations, camera, width, height) {
  const paddingX = width * map.TOUR_MAP_PADDING_RATIO;
  const paddingY = height * map.TOUR_MAP_PADDING_RATIO;

  for (const location of locations) {
    const point = map.projectGeoWithCamera(
      location.latitude,
      location.longitude,
      camera,
      width,
      height
    );

    assert.ok(
      point.x >= paddingX - 0.000001 &&
        point.x <= width - paddingX + 0.000001,
      "x outside padding: " + JSON.stringify(point)
    );
    assert.ok(
      point.y >= paddingY - 0.000001 &&
        point.y <= height - paddingY + 0.000001,
      "y outside padding: " + JSON.stringify(point)
    );
  }
}

function sameCamera(actual, expected) {
  close(actual.centerX, expected.centerX);
  close(actual.centerY, expected.centerY);
  close(actual.zoom, expected.zoom);
}

let p = map.projectGeoToViewBox(0, 0);
close(p.x, 500);
close(p.y, 250);

p = map.projectGeoToViewBox(90, -180);
close(p.x, 0);
close(p.y, 0);

p = map.projectGeoToViewBox(-90, 180);
close(p.x, 1000);
close(p.y, 500);

let rect = map.getContainedMapRect({ clientWidth: 1200, clientHeight: 500 });
close(rect.left, 100);
close(rect.top, 0);
close(rect.width, 1000);
close(rect.height, 500);

rect = map.getContainedMapRect({ clientWidth: 800, clientHeight: 800 });
close(rect.left, 0);
close(rect.top, 200);
close(rect.width, 800);
close(rect.height, 400);

p = map.projectGeoToViewport(0, 0, { clientWidth: 1200, clientHeight: 500 });
close(p.x, 600);
close(p.y, 250);

const worldCamera = map.computeTourCamera([
  { latitude: 40.7128, longitude: -74.0060 },
  { latitude: 35.6762, longitude: 139.6503 },
  { latitude: -33.8688, longitude: 151.2093 },
], 1000, 500);
assertFiniteCamera(worldCamera);
close(worldCamera.centerX, 500);
close(worldCamera.centerY, 250);
close(worldCamera.zoom, 1);

const europe = [
  { latitude: 45.7640, longitude: 4.8357 },
  { latitude: 46.5197, longitude: 6.6323 },
  { latitude: 51.5074, longitude: -0.1278 },
];
const europeCamera = map.computeTourCamera(europe, 1000, 500);
assertFiniteCamera(europeCamera);
assert.ok(europeCamera.zoom > 1, "European cluster should zoom in");
assertInsidePadding(europe, europeCamera, 1000, 500);

const centerLongitude =
  (europeCamera.centerX / 1000) * 360 - 180;
const centerLatitude =
  90 - (europeCamera.centerY / 500) * 180;
p = map.projectGeoWithCamera(
  centerLatitude,
  centerLongitude,
  europeCamera,
  1000,
  500
);
close(p.x, 500);
close(p.y, 250);

const singleCamera = map.computeTourCamera([
  { latitude: 48.8566, longitude: 2.3522 },
], 1000, 500);
assertFiniteCamera(singleCamera);
close(singleCamera.zoom, map.TOUR_MAP_SINGLE_POINT_ZOOM);
p = map.projectGeoWithCamera(48.8566, 2.3522, singleCamera, 1000, 500);
close(p.x, 500);
close(p.y, 250);

const nearDuplicateCamera = map.computeTourCamera([
  { latitude: 48.856600, longitude: 2.352200 },
  { latitude: 48.856601, longitude: 2.352201 },
], 1000, 500);
assertFiniteCamera(nearDuplicateCamera);
close(nearDuplicateCamera.zoom, map.TOUR_MAP_MAX_ZOOM);

const zeroSpanCamera = map.computeTourCamera([
  { latitude: 10, longitude: 20 },
  { latitude: 10, longitude: 20 },
], 1000, 500);
assertFiniteCamera(zeroSpanCamera);
close(zeroSpanCamera.zoom, map.TOUR_MAP_MAX_ZOOM);

sameCamera(
  map.computeTourCamera(europe, 1000, 500),
  map.computeTourCamera([...europe].reverse(), 1000, 500)
);

const portraitCamera = map.computeTourCamera(europe, 390, 844);
assertFiniteCamera(portraitCamera);
assert.ok(portraitCamera.zoom > 1, "portrait viewport should still zoom compact cluster");
assertInsidePadding(europe, portraitCamera, 390, 844);

const antimeridian = [
  { latitude: 10, longitude: 170 },
  { latitude: 12, longitude: -170 },
];
const antimeridianCamera = map.computeTourCamera(antimeridian, 1000, 500);
assertFiniteCamera(antimeridianCamera);
assert.ok(antimeridianCamera.zoom > 1, "antimeridian cluster should zoom in");
assertInsidePadding(antimeridian, antimeridianCamera, 1000, 500);
const antiA = map.projectGeoWithCamera(10, 170, antimeridianCamera, 1000, 500);
const antiB = map.projectGeoWithCamera(12, -170, antimeridianCamera, 1000, 500);
assert.ok(Math.abs(antiA.x - antiB.x) < 300, "antimeridian points should project near each other");

const ordinaryWideCamera = map.computeTourCamera([
  { latitude: 0, longitude: -120 },
  { latitude: 0, longitude: 120 },
], 1000, 500);
assertFiniteCamera(ordinaryWideCamera);
close(ordinaryWideCamera.centerX, 500);
close(ordinaryWideCamera.centerY, 250);
close(ordinaryWideCamera.zoom, 1);

const clamped = map.projectGeoWithCamera(120, 200, {
  centerX: 500,
  centerY: 250,
  zoom: 1,
}, 1000, 500);
const expectedClamped = map.projectGeoWithCamera(90, 180, {
  centerX: 500,
  centerY: 250,
  zoom: 1,
}, 1000, 500);
close(clamped.x, expectedClamped.x);
close(clamped.y, expectedClamped.y);
`
	cmd := exec.Command(nodePath, "-e", script, jsPath)
	output, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("node projection test failed: %v\n%s", err, output)
	}
}
