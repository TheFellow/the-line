// Package racecraft plans deterministic two-car experiments on open roads.
// Tactical intentions are authored; speeds and outcomes are computed, and only
// continuously collision-certified pairs are returned.
package racecraft

import (
	"context"
	"fmt"
	"math"

	"github.com/TheFellow/the-line/pkg/solver"
	"github.com/TheFellow/the-line/pkg/track"
	"github.com/TheFellow/the-line/pkg/vehicle"
)

const BodyLength = 4.4

type Config struct {
	Version    int     `json:"version"`
	Scenario   string  `json:"scenario"`
	Gap        float64 `json:"gap_m"`
	Overspeed  float64 `json:"overspeed_mps"`
	Separation float64 `json:"separation_m"`
	Clearance  float64 `json:"clearance_m"`
}

type Car struct {
	Name      string        `json:"name"`
	Intent    string        `json:"intent"`
	Path      solver.Result `json:"path"`
	StartTime float64       `json:"start_time"`
	Radius    float64       `json:"radius_m"`
}

type Event struct {
	Time   float64 `json:"time"`
	Leader int     `json:"leader"`
	Kind   string  `json:"kind"`
}

type Result struct {
	Config       Config         `json:"config"`
	Scene        track.Scene    `json:"scene"`
	Vehicle      vehicle.Config `json:"vehicle"`
	Cars         [2]Car         `json:"cars"`
	Duration     float64        `json:"duration"`
	MinClearance float64        `json:"certified_body_clearance_m"`
	Events       []Event        `json:"events"`
	Candidates   int            `json:"candidates"`
}

func Scenarios() []string { return []string{"over-under", "pass-repass", "defend", "esses-duel"} }

func DefaultConfig(name string) Config {
	c := Config{Version: 1, Scenario: name, Gap: 6, Overspeed: 3, Separation: 6, Clearance: .2}
	if name == "pass-repass" {
		c.Gap = 5
		c.Overspeed = 8
	}
	if name == "defend" {
		c.Gap = 14
		c.Overspeed = 0
	}
	if name == "esses-duel" {
		c.Gap = 6
		c.Overspeed = 4
	}
	return c
}

func (c Config) Validate() error {
	if c.Version != 1 {
		return fmt.Errorf("racecraft version must be 1")
	}
	found := false
	for _, s := range Scenarios() {
		found = found || s == c.Scenario
	}
	if !found {
		return fmt.Errorf("unknown racecraft scenario %q", c.Scenario)
	}
	for _, x := range []struct {
		name      string
		v, lo, hi float64
	}{{"gap", c.Gap, 0, 50}, {"overspeed", c.Overspeed, -15, 20}, {"separation", c.Separation, 2, 7}, {"clearance", c.Clearance, 0, 2}} {
		if math.IsNaN(x.v) || math.IsInf(x.v, 0) || x.v < x.lo || x.v > x.hi {
			return fmt.Errorf("racecraft %s must be finite and in [%g, %g]", x.name, x.lo, x.hi)
		}
	}
	return nil
}

func Scene(name string) (track.Scene, error) {
	preset := "hairpin"
	if name == "esses-duel" {
		preset = "esses"
	}
	return track.Preset(preset)
}

// Plan keeps the requested starting gap and never substitutes an unsafe pair.
// Its finite candidate order is deterministic. It supports open custom scenes;
// the scenario's normalized placements are intentions, not inferred apexes.
func Plan(ctx context.Context, scene track.Scene, v vehicle.Config, c Config) (Result, error) {
	if err := ctx.Err(); err != nil {
		return Result{}, err
	}
	if err := c.Validate(); err != nil {
		return Result{}, err
	}
	if err := v.Validate(); err != nil {
		return Result{}, err
	}
	if scene.Closed {
		return Result{}, fmt.Errorf("racecraft requires an open corner sequence")
	}
	radius := math.Hypot(BodyLength, v.Width) / 2
	opts := solver.Options{Spacing: 2, Margin: radius - v.Width/2 + .25}
	road, err := track.SampleRoad(scene, opts.Spacing)
	if err != nil {
		return Result{}, err
	}
	if c.Gap >= road[len(road)-1].S {
		return Result{}, fmt.Errorf("starting gap exceeds the road length")
	}
	r := Result{Config: c, Scene: scene, Vehicle: v, Events: []Event{}}
	// Prefer the requested line, then try progressively wider space-giving lines.
	for _, variant := range []float64{0, .25, .5} {
		if err := ctx.Err(); err != nil {
			return Result{}, err
		}
		r.Candidates++
		valid := true
		for car := 0; car < 2; car++ {
			controls, intent := placements(c, car, variant, len(road))
			offsets, e := solver.ManualOffsets(road, controls, radius+.25)
			if e != nil {
				valid = false
				break
			}
			s := scene
			if car == 1 {
				s.EntrySpeed = math.Max(1, scene.EntrySpeed+c.Overspeed-variant*8)
				if variant > 0 {
					intent += " / give room"
				}
			}
			path, e := solver.EvaluateContext(ctx, s, v, offsets, opts)
			if e != nil {
				if ctx.Err() != nil {
					return Result{}, ctx.Err()
				}
				valid = false
				break
			}
			start := 0.
			if car == 0 {
				n, _ := path.AtStation(c.Gap)
				start = n.Time
			}
			r.Cars[car] = Car{Name: []string{"A", "B"}[car], Intent: intent, Path: path, StartTime: start, Radius: radius}
		}
		if !valid {
			continue
		}
		r.Duration = math.Min(r.Cars[0].Path.Duration-r.Cars[0].StartTime, r.Cars[1].Path.Duration)
		clearance, ok := certify(ctx, r)
		if ctx.Err() != nil {
			return Result{}, ctx.Err()
		}
		if !ok {
			continue
		}
		r.MinClearance = clearance
		r.Events = events(r)
		return r, nil
	}
	return Result{}, fmt.Errorf("no collision-free plan for these placements; increase gap or separation, or reduce overspeed/clearance")
}

func placements(c Config, car int, variant float64, count int) ([]solver.LineControl, string) {
	// Fractions describe approach, rotation and exit on the example road.
	f := []float64{0, .25, .4, .52, .64, .78, 1}
	a := c.Separation / 2
	var values []float64
	intent := ""
	switch c.Scenario {
	case "over-under":
		if car == 0 {
			f = []float64{0, .25, .4, .60, .70, .82, 1}
			values = []float64{a, a, a, a, -a, -a, -a}
			intent = "Defend inside / open exit"
		} else {
			f = []float64{0, .25, .38, .46, .72, .86, 1}
			values = []float64{-a, -a, -a, -a, a, a, a}
			intent = "Outside entry / cut back"
		}
	case "pass-repass":
		if car == 0 {
			f = []float64{0, .25, .38, .46, .72, .86, 1}
			values = []float64{-a, -a, -a, -a, a, a, a}
			intent = "Yield entry / recover exit"
		} else {
			f = []float64{0, .25, .4, .60, .70, .82, 1}
			values = []float64{a, a, a, a, -a, -a, -a}
			intent = "Attack inside / run wide"
		}
	case "defend":
		if car == 0 {
			values = []float64{a, a, a, a, a, a, a}
			intent = "Hold the inside"
		} else {
			values = []float64{-a, -a, -a, -a, -a, -a, -a}
			intent = "Try around the outside"
		}
	case "esses-duel":
		if car == 0 {
			values = []float64{a, a, a, a, a, a, a}
			intent = "Protect first apex"
		} else {
			values = []float64{-a, -a, -a, -a, -a, -a, -a}
			intent = "Trade position at next bend"
		}
	}
	// If the preferred crossing is occupied, delay B's lateral transition
	// as well as giving a little more room. The full speed profile is reevaluated.
	if car == 1 && (c.Scenario == "over-under" || c.Scenario == "pass-repass") {
		f[3] += variant * .08
		f[4] += variant * .08
	}
	out := make([]solver.LineControl, len(f))
	for i := range f {
		v := values[i]
		if car == 1 {
			v += math.Copysign(variant, v)
		}
		out[i] = solver.LineControl{Index: int(math.Round(f[i] * float64(count-1))), Offset: v}
	}
	return out, intent
}

func (r Result) At(t float64) [2]solver.Node {
	if math.IsNaN(t) {
		t = 0
	}
	t = math.Max(0, math.Min(t, r.Duration))
	return [2]solver.Node{r.Cars[0].Path.At(t + r.Cars[0].StartTime), r.Cars[1].Path.At(t + r.Cars[1].StartTime)}
}

func events(r Result) []Event {
	out := []Event{}
	leader := 0
	for t := 0.; t <= r.Duration; t = math.Min(t+.02, r.Duration) {
		n := r.At(t)
		gap := n[0].Station - n[1].Station
		next := leader
		if gap > BodyLength {
			next = 0
		}
		if gap < -BodyLength {
			next = 1
		}
		if next != leader {
			kind := "pass"
			if len(out) > 0 {
				kind = "repass"
			}
			out = append(out, Event{Time: t, Leader: next, Kind: kind})
			leader = next
		}
		if t == r.Duration {
			break
		}
	}
	return out
}
