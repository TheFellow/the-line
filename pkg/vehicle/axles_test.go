package vehicle

import (
	"encoding/json"
	"math"
	"testing"
)

func TestTransferLaunchOracles(t *testing.T) {
	c := oracleCar()
	c.FrontWeight = .6
	c.CGHeight = .5
	c.Wheelbase = 2.5
	// Moment balance: front load = .6mg - ma*h/L. With mu=1,
	// FWD a=(.6g)/(1+h/L), RWD a=(.4g)/(1-h/L).
	c.FrontDrive = 1
	near(t, c.Limits(0, 0, 0, 0, 1).Acceleration, .6*Gravity/1.2)
	c.FrontDrive = 0
	near(t, c.Limits(0, 0, 0, 0, 1).Acceleration, .4*Gravity/.8)
}

func TestBrakeBiasOracle(t *testing.T) {
	c := oracleCar()
	c.FrontWeight = .6
	c.FrontBrake = .8
	near(t, c.Limits(0, 0, 0, 0, 1).Braking, .6*Gravity/.8)
	c.CGHeight = .5
	c.Wheelbase = 2.5
	// Front-limited braking b: .8b=.6g+.2b; rear remains feasible.
	near(t, c.Limits(0, 0, 0, 0, 1).Braking, Gravity)
	c.FrontBrake = 1
	near(t, c.Limits(0, 0, 0, 0, 1).Braking, .6*Gravity/.8)
}

func TestDownforceCornerOracleAndDragIndependence(t *testing.T) {
	c := oracleCar()
	c.LiftArea = 3
	c.AeroBalance = .5
	// A balanced car in a flat circle has mv²/R=mu(mg+.5rho ClA v²).
	k := .025
	v := math.Sqrt(Gravity / (k - .5*AirDensity*c.LiftArea/c.Mass))
	e := c.Limits(v*(1-1e-10), k, 0, 0, 1)
	if !e.Feasible {
		t.Fatal("analytic corner speed rejected")
	}
	near(t, e.Utilization, 1)
	if c.Limits(v*1.00001, k, 0, 0, 1).Feasible {
		t.Fatal("above analytic corner speed accepted")
	}
	c.Power = 50000
	c.DragArea = .8
	speed := 50.0
	aero := c.Limits(speed, 0, 0, 0, 1)
	c.LiftArea = 0
	flat := c.Limits(speed, 0, 0, 0, 1)
	near(t, aero.Acceleration, flat.Acceleration) // downforce alone does not create drag
	c.DragArea = 1.2
	if c.Limits(speed, 0, 0, 0, 1).Acceleration >= flat.Acceleration {
		t.Fatal("higher drag must reduce net acceleration")
	}
}

func TestLoadSensitivityAndActualForceWidget(t *testing.T) {
	c := oracleCar()
	c.LiftArea = 2
	c.AeroBalance = .5
	c.LoadSensitivity = .2
	// Downforce exactly equals weight: each axle doubles its reference load,
	// so total grip is 2^(1-s) times its static value.
	speed := math.Sqrt(c.Mass * Gravity / (.5 * AirDensity * c.LiftArea))
	e := c.Limits(speed, 0, 0, 0, 1)
	near(t, e.Tyres.Capacity, Gravity*math.Pow(2, .8))
	c.CGHeight = .5
	c.Wheelbase = 2.5
	c.FrontBrake = .65
	e = c.Limits(20, .008, 0, 0, 1)
	for _, a := range []float64{e.Acceleration, -e.Braking, 0} {
		f := e.Tyres.Forces(a, 20, 100)
		if f.Utilization > 1+1e-9 {
			t.Fatalf("feasible bound has axle utilization %g", f.Utilization)
		}
		d, b := f.LongitudinalBounds(f.Lateral)
		near(t, d, e.Acceleration+e.Tyres.Resistance)
		near(t, b, e.Braking-e.Tyres.Resistance)
	}
}

func TestOptionalFieldsValidationAndPersistence(t *testing.T) {
	c := oracleCar()
	c.CGHeight = .5
	if c.Validate() == nil {
		t.Fatal("transfer accepted without wheelbase")
	}
	c.Wheelbase = 2.5
	c.FrontBrake = .6
	c.LiftArea = 2
	c.AeroBalance = .45
	c.LoadSensitivity = .1
	if err := c.Validate(); err != nil {
		t.Fatal(err)
	}
	data, err := json.Marshal(c)
	if err != nil {
		t.Fatal(err)
	}
	var loaded Config
	if err = json.Unmarshal(data, &loaded); err != nil {
		t.Fatal(err)
	}
	if loaded != c {
		t.Fatal("optional configuration did not round-trip")
	}
	c.LoadSensitivity = .51
	if c.Validate() == nil {
		t.Fatal("nonconcave sensitivity accepted")
	}
}

func TestInactiveOptionsKeepLegacyBits(t *testing.T) {
	for _, name := range Presets() {
		c, _ := Preset(name)
		inactive := c
		inactive.AeroBalance = .7
		inactive.Wheelbase = 2.8
		for _, v := range []float64{0, 10, 30, 60} {
			for _, k := range []float64{-.01, 0, .01} {
				a, b := c.Limits(v, k, 5, .1, 1), inactive.Limits(v, k, 5, .1, 1)
				if math.Float64bits(a.Acceleration) != math.Float64bits(b.Acceleration) || math.Float64bits(a.Braking) != math.Float64bits(b.Braking) || math.Float64bits(a.Utilization) != math.Float64bits(b.Utilization) || a.Feasible != b.Feasible {
					t.Fatal("inactive fields changed legacy arithmetic")
				}
			}
		}
	}
}

func BenchmarkAxleEnvelope(b *testing.B) {
	c := oracleCar()
	c.CGHeight = .3
	c.Wheelbase = 2.7
	c.FrontBrake = .62
	for _, sensitivity := range []float64{0, .08} {
		c.LoadSensitivity = sensitivity
		name := "linear"
		if sensitivity > 0 {
			name = "sensitive"
		}
		b.Run(name, func(b *testing.B) {
			b.ReportAllocs()
			for i := 0; i < b.N; i++ {
				e := c.Limits(20, .008, 5, .03, 1)
				if !e.Feasible {
					b.Fatal("invalid benchmark sample")
				}
			}
		})
	}
}
