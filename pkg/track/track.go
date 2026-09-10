// Package track describes open and periodic road ribbons in metres, with z up and left turns
// positive. Width is horizontal; positive bank raises the left road edge.
package track

import (
	"fmt"
	"math"

	"github.com/TheFellow/the-line/pkg/vehicle"
)

const Version = 2

type Vec3 struct{ X, Y, Z float64 }

func (v Vec3) Add(w Vec3) Vec3    { return Vec3{v.X + w.X, v.Y + w.Y, v.Z + w.Z} }
func (v Vec3) Sub(w Vec3) Vec3    { return Vec3{v.X - w.X, v.Y - w.Y, v.Z - w.Z} }
func (v Vec3) Mul(s float64) Vec3 { return Vec3{v.X * s, v.Y * s, v.Z * s} }
func (v Vec3) Length() float64    { return math.Sqrt(v.X*v.X + v.Y*v.Y + v.Z*v.Z) }

type Point struct {
	X          float64 `json:"x"`
	Y          float64 `json:"y"`
	Z          float64 `json:"z"`
	Width      float64 `json:"width,omitempty"` // Deprecated symmetric v1 width.
	WidthLeft  float64 `json:"width_left,omitempty"`
	WidthRight float64 `json:"width_right,omitempty"`
	KerbLeft   Kerb    `json:"kerb_left,omitzero"`
	KerbRight  Kerb    `json:"kerb_right,omitzero"`
	Bank       float64 `json:"bank"`
	Surface    string  `json:"surface"`
}

func (p Point) Position() Vec3 { return Vec3{p.X, p.Y, p.Z} }

type Scene struct {
	Closed           bool            `json:"closed,omitempty"` // Periodic lap; endpoint caps do not apply.
	KerbsCountAsRoad bool            `json:"kerbs_count_as_road,omitempty"`
	VehicleConfig    *vehicle.Config `json:"vehicle_config,omitempty"`
	Study            *Study          `json:"study,omitempty"`
	Version          int             `json:"version"`
	Name             string          `json:"name"`
	Vehicle          string          `json:"vehicle"`
	EntrySpeed       float64         `json:"entry_speed_cap"`
	ExitSpeed        float64         `json:"exit_speed_cap"`
	Points           []Point         `json:"points"`
}

type Surface struct {
	Name string  `json:"name"`
	Grip float64 `json:"grip"`
}

func Surfaces() []Surface {
	return []Surface{{"asphalt", 1}, {"wet", 0.65}, {"gravel", 0.55}, {"dirt", 0.45}, {"ice", 0.16}}
}
func grip(name string) (float64, bool) {
	for _, s := range Surfaces() {
		if s.Name == name {
			return s.Grip, true
		}
	}
	return 0, false
}
func finite(v float64) bool { return !math.IsNaN(v) && !math.IsInf(v, 0) }

// Validate checks the serialized fields. SampleRoad also validates the interpolated ribbon.
func (s Scene) Validate() error {
	if s.VehicleConfig != nil {
		if err := s.VehicleConfig.Validate(); err != nil {
			return err
		}
	}
	if s.Study != nil {
		if err := s.Study.Validate(); err != nil {
			return err
		}
	}
	if s.Version != 1 && s.Version != Version {
		return fmt.Errorf("track: unsupported version %d (want %d)", s.Version, Version)
	}
	if s.Name == "" {
		return fmt.Errorf("track: name is required")
	}
	if len(s.Points) < 2 || len(s.Points) > 1000 {
		return fmt.Errorf("track: need 2 to 1000 control points")
	}
	if !finite(s.EntrySpeed) || !finite(s.ExitSpeed) || s.EntrySpeed < 0 || s.ExitSpeed < 0 || s.EntrySpeed > 200 || s.ExitSpeed > 200 {
		return fmt.Errorf("track: speed caps must be finite and between 0 and 200 m/s")
	}
	for i, p := range s.Points {
		for _, v := range []float64{p.X, p.Y, p.Z, p.Width, p.WidthLeft, p.WidthRight, p.Bank} {
			if !finite(v) {
				return fmt.Errorf("track: point %d has a non-finite value", i)
			}
		}
		if math.Abs(p.X) > 1e6 || math.Abs(p.Y) > 1e6 || math.Abs(p.Z) > 1e4 {
			return fmt.Errorf("track: point %d exceeds coordinate bounds", i)
		}
		if p.LeftWidth() < 1.5 || p.RightWidth() < 1.5 || p.LeftWidth()+p.RightWidth() > 80 {
			return fmt.Errorf("track: point %d side widths must each be at least 1.5 m and total at most 80 metres", i)
		}
		if s.Version == 1 && (p.WidthLeft != 0 || p.WidthRight != 0 || p.KerbLeft.Width != 0 || p.KerbRight.Width != 0 || s.KerbsCountAsRoad) {
			return fmt.Errorf("track: asymmetric widths and kerbs require version 2")
		}
		for _, k := range []Kerb{p.KerbLeft, p.KerbRight} {
			if err := k.validate(); err != nil {
				return fmt.Errorf("track: point %d: %w", i, err)
			}
		}
		if math.Abs(p.Bank) > 35 {
			return fmt.Errorf("track: point %d bank must be between -35 and 35 degrees", i)
		}
		if _, ok := grip(p.Surface); !ok {
			return fmt.Errorf("track: point %d has unknown surface %q", i, p.Surface)
		}
		if i > 0 {
			q := s.Points[i-1]
			d := math.Hypot(p.X-q.X, p.Y-q.Y)
			if d < 1 {
				return fmt.Errorf("track: points %d and %d must be at least 1 metre apart in plan", i-1, i)
			}
			if math.Abs(p.Z-q.Z)/d > 0.5 {
				return fmt.Errorf("track: segment %d grade exceeds 50%%", i-1)
			}
		}
	}
	return s.validateClosed()
}

type Sample struct {
	Closed                      bool // The final sample duplicates the first at full lap station.
	WidthLeft, WidthRight       float64
	KerbLeft, KerbRight         Kerb
	KerbsCountAsRoad            bool
	Position, Normal            Vec3
	S, Width, Bank, Grade, Grip float64
	Surface                     string
}

// AtOffset returns the surface position at a horizontal leftward offset.
func (s Sample) AtOffset(offset float64) Vec3 {
	return s.Position.Add(s.Normal.Mul(offset)).Add(Vec3{Z: offset * math.Tan(s.Bank*math.Pi/180)})
}

// SampleRoad interpolates source points with a C2 cubic spline in spatial
// chord length: natural for open sequences, periodic for closed laps. Width and bank use bounded C1 interpolation in road station.
// Every source point remains a station, so categorical surface transitions are retained.
// S is centreline spatial arc length; Normal and Grade use the horizontal tangent.
func SampleRoad(scene Scene, spacing float64) ([]Sample, error) {
	if err := scene.Validate(); err != nil {
		return nil, err
	}
	if !finite(spacing) || spacing < 0.25 || spacing > 20 {
		return nil, fmt.Errorf("track: spacing must be between 0.25 and 20 metres")
	}
	p := scene.Points
	spline := newCenterSpline(p)
	if scene.Closed {
		spline = newPeriodicCenterSpline(p)
		p = append(append([]Point(nil), p...), p[0])
	}
	var out []Sample
	for i := 0; i < len(p)-1; i++ {
		a := p[i].Position()
		chord := spline.spans[i]
		curve := func(t float64) (Vec3, Vec3) {
			position, derivative, _ := spline.at(i, t)
			return position, derivative
		}
		dense := max(32, int(math.Ceil(chord/0.25)))
		lengths := make([]float64, dense+1)
		prev := a
		for j := 1; j <= dense; j++ {
			v, _ := curve(float64(j) / float64(dense))
			lengths[j] = lengths[j-1] + v.Sub(prev).Length()
			prev = v
		}
		count := max(1, int(math.Ceil(lengths[dense]/spacing)))
		cursor := 1
		for j := 0; j <= count; j++ {
			if i > 0 && j == 0 {
				continue
			}
			target := float64(j) / float64(count) * lengths[dense]
			for cursor < dense && lengths[cursor] < target {
				cursor++
			}
			den := lengths[cursor] - lengths[cursor-1]
			if !finite(den) || den <= 0 {
				return nil, fmt.Errorf("track: segment %d has coincident arc-length samples", i)
			}
			t := (float64(cursor-1) + (target-lengths[cursor-1])/den) / float64(dense)
			pos, der := curve(t)
			horizontal := math.Hypot(der.X, der.Y)
			if !finite(horizontal) || horizontal < 1e-6 {
				return nil, fmt.Errorf("track: segment %d has a stationary tangent", i)
			}
			surface := p[i].Surface
			if j == count {
				surface = p[i+1].Surface
			}
			mu, _ := grip(surface)
			// Smoothstep in arc-length fraction has zero end slopes, preserving
			// C1 bank and width at knots without overshooting their input bounds.
			u := float64(j) / float64(count)
			blend := u * u * (3 - 2*u)
			sample := Sample{Closed: scene.Closed, Position: pos, Normal: Vec3{-der.Y / horizontal, der.X / horizontal, 0}, Width: p[i].Width + (p[i+1].Width-p[i].Width)*blend, Bank: p[i].Bank + (p[i+1].Bank-p[i].Bank)*blend, Grade: der.Z / horizontal, Grip: mu, Surface: surface}
			sample.WidthLeft = p[i].LeftWidth() + (p[i+1].LeftWidth()-p[i].LeftWidth())*blend
			sample.WidthRight = p[i].RightWidth() + (p[i+1].RightWidth()-p[i].RightWidth())*blend
			sample.Width = sample.WidthLeft + sample.WidthRight
			sample.KerbsCountAsRoad = scene.KerbsCountAsRoad
			sample.KerbLeft = sampleKerb(p[i].KerbLeft, p[i+1].KerbLeft, blend)
			sample.KerbRight = sampleKerb(p[i].KerbRight, p[i+1].KerbRight, blend)
			if len(out) > 0 {
				last := out[len(out)-1]
				sample.S = last.S + pos.Sub(last.Position).Length()
			}
			out = append(out, sample)
			if len(out) > 20000 {
				return nil, fmt.Errorf("track: road exceeds 20000 samples; shorten the scene or increase spacing")
			}
		}
	}
	if scene.Closed {
		length := out[len(out)-1].S
		out[len(out)-1] = out[0]
		out[len(out)-1].S = length
	}
	outer := append([]Sample(nil), out...)
	for i := range outer {
		outer[i].KerbsCountAsRoad = true
	}
	if err := validateRibbon(outer); err != nil {
		return nil, err
	}
	return out, nil
}
func cross(a, b, c Vec3) float64 { return (b.X-a.X)*(c.Y-a.Y) - (b.Y-a.Y)*(c.X-a.X) }
func intersects(a, b, c, d Vec3) bool {
	return cross(a, b, c)*cross(a, b, d) < -1e-8 && cross(c, d, a)*cross(c, d, b) < -1e-8
}
func validateRibbon(road []Sample) error {
	type edge struct {
		a, b    Vec3
		station int
	}
	edges := make([]edge, 0, len(road)*2)
	for i := 0; i < len(road)-1; i++ {
		a, b := road[i], road[i+1]
		q := []Vec3{a.AtOffset(-a.RightLimit()), b.AtOffset(-b.RightLimit()), b.AtOffset(b.LeftLimit()), a.AtOffset(a.LeftLimit())}
		for j := 0; j < 4; j++ {
			if cross(q[j], q[(j+1)%4], q[(j+2)%4]) <= 1e-7 {
				return fmt.Errorf("track: folded or degenerate road ribbon at station %.1f m; spread control points or reduce width", a.S)
			}
		}
		edges = append(edges, edge{q[0], q[1], i}, edge{q[3], q[2], i})
	}
	for i, a := range edges {
		for _, b := range edges[i+1:] {
			if abs(a.station-b.station) <= 1 || road[0].Closed && abs(a.station-b.station) == len(road)-2 {
				continue
			}
			if math.Max(a.a.X, a.b.X) < math.Min(b.a.X, b.b.X) || math.Max(b.a.X, b.b.X) < math.Min(a.a.X, a.b.X) || math.Max(a.a.Y, a.b.Y) < math.Min(b.a.Y, b.b.Y) || math.Max(b.a.Y, b.b.Y) < math.Min(a.a.Y, a.b.Y) {
				continue
			}
			if intersects(a.a, a.b, b.a, b.b) {
				return fmt.Errorf("track: road ribbon intersects itself near station %.1f m", road[a.station].S)
			}
		}
	}
	return nil
}
func abs(v int) int {
	if v < 0 {
		return -v
	}
	return v
}
