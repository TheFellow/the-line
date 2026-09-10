package vehicle

import (
	"math"
	"testing"
)

func TestTyreForcesUseForceBalanceNotNetAcceleration(t *testing.T) {
	c := oracleCar()
	c.DragArea = .8
	env := c.Limits(20, .01, 0, .1, 1)
	f := env.Tyres.Forces(0, 20, 100)
	resistance := .5*AirDensity*c.DragArea*400/c.Mass + Gravity*.1/math.Sqrt(1.01)
	near(t, f.Longitudinal, resistance)
	if f.Phase != "drive" {
		t.Fatal("constant speed uphill still requires positive tyre force")
	}
	coast := env.Tyres.Forces(-resistance, 20, 100)
	near(t, coast.Longitudinal, 0)
	if coast.Phase != "coast" {
		t.Fatal("unpowered coasting mistaken for braking")
	}
	brake := env.Tyres.Forces(-resistance-2, 20, 100)
	if brake.Phase != "brake" {
		t.Fatal("negative tyre force not identified as braking")
	}
	near(t, f.Utilization, math.Hypot(resistance, f.Lateral)/f.Capacity)
}

func TestTyreWidgetEnvelopeMatchesAxleAndPowerLimits(t *testing.T) {
	c := oracleCar()
	c.FrontWeight = .7
	c.FrontDrive = 1
	c.Brake = 4
	c.Power = 10000
	env := c.Limits(10, .01, 0, 0, 1)
	drive, brake := env.Tyres.LongitudinalBounds(env.Tyres.Lateral)
	near(t, drive, env.Acceleration)
	near(t, brake, env.Braking)
	drive, brake = env.Tyres.LongitudinalBounds(0)
	near(t, drive, 1) // 10 kW / (1000 kg * 10 m/s)
	near(t, brake, 4)
	drive, brake = env.Tyres.LongitudinalBounds(env.Tyres.Capacity)
	near(t, drive, 0)
	near(t, brake, 0)
	f := env.Tyres.Forces(env.Acceleration, 10, 100)
	if f.Limit != "power" {
		t.Fatalf("power-limited launch identified as %q", f.Limit)
	}
	if env.Tyres.Forces(0, 10, 10).Limit != "speed_cap" {
		t.Fatal("speed cap not identified")
	}
	if env.Tyres.Forces(-env.Braking, 10, 100).Limit != "brake" {
		t.Fatal("hardware brake limit not identified")
	}
}

func TestOpaqueEnvelopeDoesNotInventChannels(t *testing.T) {
	f := (TyreEnvelope{}).Forces(3, 20, 80)
	if f.Available || f.Phase != "unknown" || f.Limit != "unknown" {
		t.Fatal("opaque model advertised physical channels")
	}
}
