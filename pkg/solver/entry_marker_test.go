package solver

import "testing"

func TestImmediateCornerMarksEntryBraking(t *testing.T) {
	nodes := make([]Node, 201)
	for i := range nodes {
		s := float64(i) / 4
		speed := 20 + (s-25)/5
		phase := "drive"
		if s < 25 {
			speed = 20 + (25-s)/5
			phase = "brake"
		}
		nodes[i] = Node{Station: s, S: s, Time: s / 20, Speed: speed, Curvature: .02}
		nodes[i].Forces.Phase = phase
	}
	markers := DetectMarkers(nodes)
	if len(markers) < 3 || markers[0].Kind != "brake" || markers[0].Station != 0 {
		t.Fatalf("entry braking omitted: %+v", markers)
	}
}
