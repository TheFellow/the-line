package cli

import (
	"encoding/csv"
	"fmt"
	"io"
	"strconv"

	"github.com/TheFellow/the-line/pkg/render"
	"github.com/TheFellow/the-line/pkg/solver"
	"github.com/TheFellow/the-line/pkg/track"
)

// writeComparisonCSV aligns both profiles at the current trajectory's road
// stations; path distance and time remain independent for each car.
func writeComparisonCSV(w io.Writer, scene track.Scene, current solver.Result) error {
	reference := current.CenterTrajectory()
	if scene.Study != nil && scene.Study.Reference != nil {
		ref, err := render.RestoreReference(scene.Study.Reference)
		if err != nil {
			return err
		}
		if !ref.Compatible(scene) {
			return fmt.Errorf("comparison reference stale: different road")
		}
		reference = ref.Trajectory
	}
	if len(reference.Nodes) < 2 {
		return fmt.Errorf("comparison requires a reference trajectory")
	}
	c := csv.NewWriter(w)
	fields := []string{"station_m"}
	for _, prefix := range []string{"current_", "reference_"} {
		for _, name := range []string{"time_s", "path_m", "speed_mps", "x_m", "y_m", "z_m", "offset_m", "lateral_mps2", "longitudinal_mps2", "capacity_mps2", "utilization", "phase", "limit"} {
			fields = append(fields, prefix+name)
		}
	}
	fields = append(fields, "delta_s")
	if err := c.Write(fields); err != nil {
		return err
	}
	number := func(v float64) string { return strconv.FormatFloat(v, 'g', 17, 64) }
	for _, node := range current.Nodes {
		a, err := current.AtStation(node.Station)
		if err != nil {
			return err
		}
		b, err := reference.AtStation(node.Station)
		if err != nil {
			return err
		}
		row := []string{number(node.Station)}
		for _, n := range []solver.Node{a, b} {
			for _, v := range []float64{n.Time, n.S, n.Speed, n.Position.X, n.Position.Y, n.Position.Z, n.Offset} {
				row = append(row, number(v))
			}
			if n.Forces.Available {
				for _, v := range []float64{n.Forces.Lateral, n.Forces.Longitudinal, n.Forces.Capacity, n.Forces.Utilization} {
					row = append(row, number(v))
				}
				row = append(row, n.Forces.Phase, n.Forces.Limit)
			} else {
				row = append(row, "", "", "", "", "", "")
			}
		}
		row = append(row, number(a.Time-b.Time))
		if err := c.Write(row); err != nil {
			return err
		}
	}
	c.Flush()
	return c.Error()
}
