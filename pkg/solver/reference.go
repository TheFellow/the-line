package solver

// CenterTrajectory presents the retained centreline as a trajectory while
// retaining its live force model for exact interpolated telemetry queries.
// Nodes and Road are shared read-only, as with other Result query methods.
func (r Result) CenterTrajectory() Result {
	center := Result{Nodes: r.CenterNodes, Road: r.Road, Duration: r.CenterDuration,
		Spacing: r.Spacing, Closed: r.Closed, EntrySpeedCap: r.EntrySpeedCap,
		ExitSpeedCap: r.ExitSpeedCap, model: r.model}
	if len(center.Nodes) > 0 {
		center.Length = center.Nodes[len(center.Nodes)-1].S
	}
	return center
}
