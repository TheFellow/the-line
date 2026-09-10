// Package vehicle supplies a quasi-static friction-circle model with fixed
// axle load and torque distributions. Presets are fictional, illustrative cars.
// It omits load transfer, downforce, tyre slip, suspension and crest unloading.
package vehicle

import (
	"fmt"
	"math"
)

const Gravity = 9.80665
const AirDensity = 1.225 // kg/m³

// Config uses SI units: kg, watts, m/s, m/s², and CdA in square metres.
// FrontWeight is static front normal-load fraction. FrontDrive is a fixed
// front torque fraction; neither distribution changes with acceleration.
type Config struct {
	Name        string  `json:"name"`
	Mass        float64 `json:"mass"`
	Power       float64 `json:"power"`
	Brake       float64 `json:"brake"`
	MaxSpeed    float64 `json:"max_speed"`
	Grip        float64 `json:"grip"`
	DragArea    float64 `json:"drag_area"`
	FrontWeight float64 `json:"front_weight"`
	FrontDrive  float64 `json:"front_drive"`
	Width       float64 `json:"width"`
}

// Model can be replaced without changing track geometry or optimization.
// Speed is spatial speed in m/s; curvature is signed horizontal curvature in
// 1/m; bank is degrees, positive with the left edge raised; grade is rise/run.
// Grip is the surface coefficient, multiplied by the vehicle tyre Grip.
type Model interface {
	Parameters() Config
	Limits(speed, curvature, bank, grade, grip float64) Envelope
}

// Envelope reports net acceleration bounds in m/s². Acceleration is maximum
// signed forward acceleration; Braking is maximum deceleration magnitude.
// Either may be negative if resistance or gravity exceeds tyre capability.
// Feasible reports lateral/load feasibility, not the sign of these bounds.
// Utilization is absolute lateral tyre demand divided by total tyre capacity.
type Envelope struct {
	Acceleration, Braking, Utilization float64
	Feasible                           bool
	// Tyres exposes the force balance used above. Custom models may leave it
	// unavailable; presentation must not invent channels for an opaque model.
	Tyres TyreEnvelope
}

func finite(v float64) bool { return !math.IsNaN(v) && !math.IsInf(v, 0) }
func (c Config) Validate() error {
	if c.Name == "" {
		return fmt.Errorf("vehicle: name is required")
	}
	values := []struct {
		name      string
		v, lo, hi float64
	}{{"mass", c.Mass, 50, 10000}, {"power", c.Power, 1, 5e6}, {"brake", c.Brake, 0.1, 40}, {"max_speed", c.MaxSpeed, 1, 200}, {"grip", c.Grip, 0.1, 3}, {"drag_area", c.DragArea, 0, 10}, {"front_weight", c.FrontWeight, 0.01, 0.99}, {"front_drive", c.FrontDrive, 0, 1}, {"width", c.Width, 0.5, 4}}
	for _, x := range values {
		if !finite(x.v) || x.v < x.lo || x.v > x.hi {
			return fmt.Errorf("vehicle: %s must be finite and between %g and %g", x.name, x.lo, x.hi)
		}
	}
	return nil
}
func (c Config) Parameters() Config { return c }

// Limits resolves gravity and horizontal centripetal acceleration in the
// banked road frame. Lateral force is distributed in proportion to static axle
// loads; brakes use both axles, drive obeys the fixed FrontDrive torque split.
// Grade projects spatial speed horizontally and gravity along the slope.
// This decoupled bank/grade approximation is exact for the flat constant-bank
// circle oracle, but is not full coupled three-dimensional vehicle dynamics.
func (c Config) Limits(speed, curvature, bank, grade, grip float64) Envelope {
	for _, v := range []float64{speed, curvature, bank, grade, grip} {
		if !finite(v) {
			return Envelope{}
		}
	}
	if speed < 0 || grip <= 0 || c.Mass <= 0 || c.Grip <= 0 {
		return Envelope{}
	}
	beta := bank * math.Pi / 180
	cosGrade := 1 / math.Sqrt(1+grade*grade)
	lateralAccel := speed * speed * cosGrade * cosGrade * curvature
	normal := Gravity*cosGrade*math.Cos(beta) - lateralAccel*math.Sin(beta)
	lateral := lateralAccel*math.Cos(beta) + Gravity*cosGrade*math.Sin(beta)
	capacity := grip * c.Grip * normal
	if normal <= 0 || capacity <= 0 {
		return Envelope{}
	}
	utilization := math.Abs(lateral) / capacity
	if utilization > 1+1e-10 {
		return Envelope{Utilization: utilization}
	}
	residual := math.Sqrt(math.Max(0, capacity*capacity-lateral*lateral))
	front, rear := residual*c.FrontWeight, residual*(1-c.FrontWeight)
	var drive float64
	switch c.FrontDrive {
	case 0:
		drive = rear
	case 1:
		drive = front
	default:
		drive = math.Min(front/c.FrontDrive, rear/(1-c.FrontDrive))
	}
	driveGrip := drive
	power := capacity
	if speed > 1e-6 {
		power = c.Power / (c.Mass * speed)
		drive = math.Min(drive, power)
	}
	resistance := 0.5*AirDensity*c.DragArea*speed*speed/c.Mass + Gravity*grade*cosGrade
	scale := 1.0
	if c.FrontDrive > 0 {
		scale = math.Min(scale, c.FrontWeight/c.FrontDrive)
	}
	if c.FrontDrive < 1 {
		scale = math.Min(scale, (1-c.FrontWeight)/(1-c.FrontDrive))
	}
	brake := math.Min(c.Brake, residual)
	return Envelope{Acceleration: drive - resistance, Braking: brake + resistance, Utilization: utilization, Feasible: true,
		Tyres: TyreEnvelope{Available: true, Lateral: lateral, Capacity: capacity, Resistance: resistance,
			Drive: drive, Brake: brake, DriveGrip: driveGrip, Power: power,
			DriveScale: scale, BrakeScale: 1, BrakeLimit: c.Brake}}
}
func Presets() []string { return []string{"road", "gt", "rally"} }
func Preset(name string) (Config, error) {
	var c Config
	switch name {
	case "road", "club":
		c = Config{Name: "Club Sport · RWD", Mass: 1250, Power: 155000, Brake: 10, MaxSpeed: 65, Grip: 1.05, DragArea: 0.66, FrontWeight: 0.52, FrontDrive: 0, Width: 1.8}
	case "gt":
		c = Config{Name: "GT Prototype · RWD", Mass: 1180, Power: 390000, Brake: 14, MaxSpeed: 88, Grip: 1.45, DragArea: 0.82, FrontWeight: 0.45, FrontDrive: 0, Width: 1.95}
	case "rally":
		c = Config{Name: "Rally Cross · AWD", Mass: 1320, Power: 235000, Brake: 11, MaxSpeed: 62, Grip: 1.1, DragArea: 0.8, FrontWeight: 0.57, FrontDrive: 0.45, Width: 1.85}
	default:
		return Config{}, fmt.Errorf("vehicle: unknown preset %q", name)
	}
	return c, nil
}
