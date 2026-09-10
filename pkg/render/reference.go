package render

import (
	"fmt"

	"github.com/TheFellow/the-line/pkg/solver"
	"github.com/TheFellow/the-line/pkg/track"
	"github.com/TheFellow/the-line/pkg/vehicle"
)

// Reference is an immutable presentation snapshot. A physical-road digest
// guards station comparisons; changing a car never changes the pinned profile.
type Reference struct {
	Name       string
	Trajectory solver.Result
	Vehicle    vehicle.Config
	RoadDigest string
}

func PinReference(name string, scene track.Scene, result solver.Result, car vehicle.Config) Reference {
	result.Nodes = append([]solver.Node(nil), result.Nodes...)
	result.Road = append([]track.Sample(nil), result.Road...)
	result.Offsets = append([]float64(nil), result.Offsets...)
	result.CenterNodes = nil
	return Reference{Name: name, Trajectory: result, Vehicle: car, RoadDigest: track.RoadDigest(scene)}
}

func (r Reference) Compatible(scene track.Scene) bool {
	return r.RoadDigest == track.RoadDigest(scene) && len(r.Trajectory.Nodes) > 1
}
func (r *Renderer) referenceCompatible() bool {
	return r.opts.Reference == nil || r.opts.Reference.Compatible(r.scene)
}
func (r *Renderer) referenceTrajectory() solver.Result {
	if !r.referenceCompatible() {
		return solver.Result{}
	}
	if r.opts.Reference != nil {
		return r.opts.Reference.Trajectory
	}
	return solver.Result{Nodes: r.result.CenterNodes, Duration: r.result.CenterDuration}
}
func (r *Renderer) referenceName() string {
	if r.opts.Reference != nil {
		return r.opts.Reference.Name
	}
	return "Centreline"
}
func (r *Renderer) referenceNodes() []solver.Node { return r.referenceTrajectory().Nodes }
func (r *Renderer) referenceAt(t float64) solver.Node {
	if r.opts.Reference == nil {
		return r.result.CenterAt(t)
	}
	return r.referenceTrajectory().At(t)
}
func (r *Renderer) referenceAtStation(station float64) (solver.Node, error) {
	if !r.referenceCompatible() {
		return solver.Node{}, fmt.Errorf("reference stale: different road")
	}
	if r.opts.Reference == nil {
		return r.result.CenterAtStation(station)
	}
	return r.referenceTrajectory().AtStation(station)
}
func (r *Renderer) referenceDuration() float64 { return r.referenceTrajectory().Duration }

// RestoreReference reevaluates the exact saved line under its original car and
// original caps. The caller keeps it stale when the current physical road differs.
func RestoreReference(saved *track.PinnedLine) (*Reference, error) {
	if saved == nil {
		return nil, nil
	}
	if err := (track.Study{Version: 1, Reference: saved}).Validate(); err != nil {
		return nil, err
	}
	opts := solver.DefaultOptions()
	opts.Spacing = saved.Line.Spacing
	result, err := solver.Evaluate(*saved.Scene, saved.Vehicle, saved.Line.Offsets, opts)
	if err != nil {
		return nil, fmt.Errorf("restore reference %q: %w", saved.Name, err)
	}
	ref := PinReference(saved.Name, *saved.Scene, result, saved.Vehicle)
	return &ref, nil
}

func StoreReference(name string, scene track.Scene, result solver.Result, car vehicle.Config) *track.PinnedLine {
	scene.Study = nil
	scene.Points = append([]track.Point(nil), scene.Points...)
	return &track.PinnedLine{Name: name, Scene: &scene, Vehicle: car, Line: track.ManualLine{Spacing: result.Spacing, RoadDigest: track.RoadDigest(scene), Offsets: append([]float64(nil), result.Offsets...)}}
}
