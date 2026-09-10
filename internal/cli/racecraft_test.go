package cli

import (
	"bytes"
	"encoding/csv"
	"encoding/json"
	"github.com/TheFellow/the-line/pkg/racecraft"
	"image/gif"
	"image/png"
	"os"
	"path/filepath"
	"testing"
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
