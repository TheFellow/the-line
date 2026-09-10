package cli

import (
	"bytes"
	"encoding/csv"
	"strconv"
	"testing"

	"github.com/TheFellow/the-line/pkg/solver"
	"github.com/TheFellow/the-line/pkg/vehicle"
)

func TestForceCSVPreservesPrecisionAndExistingColumns(t *testing.T) {
	n := solver.Node{Station: 123.456789012345, Forces: vehicle.TyreForces{TyreEnvelope: vehicle.TyreEnvelope{Available: true, Lateral: 4.123456789123, Capacity: 9.80665}, Longitudinal: -3.123456789123, Utilization: .527481925, Phase: "brake", Limit: "grip"}}
	var out bytes.Buffer
	if err := writeCSV(&out, solver.Result{Nodes: []solver.Node{n}}); err != nil {
		t.Fatal(err)
	}
	rows, err := csv.NewReader(&out).ReadAll()
	if err != nil {
		t.Fatal(err)
	}
	if rows[0][9] != "station_m" || rows[0][13] != "tyre_lateral_mps2" || rows[0][16] != "tyre_utilization" {
		t.Fatal("existing or appended CSV column meaning changed")
	}
	for i, want := range map[int]float64{9: n.Station, 13: n.Forces.Lateral, 14: n.Forces.Longitudinal, 15: n.Forces.Capacity, 16: n.Forces.Utilization} {
		got, err := strconv.ParseFloat(rows[1][i], 64)
		if err != nil || got != want {
			t.Fatalf("CSV lost force precision at %s", rows[0][i])
		}
	}
	if rows[1][17] != "brake" || rows[1][18] != "grip" || rows[1][19] != "true" {
		t.Fatal("CSV omits phase, limit or availability")
	}
}
