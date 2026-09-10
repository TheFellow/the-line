package cli

import (
	"bytes"
	"encoding/csv"
	"encoding/json"
	"github.com/TheFellow/the-line/pkg/track"
	"github.com/TheFellow/the-line/pkg/vehicle"
	"path/filepath"
	"strconv"
	"testing"
)

func TestSweepFixedLineAndValidation(t *testing.T) {
	var stdout, stderr bytes.Buffer
	err := Run([]string{"sweep", "--preset", "hairpin", "--parameter", "power", "--from", "100000", "--to", "150000", "--steps", "3", "--iterations", "0"}, &stdout, &stderr)
	if err != nil {
		t.Fatal(err)
	}
	rows, err := csv.NewReader(&stdout).ReadAll()
	if err != nil {
		t.Fatal(err)
	}
	if len(rows) != 4 || rows[0][1] != "value_si" {
		t.Fatalf("bad CSV: %v", rows)
	}
	first, _ := strconv.ParseFloat(rows[1][2], 64)
	last, _ := strconv.ParseFloat(rows[3][2], 64)
	if last > first {
		t.Fatalf("extra power slowed same line: %v", rows)
	}
	for _, args := range [][]string{{"sweep", "--steps", "1"}, {"sweep", "--parameter", "bogus"}, {"sweep", "--parameter", "mass", "--from", "-1"}} {
		if err := Run(args, &stdout, &stderr); err == nil {
			t.Fatalf("accepted %v", args)
		}
	}
}

func TestCLIInlineSetupAndExplicitOverride(t *testing.T) {
	scene, _ := track.Preset("hairpin")
	car, _ := vehicle.Preset("gt")
	car.Name = "Saved custom car"
	scene.VehicleConfig = &car
	path := filepath.Join(t.TempDir(), "study.json")
	if err := track.Save(path, scene); err != nil {
		t.Fatal(err)
	}
	var stdout, stderr bytes.Buffer
	if err := Run([]string{"solve", "--scene", path, "--iterations", "0"}, &stdout, &stderr); err != nil {
		t.Fatal(err)
	}
	var decoded struct {
		Scene   track.Scene
		Vehicle vehicle.Config
	}
	if err := json.Unmarshal(stdout.Bytes(), &decoded); err != nil {
		t.Fatal(err)
	}
	if decoded.Vehicle != car {
		t.Fatal("CLI ignored embedded setup")
	}
	stdout.Reset()
	if err := Run([]string{"solve", "--scene", path, "--vehicle", "rally", "--iterations", "0"}, &stdout, &stderr); err != nil {
		t.Fatal(err)
	}
	decoded.Scene = track.Scene{}
	if err := json.Unmarshal(stdout.Bytes(), &decoded); err != nil {
		t.Fatal(err)
	}
	want, _ := vehicle.Preset("rally")
	if decoded.Vehicle != want || decoded.Scene.VehicleConfig != nil {
		t.Fatal("preset override left stale embedded car")
	}
}
