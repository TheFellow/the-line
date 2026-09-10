package solver

import (
	"fmt"
	"math"

	"github.com/TheFellow/the-line/internal/periodic"
	"github.com/TheFellow/the-line/pkg/track"
)

func (e evaluator) closed() bool { return len(e.road) > 0 && e.road[0].Closed }

func periodicOffsets(coarse []track.Sample, offsets []float64, fine []track.Sample, clearance float64) []float64 {
	n := len(coarse) - 1
	latent, spans := make([]float64, n), make([]float64, n)
	for i := range latent {
		center := (coarse[i].LeftLimit() - coarse[i].RightLimit()) / 2
		bound := (coarse[i].LeftLimit()+coarse[i].RightLimit())/2 - clearance
		latent[i] = math.Atanh(clamp((offsets[i]-center)/bound, -.999999, .999999))
		spans[i] = coarse[i+1].S - coarse[i].S
	}
	second := periodic.Second(latent, spans)
	result := make([]float64, len(fine))
	j := 0
	for i, p := range fine {
		station := p.S * coarse[n].S / fine[len(fine)-1].S
		for j < n-1 && coarse[j+1].S < station {
			j++
		}
		b := clamp((station-coarse[j].S)/spans[j], 0, 1)
		a := 1 - b
		next := (j + 1) % n
		value := a*latent[j] + b*latent[next] + ((a*a*a-a)*second[j]+(b*b*b-b)*second[next])*spans[j]*spans[j]/6
		result[i] = (p.LeftLimit()-p.RightLimit())/2 + ((p.LeftLimit()+p.RightLimit())/2-clearance)*math.Tanh(value)
	}
	result[len(result)-1] = result[0]
	return result
}

func validatePeriodicOffsets(road []track.Sample, offsets []float64) error {
	if len(road) > 0 && road[0].Closed && offsets[0] != offsets[len(offsets)-1] {
		return fmt.Errorf("closed line: first and final offsets must match at the lap seam")
	}
	return nil
}

// LapAt samples a continuous steady-state lap. At and AtStation retain their
// clamped single-lap semantics for charts and exports. Negative times wrap too.
func (r Result) LapAt(t float64) Node {
	if r.Closed && finite(t) && r.Duration > 0 {
		t = math.Mod(t, r.Duration)
		if t < 0 {
			t += r.Duration
		}
	}
	return r.At(t)
}

// CenterLapAt independently wraps the centreline reference's own lap period.
func (r Result) CenterLapAt(t float64) Node {
	if r.Closed && finite(t) && r.CenterDuration > 0 {
		t = math.Mod(t, r.CenterDuration)
		if t < 0 {
			t += r.CenterDuration
		}
	}
	return r.CenterAt(t)
}
