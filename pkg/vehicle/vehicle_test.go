package vehicle

import (
	"math"
	"testing"
)

func oracleCar() Config {
	return Config{Name: "oracle", Mass: 1000, Power: 1e6, Brake: 30, MaxSpeed: 100, Grip: 1, DragArea: 0, FrontWeight: 0.5, FrontDrive: 0.5, Width: 2}
}
func near(t *testing.T, got, want float64) {
	t.Helper()
	if math.Abs(got-want) > 1e-8 {
		t.Fatalf("got %.12g, want %.12g", got, want)
	}
}
func TestFlatCircleOracle(t *testing.T) {
	c := oracleCar()
	v, k := 20.0, 0.01
	got := c.Limits(v, k, 0, 0, 1)
	near(t, got.LateralUtilization, v*v*k/Gravity)
	near(t, got.Acceleration, math.Sqrt(Gravity*Gravity-math.Pow(v*v*k, 2)))
	if !got.Feasible {
		t.Fatal("feasible circle rejected")
	}
	if c.Limits(40, k, 0, 0, 1).Feasible {
		t.Fatal("infeasible circle accepted")
	}
}
func TestBankSignsAndSymmetry(t *testing.T) {
	c := oracleCar()
	flat := c.Limits(20, .01, 0, 0, 1)
	adverse := c.Limits(20, .01, 12, 0, 1)
	support := c.Limits(20, .01, -12, 0, 1)
	mirror := c.Limits(20, -.01, 12, 0, 1)
	if !(adverse.LateralUtilization > flat.LateralUtilization && flat.LateralUtilization > support.LateralUtilization) {
		t.Fatal("incorrect bank sign")
	}
	near(t, support.LateralUtilization, mirror.LateralUtilization)
	near(t, support.Acceleration, mirror.Acceleration)
	straight := c.Limits(0, 0, 12, 0, 1)
	near(t, straight.LateralUtilization, math.Tan(12*math.Pi/180))
	if !(straight.Acceleration < c.Limits(0, 0, 0, 0, 1).Acceleration) {
		t.Fatal("straight cross slope did not consume grip")
	}
	v, k, b := 20.0, .01, 12*math.Pi/180
	near(t, adverse.LateralUtilization, (v*v*k*math.Cos(b)+Gravity*math.Sin(b))/(Gravity*math.Cos(b)-v*v*k*math.Sin(b)))
}
func TestGradeAndDrag(t *testing.T) {
	c := oracleCar()
	up, down := c.Limits(15, 0, 0, .1, 1), c.Limits(15, 0, 0, -.1, 1)
	if up.Acceleration >= down.Acceleration || up.Braking <= down.Braking {
		t.Fatal("grade signs incorrect")
	}
	near(t, down.Acceleration-up.Acceleration, 2*Gravity*.1/math.Sqrt(1.01))
	c.DragArea = 1
	drag := c.Limits(15, 0, 0, 0, 1)
	near(t, Gravity-drag.Acceleration, .5*AirDensity*225/c.Mass)
	near(t, drag.Braking-Gravity, .5*AirDensity*225/c.Mass)
	c.Brake = .1
	if c.Limits(0, 0, 0, -.5, 1).Braking >= 0 {
		t.Fatal("impossible downhill braking clamped")
	}
}
func TestTorqueSplit(t *testing.T) {
	c := oracleCar()
	c.FrontDrive = 1
	front := c.Limits(0, 0, 0, 0, 1)
	c.FrontDrive = 0
	rear := c.Limits(0, 0, 0, 0, 1)
	near(t, front.Acceleration, rear.Acceleration)
	c.FrontWeight = .7
	c.FrontDrive = 1
	front = c.Limits(0, 0, 0, 0, 1)
	c.FrontDrive = 0
	rear = c.Limits(0, 0, 0, 0, 1)
	near(t, front.Acceleration, .7*Gravity)
	near(t, rear.Acceleration, .3*Gravity)
	c.FrontDrive = .5
	near(t, c.Limits(0, 0, 0, 0, 1).Acceleration, .6*Gravity)
}
func TestPowerAndPresets(t *testing.T) {
	c := oracleCar()
	c.Power = 10000
	near(t, c.Limits(10, 0, 0, 0, 1).Acceleration, 1)
	if !finite(c.Limits(0, 0, 0, 0, 1).Acceleration) {
		t.Fatal("nonfinite launch")
	}
	for _, name := range Presets() {
		c, err := Preset(name)
		if err != nil {
			t.Fatal(err)
		}
		if err = c.Validate(); err != nil {
			t.Fatal(err)
		}
	}
	c.Mass = math.NaN()
	if c.Validate() == nil {
		t.Fatal("accepted NaN")
	}
}
