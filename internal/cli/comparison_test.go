package cli

import (
	"bytes"
	"encoding/csv"
	"github.com/TheFellow/the-line/pkg/solver"
	"github.com/TheFellow/the-line/pkg/track"
	"math"
	"strconv"
	"testing"
)

func TestComparisonCSVAlignsRoadStationNotDistance(t *testing.T) {
	r := solver.Result{Nodes: []solver.Node{{Station: 0, S: 0, Speed: 10}, {Station: 10, S: 20, Speed: 10, Time: 2}, {Station: 20, S: 40, Speed: 10, Time: 4}}, CenterNodes: []solver.Node{{Station: 0, S: 0, Speed: 5}, {Station: 20, S: 20, Speed: 5, Time: 4}}, Duration: 4, CenterDuration: 4}
	var b bytes.Buffer
	if err := writeComparisonCSV(&b, track.Scene{}, r); err != nil {
		t.Fatal(err)
	}
	rows, err := csv.NewReader(&b).ReadAll()
	if err != nil {
		t.Fatal(err)
	}
	index := map[string]int{}
	for i, k := range rows[0] {
		index[k] = i
	}
	for field, want := range map[string]float64{"station_m": 10, "current_path_m": 20, "reference_path_m": 10, "current_time_s": 2, "reference_time_s": 2, "delta_s": 0} {
		v, err := strconv.ParseFloat(rows[2][index[field]], 64)
		if err != nil || math.Abs(v-want) > 1e-12 {
			t.Fatalf("%s = %g want %g (%v)", field, v, want, err)
		}
	}
	if rows[2][index["current_utilization"]] != "" {
		t.Fatal("unavailable force telemetry must remain empty")
	}
}
