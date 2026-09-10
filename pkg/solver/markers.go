package solver

import (
	"math"
	"sort"

	"github.com/TheFellow/the-line/pkg/track"
)

// Marker identifies a model-derived event at a common road station. "drive"
// means sustained positive tyre force, not measured accelerator application.
type Marker struct {
	Kind     string     `json:"kind"`
	Station  float64    `json:"station"`
	Time     float64    `json:"time"`
	Position track.Vec3 `json:"position"`
}

// DetectMarkers works on a fixed spatial grid so a different mesh does not
// change its thresholds. Apexes are prominent local speed minima in corners;
// a 2 m average suppresses sub-metre profile noise. Brake/drive runs must last
// at least 2 m. Straights have no corner markers, including endpoint stops.
func DetectMarkers(nodes []Node) []Marker {
	if len(nodes) < 3 {
		return nil
	}
	const step = .25
	first, last := nodes[0].Station, nodes[len(nodes)-1].Station
	count := int((last-first)/step) + 1
	if count < 33 || count > 200000 {
		return nil
	}
	samples := make([]Node, count)
	smooth := make([]float64, count)
	for i := range samples {
		samples[i], _ = atStation(nodes, first+float64(i)*step)
	}
	for i := range samples {
		weight := 0.
		for j := max(0, i-4); j <= min(count-1, i+4); j++ {
			w := 5 - math.Abs(float64(i-j))
			smooth[i] += samples[j].Speed * w
			weight += w
		}
		smooth[i] /= weight
	}
	var apexes []int
	for i := 8; i < count-8; i++ {
		if math.Abs(samples[i].Curvature) < .001 || smooth[i] > smooth[i-1] || smooth[i] >= smooth[i+1] {
			continue
		}
		left, right := smooth[i], smooth[i]
		for j := max(0, i-80); j < i; j++ {
			left = math.Max(left, smooth[j])
		}
		for j := i + 1; j <= min(count-1, i+80); j++ {
			right = math.Max(right, smooth[j])
		}
		if math.Min(left, right)-smooth[i] < .35 {
			continue
		}
		if len(apexes) > 0 && i-apexes[len(apexes)-1] < 40 {
			if smooth[i] < smooth[apexes[len(apexes)-1]] {
				apexes[len(apexes)-1] = i
			}
			continue
		}
		apexes = append(apexes, i)
	}
	var out []Marker
	add := func(kind string, i int) {
		n := samples[i]
		out = append(out, Marker{Kind: kind, Station: n.Station, Time: n.Time, Position: n.Position})
	}
	sustained := func(i int, phase string) bool {
		if i+8 >= count {
			return false
		}
		for j := i; j <= i+8; j++ {
			if samples[j].Forces.Phase != phase {
				return false
			}
		}
		return true
	}
	previous := 0
	for a, apex := range apexes {
		add("apex", apex)
		drove, brake := false, -1
		for i := previous; i < apex; i++ {
			if sustained(i, "drive") {
				drove = true
			}
			if drove && sustained(i, "brake") {
				brake = i
				drove = false
			}
		}
		if brake >= 0 {
			add("brake", brake)
		}
		end := count - 1
		if a+1 < len(apexes) {
			end = apexes[a+1]
		}
		for i := apex; i < end; i++ {
			if sustained(i, "drive") {
				add("drive", i)
				break
			}
		}
		previous = apex
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Station < out[j].Station })
	return out
}
