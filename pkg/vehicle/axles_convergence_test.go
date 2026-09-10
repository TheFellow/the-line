package vehicle

import (
	"math"
	"math/rand/v2"
	"testing"
)

// axleOracle reconstructs contact-patch loads by moment balance and combines
// the two tyre circles, independently of bound, capacities and gapDerivative.
func axleOracle(s axleState, force, lateral float64) float64 {
	c := s.config
	transfer := force * c.CGHeight / c.Wheelbase
	if c.CGHeight == 0 {
		transfer = 0
	}
	front, rear := s.front-transfer, s.rear+transfer
	if front <= 0 || rear <= 0 {
		return math.Inf(1)
	}
	cf := s.mu * front * math.Pow(front/(Gravity*c.FrontWeight), -c.LoadSensitivity)
	cr := s.mu * rear * math.Pow(rear/(Gravity*(1-c.FrontWeight)), -c.LoadSensitivity)
	lf, lr := lateral*c.FrontWeight, lateral*(1-c.FrontWeight)
	q := c.FrontDrive
	if force < 0 {
		q = c.FrontBrake
		if q == 0 {
			gap := math.Max(math.Abs(lf)-cf, math.Abs(lr)-cr)
			if gap > 0 {
				return gap
			}
			return math.Max(math.Max(-front, -rear), math.Max(gap, -force-math.Sqrt(cf*cf-lf*lf)-math.Sqrt(cr*cr-lr*lr)))
		}
	}
	return math.Max(math.Max(-front, -rear), math.Max(math.Hypot(q*force, lf)-cf, math.Hypot((1-q)*force, lr)-cr))
}

func TestAxleBoundaryResidualGrid(t *testing.T) {
	rng := rand.New(rand.NewPCG(42, 91))
	maxResidual, maxError := 0.0, 0.0
	for i := 0; i < 4000; i++ {
		weight := .25 + .5*rng.Float64()
		s := axleState{config: axleConfig{FrontWeight: weight, FrontDrive: rng.Float64(), FrontBrake: .1 + .8*rng.Float64(), CGHeight: .1 + .7*rng.Float64(), Wheelbase: 2 + 1.5*rng.Float64(), LoadSensitivity: .01 + .49*rng.Float64()}, front: Gravity * weight, rear: Gravity * (1 - weight), mu: .4 + 1.6*rng.Float64()}
		// Include axle-only drive and ideal brake allocation, plus aero imbalance.
		if i%3 == 0 {
			s.config.FrontDrive = 0
		}
		if i%3 == 1 {
			s.config.FrontDrive = 1
		}
		if i%4 == 0 {
			s.config.FrontBrake = 0
		}
		s.front += rng.Float64() * 8
		s.rear += rng.Float64() * 8
		maxLateral := math.Min(s.mu*s.front*math.Pow(s.front/(Gravity*weight), -s.config.LoadSensitivity)/weight, s.mu*s.rear*math.Pow(s.rear/(Gravity*(1-weight)), -s.config.LoadSensitivity)/(1-weight))
		lateral := maxLateral * rng.Float64() * .999
		if i%20 == 0 {
			lateral = 0
		}
		if i%20 == 1 {
			lateral = maxLateral * (1 - 1e-8)
		}
		if i%2 == 0 {
			lateral = -lateral
		}
		for _, sign := range []float64{-1, 1} {
			got := s.bound(lateral, sign)
			// A pure bisection of independently reconstructed forces provides a root
			// oracle; it does not reuse the production Newton residual or derivative.
			lo, hi := 0.0, 1000.0
			for range 70 {
				mid := (lo + hi) / 2
				if axleOracle(s, sign*mid, lateral) <= 0 {
					lo = mid
				} else {
					hi = mid
				}
			}
			residual := axleOracle(s, sign*got, lateral)
			maxResidual = math.Max(maxResidual, math.Abs(residual))
			maxError = math.Max(maxError, math.Abs(got-lo))
			if !s.feasible(sign*got, lateral) || math.Abs(residual) > 1e-10 || math.Abs(got-lo) > 1e-9 {
				t.Fatalf("sample %d sign %g: bound %.15g oracle %.15g residual %.3g; state %+v lateral %g", i, sign, got, lo, residual, s, lateral)
			}
		}
	}
	t.Logf("8000 signed bounds: maximum absolute residual %.3g m/s², root error %.3g m/s²", maxResidual, maxError)
}

func TestNoTransferBoundsStayInsideAxleCircles(t *testing.T) {
	rng := rand.New(rand.NewPCG(1, 2))
	for range 2000 {
		c := oracleCar()
		c.FrontWeight = .2 + .6*rng.Float64()
		c.FrontDrive = rng.Float64()
		c.FrontBrake = .1 + .8*rng.Float64()
		c.LoadSensitivity = .2
		e := c.Limits(10, rng.Float64()*.04, 0, 0, 1)
		for _, force := range []float64{e.Tyres.Drive, -e.Tyres.Brake} {
			if !e.Tyres.axles.feasible(force, e.Tyres.Lateral) {
				t.Fatalf("closed-form bound outside axle circle: %g %+v", force, c)
			}
		}
	}
}

func TestInvalidTransferRejectedByForceAPIs(t *testing.T) {
	c := oracleCar()
	c.CGHeight = .5
	for _, wheelbase := range []float64{0, -1, .49, math.NaN(), math.Inf(1)} {
		c.Wheelbase = wheelbase
		if c.Validate() == nil {
			t.Fatalf("wheelbase %g accepted", wheelbase)
		}
		road := PrepareRoad(0, 0, 0, 1)
		_, _, feasible := c.BoundsOn(10, road)
		if c.Limits(10, 0, 0, 0, 1).Feasible || c.FeasibleOn(10, road) || feasible {
			t.Fatalf("wheelbase %g reports valid force envelope", wheelbase)
		}
	}
}

func TestAllRearDownforceKeepsFrontCornerLimit(t *testing.T) {
	c := oracleCar()
	c.FrontWeight = .45
	c.FrontDrive = 0
	c.LiftArea = 3
	speed, curvature := 15.0, .025
	rear := c.Limits(speed, curvature, 0, 0, 1)
	c.AeroBalance = c.FrontWeight
	balanced := c.Limits(speed, curvature, 0, 0, 1)
	c.LiftArea = 0
	plain := c.Limits(speed, curvature, 0, 0, 1)
	near(t, rear.LateralUtilization, plain.LateralUtilization)
	if balanced.LateralUtilization >= rear.LateralUtilization {
		t.Fatal("balanced aero should reduce the binding axle's lateral utilization")
	}
}
