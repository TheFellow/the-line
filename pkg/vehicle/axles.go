package vehicle

import "math"

func (c Config) axleDynamics() bool {
	return c.FrontBrake != 0 || c.LiftArea != 0 || c.CGHeight != 0 || c.LoadSensitivity != 0
}

// axleState holds a frozen road/speed sample. All forces are divided by mass.
// Tyre force at the contact patches transfers load by F*h/L; aero force acts
// through the CG. Lateral demand follows the static CG axle distribution.
// The load-sensitive capacity mu*N*(N/Nref)^(-s) is concave for 0<=s<=0.5,
// making each sign's feasible longitudinal interval convex. Bracketed Newton finds
// its boundary from zero, with no unstable fixed-point iteration.
type axleConfig struct{ FrontWeight, FrontDrive, FrontBrake, CGHeight, Wheelbase, LoadSensitivity float64 }

type axleState struct {
	enabled         bool
	config          axleConfig
	front, rear, mu float64
}

func (s axleState) capacities(longitudinal float64) (front, rear float64) {
	c := s.config
	transfer := 0.0
	if c.CGHeight > 0 && c.Wheelbase > 0 {
		transfer = longitudinal * c.CGHeight / c.Wheelbase
	}
	f, r := s.front-transfer, s.rear+transfer
	if f <= 0 || r <= 0 {
		return 0, 0 // axle lift-off is outside this quasi-static model
	}
	front, rear = s.mu*f, s.mu*r
	if c.LoadSensitivity > 0 {
		front *= loadScale(f/(Gravity*c.FrontWeight), c.LoadSensitivity)
		rear *= loadScale(r/(Gravity*(1-c.FrontWeight)), c.LoadSensitivity)
	}
	return front, rear
}

func (s axleState) feasible(longitudinal, lateral float64) bool {
	front, rear := s.capacities(longitudinal)
	if front <= 0 || rear <= 0 {
		return false
	}
	lf, lr := lateral*s.config.FrontWeight, lateral*(1-s.config.FrontWeight)
	if math.Abs(lf) > front || math.Abs(lr) > rear {
		return false
	}
	q := s.config.FrontDrive
	if longitudinal < 0 {
		q = s.config.FrontBrake
		if q == 0 { // ideal brake allocation, legacy semantics
			return -longitudinal <= math.Sqrt(math.Max(0, front*front-lf*lf))+math.Sqrt(math.Max(0, rear*rear-lr*lr))
		}
	}
	return math.Hypot(q*longitudinal, lf) <= front && math.Hypot((1-q)*longitudinal, lr) <= rear
}

func (s axleState) utilization(longitudinal, lateral float64) float64 {
	front, rear := s.capacities(longitudinal)
	if front <= 0 || rear <= 0 {
		return math.Inf(1)
	}
	lf, lr := lateral*s.config.FrontWeight, lateral*(1-s.config.FrontWeight)
	q := s.config.FrontDrive
	if longitudinal < 0 {
		q = s.config.FrontBrake
		if q == 0 {
			rf, rr := math.Sqrt(math.Max(0, front*front-lf*lf)), math.Sqrt(math.Max(0, rear*rear-lr*lr))
			if rf+rr > 0 {
				q = rf / (rf + rr)
			}
		}
	}
	return math.Max(math.Hypot(q*longitudinal, lf)/front, math.Hypot((1-q)*longitudinal, lr)/rear)
}

func (s axleState) bound(lateral, sign float64) float64 {
	if !s.feasible(0, lateral) {
		return 0
	}
	if s.config.CGHeight == 0 {
		front, rear := s.capacities(0)
		lf, lr := lateral*s.config.FrontWeight, lateral*(1-s.config.FrontWeight)
		rf, rr := math.Sqrt(math.Max(0, front*front-lf*lf)), math.Sqrt(math.Max(0, rear*rear-lr*lr))
		q := s.config.FrontDrive
		if sign < 0 {
			q = s.config.FrontBrake
			if q == 0 {
				return rf + rr
			}
		}
		bound := math.Inf(1)
		if q > 0 {
			bound = rf / q
		}
		if q < 1 {
			bound = math.Min(bound, rr/(1-q))
		}
		return bound
	}
	if s.config.LoadSensitivity == 0 {
		q := s.config.FrontDrive
		if sign < 0 {
			q = s.config.FrontBrake
		}
		if sign > 0 || q > 0 {
			h := s.config.CGHeight / s.config.Wheelbase
			front := linearAxleBound(s.front, -sign*h, s.mu, q, lateral*s.config.FrontWeight)
			rear := linearAxleBound(s.rear, sign*h, s.mu, 1-q, lateral*(1-s.config.FrontWeight))
			return math.Max(0, math.Nextafter(math.Min(front, rear), 0))
		}
		if lateral == 0 { // ideal brakes, straight line; stop before rear lift-off
			h := s.config.CGHeight / s.config.Wheelbase
			return math.Nextafter(math.Min(s.mu*(s.front+s.rear), s.rear/h), 0)
		}
	}
	// Total load is constant under transfer; this is an upper bound even with
	// sensitivity (each axle individually receives at most the total load).
	total := s.front + s.rear
	c := s.config
	hi := s.mu * total
	if c.LoadSensitivity > 0 {
		hi = s.mu * math.Pow(total, 1-c.LoadSensitivity) * (math.Pow(Gravity*c.FrontWeight, c.LoadSensitivity) + math.Pow(Gravity*(1-c.FrontWeight), c.LoadSensitivity))
	}
	// Keep both axles loaded before using a differentiable force boundary.
	h := c.CGHeight / c.Wheelbase
	if sign > 0 {
		hi = math.Min(hi, s.front/h)
	} else {
		hi = math.Min(hi, s.rear/h)
	}
	hi = math.Nextafter(hi, 0)
	lo, x := 0.0, hi*.5
	for i := 0; i < 40; i++ {
		gap, derivative := s.gapDerivative(x, lateral, sign)
		if gap <= 0 {
			lo = x
		} else {
			hi = x
		}
		if math.Abs(gap) < 1e-10 {
			candidate := math.Max(0, x-1e-9)
			if s.feasible(sign*candidate, lateral) {
				return candidate
			}
		}
		next := x - gap/derivative
		if !finite(next) || next <= lo || next >= hi || derivative <= 0 {
			next = (lo + hi) * .5
		}
		x = next
	}
	return lo
}

func (c Config) axleLimits(speed, curvature, bank, grade, grip float64) Envelope {
	beta := bank * math.Pi / 180
	cosGrade := 1 / math.Sqrt(1+grade*grade)
	centripetal := speed * speed * cosGrade * cosGrade * curvature
	normal := Gravity*cosGrade*math.Cos(beta) - centripetal*math.Sin(beta)
	lateral := centripetal*math.Cos(beta) + Gravity*cosGrade*math.Sin(beta)
	downforce := .5 * AirDensity * c.LiftArea * speed * speed / c.Mass
	if normal <= 0 {
		return Envelope{}
	}
	s := axleState{enabled: true, config: axleConfig{c.FrontWeight, c.FrontDrive, c.FrontBrake, c.CGHeight, c.Wheelbase, c.LoadSensitivity}, front: normal*c.FrontWeight + downforce*c.AeroBalance, rear: normal*(1-c.FrontWeight) + downforce*(1-c.AeroBalance), mu: grip * c.Grip}
	front, rear := s.capacities(0)
	capacity := front + rear
	utilization := math.Max(math.Abs(lateral*c.FrontWeight)/front, math.Abs(lateral*(1-c.FrontWeight))/rear)
	if !s.feasible(0, lateral) {
		return Envelope{Utilization: utilization}
	}
	driveGrip := s.bound(lateral, 1)
	power := capacity
	if speed > 1e-6 {
		power = c.Power / (c.Mass * speed)
	}
	drive := math.Min(driveGrip, power)
	brake := math.Min(c.Brake, s.bound(lateral, -1))
	resistance := .5*AirDensity*c.DragArea*speed*speed/c.Mass + Gravity*grade*cosGrade
	return Envelope{Acceleration: drive - resistance, Braking: brake + resistance, Utilization: utilization, Feasible: true, Tyres: TyreEnvelope{Available: true, Lateral: lateral, Capacity: capacity, Resistance: resistance, Drive: drive, Brake: brake, DriveGrip: driveGrip, Power: power, BrakeLimit: c.Brake, axles: s}}
}

// linearAxleBound solves q²F²+Y² <= mu²(N+alpha*F)², starting
// inside the feasible interval. Stable quadratic roots avoid subtracting
// nearly equal numbers; a growing axle may impose no finite upper bound.
func linearAxleBound(normal, alpha, mu, q, lateral float64) float64 {
	a := q*q - mu*mu*alpha*alpha
	b := -2 * mu * mu * normal * alpha
	c := lateral*lateral - mu*mu*normal*normal
	if a <= 0 && b <= 0 {
		return math.Inf(1)
	}
	discriminant := math.Max(0, b*b-4*a*c)
	root := 0.0
	if b < 0 {
		root = (-b + math.Sqrt(discriminant)) / (2 * a)
	} else if denominator := b + math.Sqrt(discriminant); denominator > 0 {
		root = -2 * c / denominator
	}
	if alpha < 0 {
		root = math.Min(root, -normal/alpha)
	}
	return root
}

// gapDerivative evaluates the convex active constraint for safeguarded Newton
// steps. Every step remains bracketed; failure to converge returns a feasible
// lower bound after 40 iterations rather than exceeding an axle circle.
func (s axleState) gapDerivative(force, lateral, sign float64) (float64, float64) {
	c := s.config
	h := c.CGHeight / c.Wheelbase
	nf, nr := s.front-sign*h*force, s.rear+sign*h*force
	if nf <= 0 || nr <= 0 {
		return math.Inf(1), math.Inf(1)
	}
	cf, cr := s.mu*nf, s.mu*nr
	df, dr := -sign*h*s.mu, sign*h*s.mu
	if c.LoadSensitivity > 0 {
		sf, sr := loadScale(nf/(Gravity*c.FrontWeight), c.LoadSensitivity), loadScale(nr/(Gravity*(1-c.FrontWeight)), c.LoadSensitivity)
		cf *= sf
		cr *= sr
		df *= sf * (1 - c.LoadSensitivity)
		dr *= sr * (1 - c.LoadSensitivity)
	}
	lf, lr := lateral*c.FrontWeight, lateral*(1-c.FrontWeight)
	if math.Abs(lf) > cf || math.Abs(lr) > cr {
		return math.Inf(1), math.Inf(1)
	}
	q := c.FrontDrive
	if sign < 0 {
		q = c.FrontBrake
		if q == 0 {
			rf, rr := math.Sqrt(math.Max(0, cf*cf-lf*lf)), math.Sqrt(math.Max(0, cr*cr-lr*lr))
			return force - rf - rr, 1 - cf*df/rf - cr*dr/rr
		}
	}
	front, rear := math.Hypot(q*force, lf), math.Hypot((1-q)*force, lr)
	if front-cf > rear-cr {
		return front - cf, q*q*force/front - df
	}
	return rear - cr, (1-q)*(1-q)*force/rear - dr
}

// Loads are strictly positive and the sensitivity exponent is in (0, .5].
// Exp/Log implements the same power law without math.Pow's general signed,
// integer-exponent and infinite-input branches in this hot Newton iteration.
func loadScale(relativeLoad, sensitivity float64) float64 {
	return math.Exp(-sensitivity * math.Log(relativeLoad))
}
