package server

import (
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
		"dataset.latitude",
		"dataset.longitude",
		"projectGeoToViewport",
	} {
		if !strings.Contains(js, want) {
			t.Fatalf("artist-map.js should contain %q", want)
		}
	}
}

func TestArtistMapProjection(t *testing.T) {
	nodePath, err := exec.LookPath("node")
	if err != nil {
		t.Fatalf("node is required for artist-map.js projection test: %v", err)
	}
	jsPath, err := filepath.Abs(filepath.Join("static", "artist-map.js"))
	if err != nil {
		t.Fatalf("resolve artist-map.js path: %v", err)
	}
	script := `
const assert = require("assert");
const map = require(process.argv[1]);

function close(actual, expected) {
  assert.ok(Math.abs(actual - expected) < 0.000001, actual + " != " + expected);
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
`
	cmd := exec.Command(nodePath, "-e", script, jsPath)
	output, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("node projection test failed: %v\n%s", err, output)
	}
}
