package vehicle

import (
	"math"
	"testing"
)

func TestPreparedBoundsMatchPublicEnvelope(t *testing.T) {
	c, _ := Preset("gt")
	configurations := []Config{c}
	for _, q := range []float64{0, .55, 1} {
		rich := c
		rich.FrontBrake = q
		rich.CGHeight = .35
		rich.Wheelbase = 2.8
		rich.LiftArea = 3
		rich.AeroBalance = .43
		configurations = append(configurations, rich)
		rich.LoadSensitivity = .12
		configurations = append(configurations, rich)
	}
	for _, c := range configurations {
		for _, bank := range []float64{-20, 0, 11} {
			for _, grade := range []float64{-.2, 0, .3} {
				for _, curvature := range []float64{-.04, 0, .025} {
					for _, grip := range []float64{.3, 1} {
						for _, speed := range []float64{0, 5, 20, 45, 80} {
							road := PrepareRoad(curvature, bank, grade, grip)
							want := c.Limits(speed, curvature, bank, grade, grip)
							a, b, ok := c.BoundsOn(speed, road)
							if ok != want.Feasible || c.FeasibleOn(speed, road) != want.Feasible || (ok && (a != want.Acceleration || b != want.Braking)) {
								t.Fatalf("prepared mismatch car=%+v speed=%g road=%+v: (%g,%g,%v) want %+v", c, speed, road, a, b, ok, want)
							}
						}
					}
				}
			}
		}
	}
	for _, r := range []RoadState{{}, PrepareRoad(0, math.NaN(), 0, 1)} {
		if c.FeasibleOn(1, r) {
			t.Fatal("invalid road accepted")
		}
	}
}

func TestLoadPowerLawEquivalent(t *testing.T) {
	// Independently compare the specialized positive-base power to the general
	// library power over loads well beyond the supported operating envelope.
	for _, s := range []float64{.001, .12, .5} {
		for x := 1e-6; x < 1e6; x *= 1.37 {
			want := math.Pow(x, -s)
			if got := loadScale(x, s); math.Abs(got-want) > 2e-15*want {
				t.Fatalf("load scale %g exponent %g: %.17g want %.17g", x, s, got, want)
			}
		}
	}
}
