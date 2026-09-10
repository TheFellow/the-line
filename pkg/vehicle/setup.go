package vehicle

import "fmt"

// SetupField describes an editable SI parameter and its useful step size.
type SetupField struct {
	Key, Label, Unit   string
	Step, DisplayScale float64
}

func SetupFields() []SetupField {
	return []SetupField{
		{"mass", "Mass", "kg", 50, 1}, {"power", "Power", "kW", 25000, .001},
		{"brake", "Braking", "m/s²", .5, 1}, {"max_speed", "Top speed", "km/h", 5 / 3.6, 3.6},
		{"grip", "Tyre grip", "×", .05, 1}, {"drag_area", "Drag area", "m²", .1, 1},
		{"front_weight", "Front weight", "%", .01, 100}, {"front_drive", "Front drive", "%", .05, 100},
		{"width", "Car width", "m", .05, 1},
		{"front_brake", "Front brake", "%", .05, 100}, {"lift_area", "Downforce area", "m²", .25, 1},
		{"aero_balance", "Aero front", "%", .05, 100}, {"wheelbase", "Wheelbase", "m", .5, 1},
		{"cg_height", "CG height", "m", .05, 1}, {"load_sensitivity", "Load sensitivity", "", .05, 1},
	}
}

func (c Config) Value(key string) (float64, error) {
	switch key {
	case "mass":
		return c.Mass, nil
	case "power":
		return c.Power, nil
	case "brake":
		return c.Brake, nil
	case "max_speed":
		return c.MaxSpeed, nil
	case "grip":
		return c.Grip, nil
	case "drag_area":
		return c.DragArea, nil
	case "front_weight":
		return c.FrontWeight, nil
	case "front_drive":
		return c.FrontDrive, nil
	case "width":
		return c.Width, nil
	case "front_brake":
		return c.FrontBrake, nil
	case "lift_area":
		return c.LiftArea, nil
	case "aero_balance":
		return c.AeroBalance, nil
	case "cg_height":
		return c.CGHeight, nil
	case "wheelbase":
		return c.Wheelbase, nil
	case "load_sensitivity":
		return c.LoadSensitivity, nil
	}
	return 0, fmt.Errorf("vehicle: unknown setup field %q", key)
}

// With returns a validated copy with one field changed, leaving c untouched.
func (c Config) With(key string, value float64) (Config, error) {
	switch key {
	case "mass":
		c.Mass = value
	case "power":
		c.Power = value
	case "brake":
		c.Brake = value
	case "max_speed":
		c.MaxSpeed = value
	case "grip":
		c.Grip = value
	case "drag_area":
		c.DragArea = value
	case "front_weight":
		c.FrontWeight = value
	case "front_drive":
		c.FrontDrive = value
	case "width":
		c.Width = value
	case "front_brake":
		c.FrontBrake = value
	case "lift_area":
		c.LiftArea = value
	case "aero_balance":
		c.AeroBalance = value
	case "cg_height":
		c.CGHeight = value
	case "wheelbase":
		c.Wheelbase = value
	case "load_sensitivity":
		c.LoadSensitivity = value
	default:
		return c, fmt.Errorf("vehicle: unknown setup field %q", key)
	}
	return c, c.Validate()
}
