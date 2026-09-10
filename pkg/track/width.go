package track

import "fmt"

// Kerb describes a flat continuation of the banked ribbon, in horizontal metres.
// Grip overrides the named surface only when positive. No bump or suspension
// model is implied. An absent kerb has Width zero.
type Kerb struct {
	Width   float64 `json:"width,omitempty"`
	Surface string  `json:"surface,omitempty"`
	Grip    float64 `json:"grip,omitempty"`
}

func (k Kerb) Friction() float64 {
	if k.Grip > 0 {
		return k.Grip
	}
	if k.Surface == "" {
		return .8
	}
	g, _ := grip(k.Surface)
	return g
}
func (k Kerb) validate() error {
	if !finite(k.Width) || k.Width < 0 || k.Width > 8 {
		return fmt.Errorf("kerb width must be between 0 and 8 m")
	}
	if !finite(k.Grip) || k.Grip < 0 || k.Grip > 2 {
		return fmt.Errorf("kerb grip must be between 0 and 2 (zero uses surface)")
	}
	if k.Surface != "" {
		if _, ok := grip(k.Surface); !ok {
			return fmt.Errorf("unknown kerb surface %q", k.Surface)
		}
	}
	return nil
}

// LeftWidth and RightWidth resolve legacy symmetric widths without changing
// version-1 sampling arithmetic. Explicit side widths take precedence in v2.
func (p Point) LeftWidth() float64 {
	if p.WidthLeft != 0 || p.WidthRight != 0 {
		return p.WidthLeft
	}
	return p.Width / 2
}
func (p Point) RightWidth() float64 {
	if p.WidthLeft != 0 || p.WidthRight != 0 {
		return p.WidthRight
	}
	return p.Width / 2
}
func (s Sample) LeftWidth() float64 {
	if s.WidthLeft != 0 || s.WidthRight != 0 {
		return s.WidthLeft
	}
	return s.Width / 2
}
func (s Sample) RightWidth() float64 {
	if s.WidthLeft != 0 || s.WidthRight != 0 {
		return s.WidthRight
	}
	return s.Width / 2
}
func (s Sample) LeftLimit() float64 {
	w := s.LeftWidth()
	if s.KerbsCountAsRoad {
		w += s.KerbLeft.Width
	}
	return w
}
func (s Sample) RightLimit() float64 {
	w := s.RightWidth()
	if s.KerbsCountAsRoad {
		w += s.KerbRight.Width
	}
	return w
}

// EdgeOffset returns the left (+1) or right (-1) legal boundary.
func (s Sample) EdgeOffset(side float64) float64 {
	if side > 0 {
		return s.LeftLimit()
	}
	return -s.RightLimit()
}

// GripAcross returns the least grip touched by a horizontal circular clearance
// footprint, conservatively charging a tyre straddling asphalt and kerb the
// lower grip. Road-cell endpoint checks further take the minimum along a segment.
func (s Sample) GripAcross(offset, radius float64) float64 {
	g := s.Grip
	if s.KerbsCountAsRoad && offset+radius > s.LeftWidth() && s.KerbLeft.Width > 0 {
		g = min(g, s.KerbLeft.Friction())
	}
	if s.KerbsCountAsRoad && offset-radius < -s.RightWidth() && s.KerbRight.Width > 0 {
		g = min(g, s.KerbRight.Friction())
	}
	return g
}

// Migrate upgrades legacy width fields to explicit v2 side widths. It returns an
// independent scene and preserves equivalent road digests and saved studies.
func Migrate(scene Scene) Scene {
	scene.Points = append([]Point(nil), scene.Points...)
	scene.Study = CloneStudy(scene.Study)
	for i := range scene.Points {
		p := &scene.Points[i]
		p.WidthLeft, p.WidthRight = p.LeftWidth(), p.RightWidth()
		p.Width = 0
	}
	scene.Version = Version
	if scene.Study != nil && scene.Study.Reference != nil && scene.Study.Reference.Scene != nil {
		ref := Migrate(*scene.Study.Reference.Scene)
		scene.Study.Reference.Scene = &ref
	}
	return scene
}
