package vehicle

import "math"

// RoadState caches the speed-independent projection of a road sample. It is
// immutable and may be shared between concurrent evaluations. Construct it with
// PrepareRoad; the zero value is invalid. Units match Model.Limits.
type RoadState struct {
	curvature, grade, grip, cosGrade, cosBank, sinBank float64
	valid                                              bool
}

func PrepareRoad(curvature, bank, grade, grip float64) RoadState {
	if !finite(curvature) || !finite(bank) || !finite(grade) || !finite(grip) || grip <= 0 {
		return RoadState{}
	}
	beta := bank * math.Pi / 180
	return RoadState{curvature: curvature, grade: grade, grip: grip,
		cosGrade: 1 / math.Sqrt(1+grade*grade), cosBank: math.Cos(beta), sinBank: math.Sin(beta), valid: true}
}

func (r RoadState) forces(speed float64) (normal, lateral float64) {
	centripetal := speed * speed * r.cosGrade * r.cosGrade * r.curvature
	return Gravity*r.cosGrade*r.cosBank - centripetal*r.sinBank,
		centripetal*r.cosBank + Gravity*r.cosGrade*r.sinBank
}

func (c *Config) preparedAxles(speed, normal float64, r RoadState) axleState {
	downforce := .5 * AirDensity * c.LiftArea * speed * speed / c.Mass
	return axleState{enabled: true, config: axleConfig{c.FrontWeight, c.FrontDrive, c.FrontBrake, c.CGHeight, c.Wheelbase, c.LoadSensitivity},
		front: normal*c.FrontWeight + downforce*c.AeroBalance,
		rear:  normal*(1-c.FrontWeight) + downforce*(1-c.AeroBalance), mu: r.grip * c.Grip}
}

// FeasibleOn reports precisely Limits(...).Feasible without solving the unused
// longitudinal boundaries. This is useful for a solver's lateral speed ceiling.
func (c *Config) FeasibleOn(speed float64, r RoadState) bool {
	if !r.valid || !finite(speed) || speed < 0 || c.Mass <= 0 || c.Grip <= 0 {
		return false
	}
	normal, lateral := r.forces(speed)
	if normal <= 0 {
		return false
	}
	if c.axleDynamics() {
		return c.preparedAxles(speed, normal, r).feasible(0, lateral)
	}
	capacity := r.grip * c.Grip * normal
	return capacity > 0 && math.Abs(lateral)/capacity <= 1+1e-10
}

// BoundsOn evaluates exactly the Config force bounds on a prepared road state,
// omitting the large instrumentation payload. Model.Limits remains the public
// replacement-model contract; this fast path is specific to the built-in car.
func (c *Config) BoundsOn(speed float64, r RoadState) (acceleration, braking float64, feasible bool) {
	if !r.valid || !finite(speed) || speed < 0 || c.Mass <= 0 || c.Grip <= 0 {
		return 0, 0, false
	}
	normal, lateral := r.forces(speed)
	if normal <= 0 {
		return 0, 0, false
	}
	var drive, brake float64
	if c.axleDynamics() {
		s := c.preparedAxles(speed, normal, r)
		if !s.feasible(0, lateral) {
			return 0, 0, false
		}
		front, rear := s.capacities(0)
		power := front + rear
		if speed > 1e-6 {
			power = c.Power / (c.Mass * speed)
		}
		// A strictly feasible hardware cap cannot be limited by the tyre
		// boundary. Keep a margin above the bounded root's 1e-9 tolerance,
		// avoiding two expensive roots when power/brakes already limit us.
		drive = power
		if !s.feasible(power+1e-7, lateral) {
			drive = math.Min(s.bound(lateral, 1), power)
		}
		brake = c.Brake
		if !s.feasible(-c.Brake-1e-7, lateral) {
			brake = math.Min(c.Brake, s.bound(lateral, -1))
		}
	} else {
		capacity := r.grip * c.Grip * normal
		if capacity <= 0 || math.Abs(lateral)/capacity > 1+1e-10 {
			return 0, 0, false
		}
		residual := math.Sqrt(math.Max(0, capacity*capacity-lateral*lateral))
		front, rear := residual*c.FrontWeight, residual*(1-c.FrontWeight)
		switch c.FrontDrive {
		case 0:
			drive = rear
		case 1:
			drive = front
		default:
			drive = math.Min(front/c.FrontDrive, rear/(1-c.FrontDrive))
		}
		if speed > 1e-6 {
			drive = math.Min(drive, c.Power/(c.Mass*speed))
		}
		brake = math.Min(c.Brake, residual)
	}
	resistance := .5*AirDensity*c.DragArea*speed*speed/c.Mass + Gravity*r.grade*r.cosGrade
	return drive - resistance, brake + resistance, true
}
