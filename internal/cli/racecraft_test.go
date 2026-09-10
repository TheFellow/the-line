package cli

import (
	"bytes"
	"encoding/csv"
	"encoding/json"
	"errors"
	"fmt"
	"image/gif"
	"image/png"
	"io"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/TheFellow/the-line/pkg/racecraft"
	"github.com/TheFellow/the-line/pkg/track"
	"github.com/TheFellow/the-line/pkg/vehicle"
)

func TestRaceExports(t *testing.T) {
	dir := t.TempDir()
	file := filepath.Join(dir, "experiment.json")
	var out, errout bytes.Buffer
	if err := Run([]string{"race", "--scenario", "pass-repass", "--save-experiment", file}, &out, &errout); err != nil {
		t.Fatal(err)
	}
	var result racecraft.Result
	if err := json.Unmarshal(out.Bytes(), &result); err != nil {
		t.Fatal(err)
	}
	if len(result.Events) != 2 {
		t.Fatal("missing pass and repass")
	}
	out.Reset()
	if err := Run([]string{"race", "--experiment", file, "--format", "csv"}, &out, &errout); err != nil {
		t.Fatal(err)
	}
	rows, err := csv.NewReader(&out).ReadAll()
	if err != nil || len(rows) < 200 || len(rows[0]) != 7 {
		t.Fatalf("bad telemetry: %v", err)
	}
	if rows[0][6] != "sampled_body_clearance_m" {
		t.Fatalf("ambiguous clearance column: %q", rows[0][6])
	}
	for _, format := range []string{"png", "gif"} {
		path := filepath.Join(dir, "race."+format)
		if err := Run([]string{"race", "--experiment", file, "--format", format, "--out", path, "--duration", ".1", "--fps", "10"}, &out, &errout); err != nil {
			t.Fatal(err)
		}
		f, err := os.Open(path)
		if err != nil {
			t.Fatal(err)
		}
		if format == "png" {
			_, err = png.Decode(f)
		} else {
			_, err = gif.DecodeAll(f)
		}
		f.Close()
		if err != nil {
			t.Fatal(err)
		}
	}
	prior, _ := os.ReadFile(file)
	if err := Run([]string{"race", "--gap", "0", "--separation", "2", "--out", file}, &out, &errout); err == nil {
		t.Fatal("exported overlapping cars")
	}
	after, _ := os.ReadFile(file)
	if !bytes.Equal(prior, after) {
		t.Fatal("failed export damaged existing file")
	}
	for _, args := range [][]string{{"--format", "bad"}, {"--view", "perspective"}, {"--fps", "NaN"}, {"--gap", "NaN"}, {"--scenario", "missing"}, {"--format", "png"}} {
		if err := Run(append([]string{"race"}, args...), &out, &errout); err == nil {
			t.Fatalf("accepted %v", args)
		}
	}
}

func TestRaceValidationPreservesDestinations(t *testing.T) {
	for _, test := range []struct {
		name               string
		args               []string
		badSave, badOutput bool
	}{
		{name: "animation start", args: []string{"--format", "gif", "--time", "1000"}},
		{name: "animation budget", args: []string{"--format", "gif", "--width", "3840", "--height", "2160", "--fps", "50"}},
		{name: "missing save parent", badSave: true},
		{name: "missing output parent", badOutput: true},
	} {
		t.Run(test.name, func(t *testing.T) {
			dir := t.TempDir()
			save, out := filepath.Join(dir, "experiment.json"), filepath.Join(dir, "output")
			for _, path := range []string{save, out} {
				if err := os.WriteFile(path, []byte("keep existing content"), 0600); err != nil {
					t.Fatal(err)
				}
			}
			saveArg, outArg := save, out
			if test.badSave {
				saveArg = filepath.Join(dir, "missing", "experiment.json")
			}
			if test.badOutput {
				outArg = filepath.Join(dir, "missing", "output")
			}
			args := append([]string{"race", "--save-experiment", saveArg, "--out", outArg}, test.args...)
			if err := Run(args, io.Discard, io.Discard); err == nil {
				t.Fatal("accepted invalid export")
			}
			for _, path := range []string{save, out} {
				content, err := os.ReadFile(path)
				if err != nil || string(content) != "keep existing content" {
					t.Fatalf("modified %s on failure: %q, %v", path, content, err)
				}
			}
		})
	}
}

func TestRaceRejectsFileAliases(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "inputs.json")
	if err := os.WriteFile(path, []byte("keep inputs"), 0600); err != nil {
		t.Fatal(err)
	}
	hardlink := filepath.Join(dir, "hardlink.json")
	if err := os.Link(path, hardlink); err != nil {
		t.Fatal(err)
	}
	symlink := filepath.Join(dir, "symlink.json")
	if err := os.Symlink(path, symlink); err != nil {
		t.Fatal(err)
	}
	dirlink := filepath.Join(t.TempDir(), "linked-dir")
	if err := os.Symlink(dir, dirlink); err != nil {
		t.Fatal(err)
	}
	for _, alias := range []string{path, dir + "/./inputs.json", hardlink, symlink, filepath.Join(dirlink, "inputs.json")} {
		for _, inputFlag := range []string{"--save-experiment", "--experiment", "--scene"} {
			err := Run([]string{"race", "--out", alias, inputFlag, path}, io.Discard, io.Discard)
			if err == nil || !strings.Contains(err.Error(), "refer to the same file") {
				t.Fatalf("did not reject %s alias %s: %v", inputFlag, alias, err)
			}
		}
	}
	err := Run([]string{"race", "--out", filepath.Join(dirlink, "new.json"), "--save-experiment", filepath.Join(dir, "new.json")}, io.Discard, io.Discard)
	if err == nil || !strings.Contains(err.Error(), "refer to the same file") {
		t.Fatalf("did not reject alias of absent destination: %v", err)
	}
	err = Run([]string{"race", "--save-experiment", path, "--scene", path}, io.Discard, io.Discard)
	if err == nil || !strings.Contains(err.Error(), "refer to the same file") {
		t.Fatalf("did not protect scene from experiment save: %v", err)
	}
	content, err := os.ReadFile(path)
	if err != nil || string(content) != "keep inputs" {
		t.Fatalf("damaged input: %q, %v", content, err)
	}
}

type failedRaceWriter struct{}

func (failedRaceWriter) Write([]byte) (int, error) { return 0, errors.New("export write failed") }

func TestRaceFailedExportPreservesSavedInputs(t *testing.T) {
	path := filepath.Join(t.TempDir(), "experiment.json")
	if err := os.WriteFile(path, []byte("keep inputs"), 0600); err != nil {
		t.Fatal(err)
	}
	err := Run([]string{"race", "--save-experiment", path}, failedRaceWriter{}, io.Discard)
	if err == nil || !strings.Contains(err.Error(), "export write failed") {
		t.Fatalf("missing export error: %v", err)
	}
	content, err := os.ReadFile(path)
	if err != nil || string(content) != "keep inputs" {
		t.Fatalf("saved inputs despite export failure: %q, %v", content, err)
	}
}

func TestRaceRejectsNewCaseInsensitiveAlias(t *testing.T) {
	dir := t.TempDir()
	probe := filepath.Join(dir, "probe")
	if err := os.WriteFile(probe, nil, 0600); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(dir, "PROBE")); os.IsNotExist(err) {
		t.Skip("case-sensitive filesystem")
	} else if err != nil {
		t.Fatal(err)
	}
	out := filepath.Join(dir, "new.json")
	err := Run([]string{"race", "--out", out, "--save-experiment", filepath.Join(dir, "NEW.JSON")}, io.Discard, io.Discard)
	if err == nil || !strings.Contains(err.Error(), "refer to the same file") {
		t.Fatalf("silently overwrote export through case alias: %v", err)
	}
	data, err := os.ReadFile(out)
	if err != nil {
		t.Fatal(err)
	}
	var result racecraft.Result
	if err := json.Unmarshal(data, &result); err != nil || result.Duration <= 0 {
		t.Fatalf("export replaced with saved inputs: %v", err)
	}
}

func TestRaceInputOverridesAndStartSpeedSummary(t *testing.T) {
	dir := t.TempDir()
	experiment := filepath.Join(dir, "experiment.json")
	scene := filepath.Join(dir, "scene.json")
	e, err := racecraft.Example("defend")
	if err != nil {
		t.Fatal(err)
	}
	if err := racecraft.Save(experiment, e); err != nil {
		t.Fatal(err)
	}
	e.Scene.Name = "Custom race road"
	if err := track.Save(scene, e.Scene); err != nil {
		t.Fatal(err)
	}
	wantScene, err := track.Load(scene)
	if err != nil {
		t.Fatal(err)
	}
	var out, summary bytes.Buffer
	if err := Run([]string{"race", "--experiment", experiment, "--scene", scene, "--vehicle", "gt", "--save-experiment", experiment}, &out, &summary); err != nil {
		t.Fatal(err)
	}
	saved, err := racecraft.Load(experiment)
	if err != nil {
		t.Fatal(err)
	}
	gt, err := vehicle.Preset("gt")
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(saved.Scene, wantScene) || !reflect.DeepEqual(saved.Vehicle, gt) || saved.Config != e.Config {
		t.Fatal("scene/vehicle override or save-back lost effective inputs")
	}
	var result racecraft.Result
	if err := json.Unmarshal(out.Bytes(), &result); err != nil {
		t.Fatal(err)
	}
	start := result.At(0)
	want := fmt.Sprintf("realized start speeds A %.3f, B %.3f m/s; requested B-A station-zero cap delta %.3f m/s", start[0].Speed, start[1].Speed, e.Config.Overspeed)
	if !strings.Contains(summary.String(), want) {
		t.Fatalf("summary omits realized start speeds: %s", &summary)
	}
	prior, err := os.ReadFile(experiment)
	if err != nil {
		t.Fatal(err)
	}
	err = Run([]string{"race", "--experiment", experiment, "--scenario", "defend", "--save-experiment", experiment}, io.Discard, io.Discard)
	if err == nil || !strings.Contains(err.Error(), "choose --experiment or --scenario") {
		t.Fatalf("accepted conflicting experiment/scenario: %v", err)
	}
	after, err := os.ReadFile(experiment)
	if err != nil || !bytes.Equal(prior, after) {
		t.Fatal("conflicting flags changed saved experiment")
	}
}
