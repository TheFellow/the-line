package cli

import (
	"bytes"
	"encoding/csv"
	"encoding/json"
	"image/png"
	"io"
	"math"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"

	"github.com/TheFellow/the-line/pkg/solver"
	"github.com/TheFellow/the-line/pkg/track"
	"github.com/TheFellow/the-line/pkg/vehicle"
)

func savedStraightStudy(t *testing.T) (track.Scene, string) {
	t.Helper()
	scene := track.Scene{Version: track.Version, Name: "CLI study", Vehicle: "road", EntrySpeed: 10, ExitSpeed: 10,
		Points: []track.Point{{Width: 12, Surface: "asphalt"}, {X: 80, Width: 12, Surface: "asphalt"}, {X: 160, Width: 12, Surface: "asphalt"}}}
	road, err := track.SampleRoad(scene, 3)
	if err != nil {
		t.Fatal(err)
	}
	offsets := make([]float64, len(road))
	for i := range offsets {
		offsets[i] = 1
	}
	scene.Study = &track.Study{Version: 1, Manual: &track.ManualLine{Spacing: 3, RoadDigest: track.RoadDigest(scene), Offsets: offsets}}
	path := filepath.Join(t.TempDir(), "study.json")
	if err := track.Save(path, scene); err != nil {
		t.Fatal(err)
	}
	return scene, path
}

func TestSavedManualLineCLISelection(t *testing.T) {
	_, path := savedStraightStudy(t)
	for _, selection := range []string{"", "auto", "manual", "optimized"} {
		t.Run(selection, func(t *testing.T) {
			args := []string{"solve", "--scene", path, "--iterations", "1"}
			if selection != "" {
				args = append(args, "--line", selection)
			}
			var out bytes.Buffer
			if err := Run(args, &out, io.Discard); err != nil {
				t.Fatal(err)
			}
			var decoded struct {
				Method string
				Result solver.Result
			}
			if err := json.Unmarshal(out.Bytes(), &decoded); err != nil {
				t.Fatal(err)
			}
			wantOffset := 1.0
			if selection == "optimized" {
				wantOffset = 0
				if strings.Contains(decoded.Method, "manual") {
					t.Fatal("optimized export is mislabeled as an authored line")
				}
			} else if !strings.Contains(decoded.Method, "saved manual line") {
				t.Fatalf("manual export method: %s", decoded.Method)
			}
			if len(decoded.Result.Nodes) < 2 {
				t.Fatal("missing exported trajectory")
			}
			for _, node := range decoded.Result.Nodes {
				if math.Abs(node.Offset-wantOffset) > 1e-9 {
					t.Fatalf("selection %q offset %g, want %g", selection, node.Offset, wantOffset)
				}
			}
		})
	}
}

func TestManualCLIRejectsMissingAndStaleLines(t *testing.T) {
	scene, path := savedStraightStudy(t)
	scene.Points[1].Z = 1
	if err := track.Save(path, scene); err != nil {
		t.Fatal(err)
	}
	for _, selection := range []string{"auto", "manual"} {
		err := Run([]string{"solve", "--scene", path, "--line", selection}, io.Discard, io.Discard)
		if err == nil || !strings.Contains(err.Error(), "manual line stale") {
			t.Fatalf("stale %s line error = %v", selection, err)
		}
	}
	if err := Run([]string{"solve", "--scene", path, "--line", "optimized", "--iterations", "1"}, io.Discard, io.Discard); err != nil {
		t.Fatalf("explicit optimization should recover from stale manual geometry: %v", err)
	}
	scene.Study = nil
	if err := track.Save(path, scene); err != nil {
		t.Fatal(err)
	}
	err := Run([]string{"solve", "--scene", path, "--line", "manual"}, io.Discard, io.Discard)
	if err == nil || !strings.Contains(err.Error(), "no saved manual line") {
		t.Fatalf("missing manual line error = %v", err)
	}
}

func TestComparisonCSVCLIUsesPinnedVehicleAndPreservesStaleOutput(t *testing.T) {
	scene, path := savedStraightStudy(t)
	referenceScene := scene
	referenceScene.Study = nil
	referenceScene.Points = append([]track.Point(nil), scene.Points...)
	referenceCar, err := vehicle.Preset("road")
	if err != nil {
		t.Fatal(err)
	}
	referenceCar.Power *= .5
	line := *scene.Study.Manual
	line.Offsets = make([]float64, len(line.Offsets))
	scene.Study.Reference = &track.PinnedLine{Name: "Half power", Scene: &referenceScene, Vehicle: referenceCar, Line: line}
	if err := track.Save(path, scene); err != nil {
		t.Fatal(err)
	}
	var out bytes.Buffer
	if err := Run([]string{"solve", "--scene", path, "--format", "comparison-csv"}, &out, io.Discard); err != nil {
		t.Fatal(err)
	}
	rows, err := csv.NewReader(&out).ReadAll()
	if err != nil {
		t.Fatal(err)
	}
	columns := map[string]int{}
	for i, name := range rows[0] {
		columns[name] = i
	}
	last := rows[len(rows)-1]
	number := func(name string) float64 {
		t.Helper()
		i, ok := columns[name]
		if !ok {
			t.Fatalf("missing column %s", name)
		}
		v, err := strconv.ParseFloat(last[i], 64)
		if err != nil {
			t.Fatalf("%s: %v", name, err)
		}
		return v
	}
	if got := number("station_m"); math.Abs(got-160) > 1e-9 {
		t.Fatalf("comparison ends at station %g, want 160", got)
	}
	current, reference := number("current_time_s"), number("reference_time_s")
	if current >= reference || math.Abs(number("delta_s")-(current-reference)) > 1e-12 {
		t.Fatalf("pinned half-power car should be slower: current %g reference %g", current, reference)
	}
	for _, name := range []string{"current_utilization", "reference_utilization"} {
		if v := number(name); v < 0 || v > 1.001 {
			t.Fatalf("invalid %s: %g", name, v)
		}
	}
	scene.Points[1].Z = 1
	if err := track.Save(path, scene); err != nil {
		t.Fatal(err)
	}
	outputPath := filepath.Join(t.TempDir(), "comparison.csv")
	if err := os.WriteFile(outputPath, []byte("previous export"), 0600); err != nil {
		t.Fatal(err)
	}
	err = Run([]string{"solve", "--scene", path, "--line", "optimized", "--iterations", "1", "--format", "comparison-csv", "--out", outputPath}, io.Discard, io.Discard)
	if err == nil || !strings.Contains(err.Error(), "comparison reference stale") {
		t.Fatalf("stale comparison error = %v", err)
	}
	previous, err := os.ReadFile(outputPath)
	if err != nil || string(previous) != "previous export" {
		t.Fatalf("failed comparison overwrote prior export: %q, %v", previous, err)
	}
}

func TestPerspectiveRenderCLIUsesSavedManualStudy(t *testing.T) {
	_, path := savedStraightStudy(t)
	outputPath := filepath.Join(t.TempDir(), "perspective.png")
	if err := Run([]string{"render", "--scene", path, "--view", "perspective", "--width", "800", "--height", "600", "--out", outputPath}, io.Discard, io.Discard); err != nil {
		t.Fatal(err)
	}
	f, err := os.Open(outputPath)
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	img, err := png.Decode(f)
	if err != nil {
		t.Fatal(err)
	}
	if img.Bounds().Dx() != 800 || img.Bounds().Dy() != 600 {
		t.Fatalf("unexpected perspective image bounds: %v", img.Bounds())
	}
}

func TestClosedLapCLIExportsPeriodicMetadata(t *testing.T) {
	var out bytes.Buffer
	if err := Run([]string{"solve", "--preset", "club-loop", "--iterations", "1"}, &out, io.Discard); err != nil {
		t.Fatal(err)
	}
	var decoded struct {
		Scene  track.Scene
		Result solver.Result
		Method string
	}
	if err := json.Unmarshal(out.Bytes(), &decoded); err != nil {
		t.Fatal(err)
	}
	if !decoded.Scene.Closed || !decoded.Result.Closed || !strings.Contains(decoded.Method, "steady-state lap") || strings.Contains(decoded.Method, "open sequence") {
		t.Fatalf("closed metadata: scene %t result %t method %q", decoded.Scene.Closed, decoded.Result.Closed, decoded.Method)
	}
	nodes := decoded.Result.Nodes
	if len(nodes) < 3 {
		t.Fatal("missing lap trajectory")
	}
	first, last := nodes[0], nodes[len(nodes)-1]
	if first.Position.Sub(last.Position).Length() > 1e-9 || math.Abs(first.Speed-last.Speed) > 1e-9 || last.Time != decoded.Result.Duration {
		t.Fatal("exported lap has a discontinuous seam or incorrect elapsed duration")
	}
}

func TestEveryCommandHelpReturnsSuccess(t *testing.T) {
	for _, command := range []string{"new", "import", "validate", "solve", "sweep", "render", "animate"} {
		t.Run(command, func(t *testing.T) {
			var help bytes.Buffer
			if err := Run([]string{command, "--help"}, io.Discard, &help); err != nil {
				t.Fatalf("help failed: %v", err)
			}
			if help.Len() == 0 {
				t.Fatal("missing command usage")
			}
		})
	}
}
