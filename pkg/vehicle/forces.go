package vehicle

import "math"

// TyreEnvelope exposes specific forces (force divided by vehicle mass), m/s².
// Lateral is signed toward road-left. Drive and Brake are positive longitudinal
// capacities after lateral demand. Resistance is drag plus signed grade gravity.
// The shape is sqrt(Capacity²-lateral²), scaled separately for the driven/braked
// axle allocation, then clipped by Power and BrakeLimit. These are model force
// limits, never pedal percentages. Models with a different envelope may leave
// Available false instead of advertising an inaccurate shape. Optional axle
// dynamics retain an exact frozen model for the non-elliptic force shape.
type TyreEnvelope struct {
	axles      axleState // optional frozen axle model, excluded from JSON
	Available  bool      `json:"available"`
	Lateral    float64   `json:"lateral_mps2"`
	Capacity   float64   `json:"capacity_mps2"`
	Resistance float64   `json:"resistance_mps2"`
	Drive      float64   `json:"drive_available_mps2"`
	Brake      float64   `json:"brake_available_mps2"`
	DriveGrip  float64   `json:"drive_grip_mps2"`
	Power      float64   `json:"power_capacity_mps2"`
	DriveScale float64   `json:"drive_scale"`
	BrakeScale float64   `json:"brake_scale"`
	BrakeLimit float64   `json:"brake_limit_mps2"`
}

// TyreForces is a sample of the model force balance. Combined utilization is
// not clamped: tiny numerical residuals remain visible rather than concealed.
// Limit describes a currently active bound; "none" includes constraints imposed
// by another segment of the profile. Phase has a 0.05 m/s² coast dead band.
type TyreForces struct {
	TyreEnvelope
	Longitudinal float64 `json:"longitudinal_mps2"`
	Utilization  float64 `json:"utilization"`
	Phase        string  `json:"phase"`
	Limit        string  `json:"limit"`
}

// Forces resolves a net acceleration against this envelope. speedCap is the
// active endpoint cap or vehicle maximum. No acceleration/speed is changed.
func (e TyreEnvelope) Forces(acceleration, speed, speedCap float64) TyreForces {
	f := TyreForces{TyreEnvelope: e, Phase: "unknown", Limit: "unknown"}
	if !e.Available || e.Capacity <= 0 {
		return f
	}
	f.Longitudinal = acceleration + e.Resistance
	f.Utilization = math.Hypot(f.Longitudinal, e.Lateral) / e.Capacity
	if e.axles.enabled {
		front, rear := e.axles.capacities(f.Longitudinal)
		f.Capacity = front + rear
		f.Utilization = e.axles.utilization(f.Longitudinal, e.Lateral)
	}
	f.Phase, f.Limit = "coast", "none"
	if f.Longitudinal > .05 {
		f.Phase = "drive"
	} else if f.Longitudinal < -.05 {
		f.Phase = "brake"
	}
	brakeGrip := math.Sqrt(math.Max(0, e.Capacity*e.Capacity-e.Lateral*e.Lateral)) * e.BrakeScale
	if e.axles.enabled {
		brakeGrip = e.axles.bound(e.Lateral, -1)
	}
	const tolerance = .03 // active force-bound proximity in m/s²
	switch {
	case speedCap >= 0 && speed >= speedCap-1e-4:
		f.Limit = "speed_cap"
	case f.Phase == "brake" && -f.Longitudinal >= e.Brake-tolerance:
		if e.BrakeLimit < brakeGrip-tolerance {
			f.Limit = "brake"
		} else {
			f.Limit = "grip"
		}
	case f.Phase == "drive" && f.Longitudinal >= e.Drive-tolerance:
		if e.Power < e.DriveGrip-tolerance {
			f.Limit = "power"
		} else {
			f.Limit = "grip"
		}
	case f.Utilization >= .995: // within 0.5% of the combined/axle grip boundary
		f.Limit = "grip"
	}
	return f
}

// LongitudinalBounds returns the positive drive and braking capabilities for a
// hypothetical lateral specific force at this sample's normal load and speed.
func (e TyreEnvelope) LongitudinalBounds(lateral float64) (drive, brake float64) {
	if !e.Available {
		return 0, 0
	}
	if e.axles.enabled {
		return math.Min(e.Power, e.axles.bound(lateral, 1)), math.Min(e.BrakeLimit, e.axles.bound(lateral, -1))
	}
	if math.Abs(lateral) > e.Capacity {
		return 0, 0
	}
	residual := math.Sqrt(math.Max(0, e.Capacity*e.Capacity-lateral*lateral))
	return math.Min(e.Power, residual*e.DriveScale), math.Min(e.BrakeLimit, residual*e.BrakeScale)
}
