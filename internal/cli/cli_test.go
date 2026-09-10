package cli

import (
	"bytes"
	"encoding/csv"
	"encoding/json"
	"errors"
	"image/gif"
	"image/png"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/TheFellow/the-line/pkg/solver"
	"github.com/TheFellow/the-line/pkg/track"
)

func TestWorkflow(t *testing.T) {
	dir := t.TempDir()
	var stdout, stderr bytes.Buffer
	run := func(args ...string) {
		t.Helper()
		stdout.Reset()
		stderr.Reset()
		if err := Run(args, &stdout, &stderr); err != nil {
			t.Fatalf("%v: %v\n%s", args, err, stderr.String())
		}
	}
	scene := filepath.Join(dir, "corner.json")
	run("new", "--preset", "esses", "--out", scene)
	run("validate", "--scene", scene)
	if !strings.Contains(stdout.String(), "Valid:") {
		t.Fatal(stdout.String())
	}
	run("solve", "--scene", scene, "--iterations", "1")
	var solved struct {
		Scene  track.Scene
		Result solver.Result
	}
	if err := json.Unmarshal(stdout.Bytes(), &solved); err != nil {
		t.Fatal(err)
	}
	if solved.Result.Duration <= 0 || solved.Result.Duration > solved.Result.CenterDuration+1e-8 {
		t.Fatalf("invalid time comparison: %+v", solved.Result)
	}
	if len(solved.Result.CenterNodes) == 0 || solved.Result.CenterNodes[len(solved.Result.CenterNodes)-1].Time != solved.Result.CenterDuration {
		t.Fatal("JSON omits verified centreline trajectory")
	}
	run("solve", "--scene", scene, "--iterations", "1", "--format", "csv")
	rows, err := csv.NewReader(&stdout).ReadAll()
	if err != nil {
		t.Fatal(err)
	}
	if len(rows) < 10 || rows[0][1] != "time_s" || rows[0][len(rows[0])-1] != "station_m" {
		t.Fatal("missing telemetry")
	}
	for _, view := range []string{"2d", "3d"} {
		path := filepath.Join(dir, view+".png")
		run("render", "--scene", scene, "--iterations", "1", "--view", view, "--width", "800", "--height", "600", "--out", path)
		file, err := os.Open(path)
		if err != nil {
			t.Fatal(err)
		}
		img, err := png.Decode(file)
		file.Close()
		if err != nil {
			t.Fatal(err)
		}
		if img.Bounds().Dx() != 800 || img.Bounds().Dy() != 600 {
			t.Fatal(img.Bounds())
		}
	}
	animation := filepath.Join(dir, "sequence.gif")
	run("animate", "--scene", scene, "--iterations", "1", "--duration", "0.2", "--fps", "10", "--width", "800", "--height", "600", "--out", animation)
	file, err := os.Open(animation)
	if err != nil {
		t.Fatal(err)
	}
	defer file.Close()
	frames, err := gif.DecodeAll(file)
	if err != nil {
		t.Fatal(err)
	}
	if len(frames.Image) != 2 || frames.Delay[0] != 10 {
		t.Fatal("incorrect animation timing")
	}
	if bytes.Equal(frames.Image[0].Pix, frames.Image[1].Pix) {
		t.Fatal("animation frames do not move")
	}
}

func TestBadInputReturnsUsefulErrors(t *testing.T) {
	cases := [][]string{{"nope"}, {"new"}, {"solve", "--format", "xml"}, {"render", "--out", "x.png", "--view", "sideways"}, {"solve", "--spacing", "NaN"}, {"solve", "--vehicle", "unknown"}, {"animate", "--out", "x.gif", "--fps", "0"}, {"solve", "extra"}}
	for _, args := range cases {
		t.Run(strings.Join(args, "_"), func(t *testing.T) {
			if err := Run(args, io.Discard, io.Discard); err == nil {
				t.Fatal("expected validation error")
			}
		})
	}
}

func TestOutputFailurePreservesExistingFile(t *testing.T) {
	path := filepath.Join(t.TempDir(), "saved.json")
	if err := os.WriteFile(path, []byte("previous"), 0600); err != nil {
		t.Fatal(err)
	}
	sentinel := errors.New("encoding failed")
	err := output(path, func(w io.Writer) error { io.WriteString(w, "partial"); return sentinel })
	if !errors.Is(err, sentinel) {
		t.Fatal(err)
	}
	b, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if string(b) != "previous" {
		t.Fatal("failed output destroyed prior data")
	}
}
