# Optional axle dynamics

The vehicle remains an illustrative quasi-static model, not calibrated telemetry from a real car. Existing road/GT/rally presets have all new effects disabled and retain their previous force arithmetic exactly. The separate [Aero Study](../examples/vehicles/aero-study.json) configuration demonstrates the additions without changing those presets.

The setup workbench's second page exposes these optional JSON fields:

| Field | Units and behavior |
| --- | --- |
| `front_brake` | Front brake-force fraction in (0,1]. Zero retains the legacy ideal distribution between available axle residuals; it does **not** mean rear-only brakes. |
| `lift_area` | Positive downforce coefficient × area, m²; zero disables aero load. |
| `aero_balance` | Fraction of downforce on the front axle, [0,1]. Zero means **all rear**, one means all front. |
| `wheelbase` | Metres; set before enabling CG height, at least 0.5 m when transfer is active. |
| `cg_height` | Metres above the road contact plane; zero disables longitudinal transfer. |
| `load_sensitivity` | Exponent in [0,0.5]; zero preserves load-independent friction. |

When the setup panel first enables downforce from zero, an unset (zero) aero balance is seeded to the static front weight fraction. This gives both axles a proportional cornering benefit; explicitly setting aero balance to zero afterward gives all-rear downforce. Loaded JSON keeps its exact specified balance.

The independent drag-area parameter controls drag. Adding downforce alone **does not lower a power-limited straight-line top speed** when drag is held fixed. The roadmap's proposed downforce/top-speed acceptance sentence assumed a coupling that the model does not contain. Increase drag separately to study that tradeoff; the illustrative aero configuration does so explicitly.

## Equations and conventions

All forces below are divided by vehicle mass, giving m/s². Let `w` be static front weight fraction, `d` front drive fraction, `b` front brake fraction, `mu` surface grip × vehicle grip, and `Y` signed road-frame lateral tyre demand. The existing signed-bank/grade equations supply ground normal load `N` and `Y`. Aero normal load is `A = 0.5 rho lift_area v² / mass`, split by `aero_balance`.

At longitudinal tyre demand `X` (net acceleration plus drag and grade resistance), the two axle loads are:

```
Nf = w N + aero_balance A - X cg_height / wheelbase
Nr = (1-w) N + (1-aero_balance) A + X cg_height / wheelbase
```

This moment balance treats the tyre contact plane as the pitch-force lever arm and the aerodynamic drag resultant as acting through the CG. A grade's required holding tyre force therefore transfers load even at constant speed. Aero balance specifies the normal force's pitch allocation. Vertical curvature, suspension response, pitch transients, lateral load transfer and wheel lift are omitted; nonpositive axle loads are rejected.

Lateral force follows the static CG allocation: `Yf = w Y`, `Yr = (1-w) Y`. This is the steady bicycle model's yaw-moment balance, not allocation in proportion to whichever axle gained downforce. Unbalanced aero can therefore make one axle reach its lateral limit first.

Per-axle capacity, around its static level-road reference load, is:

```
Cf = mu Nf (Nf / (g w))^(-load_sensitivity)
Cr = mu Nr (Nr / (g (1-w)))^(-load_sensitivity)
```

Drive must satisfy `hypot(d X, Yf) <= Cf` and `hypot((1-d) X, Yr) <= Cr`, as well as the power bound. Fixed brake bias substitutes `b`; legacy ideal braking instead sums the two residual longitudinal capacities. Both braking modes obey the separate brake capability cap.

The solver retains its explicit feasible-interval assumption: lateral demand must be feasible at **zero longitudinal tyre force** before either signed bound is searched. Situations requiring a particular nonzero acceleration to make lateral demand feasible are intentionally excluded. This is conservative and allows the existing forward/backward profile solver to use one contiguous acceleration interval.

## Bounded solution and telemetry

With no transfer, the axle residuals are explicit. With transfer and zero load sensitivity, fixed-split axle bounds use stable quadratic roots. The remaining concave load-sensitive capacities produce convex force constraints; a bracketed Newton solve uses at most 40 steps, falling back to bisection whenever a proposed step leaves its bracket. It returns a verified feasible lower bound. Force-bound termination uses a 1e-10 m/s² residual. Finite constraint residuals on both sides of an axle’s lateral limit preserve Newton progress even when load transfer unloads that axle. Returned bounds are moved inward by floating-point ulps and checked against the actual tyre-circle predicate; exceptionally shallow boundaries use a bounded 48-step feasibility bisection for that rounding correction. The 40-step Newton cap retains a conservative feasible bound if the target is not reached. No unconstrained fixed-point loop is used.

A car with positive CG height requires a finite wheelbase of at least 0.5 m. `Validate` reports the explicit configuration error; low-level `Limits`, `FeasibleOn`, and `BoundsOn` return infeasibility even if a caller bypasses validation.

`Envelope.LateralUtilization` names lateral demand at zero longitudinal tyre force. `TyreForces.Utilization` names combined demand at the actual longitudinal force; its existing JSON field remains `utilization`.

The force widget queries this same frozen axle model. Its shape can depart from an ellipse because the axle loads vary with longitudinal force. Exported capacity is the sum of the two capacities at the **actual** tyre demand; utilization is the maximum axle-circle demand/capacity ratio, so a single overloaded axle cannot be concealed by unused capacity on the other. Legacy presets retain their previous combined-circle utilization. These remain model tyre forces, never pedal positions.

Analytical tests cover FWD/RWD launch transfer, fixed brake bias, balanced versus all-rear downforce, separate drag effects and load sensitivity. A deterministic grid independently reconstructs axle moment balance and tyre circles and bisects 8,000 signed force boundaries across drive splits, brake biases, aero imbalance and load sensitivity. Its acceptance threshold is an absolute boundary residual of 1e-10 m/s² and root error of 1e-9 m/s². The grid includes straight-line braking near axle lift-off and near-saturated lateral demand; it measures the active tyre-circle or positive-load boundary. The measured maxima were 2.66e-11 m/s² residual and 3.49e-11 m/s² root error. Another 4,000 no-transfer bounds check direct circle feasibility, including floating-point rounding. Independent trajectory verification reconstructs both axle forces from exported geometry and kinematics, without calling the production envelope. The geometry, banking, heuristic search and numerical-refinement limitations in [the implementation review](../research/IMPLEMENTATION_REVIEW.md) continue to apply.

## Measured acceptance

The implementation passed `go test ./pkg/vehicle` and the independent enhanced-axle checks for esses and the banked sweep. Those checks sample both the evaluated line and its centreline reference at 17 points per segment; the force excess tolerance is 0.0005 m/s². The final value-state implementation completed that two-road test in 13.04 s under concurrent development work.

Before/after banked/GT CLI exports were identical with the optional fields left at zero. Analytical inactive-field checks additionally compare floating-point bits for acceleration, braking and lateral utilization across all three unchanged presets. These tests establish compatibility of the existing force path, not calibration of the new effects.

The paged setup hit targets are checked for duplicates and overlap at 1440×900 and 1000×700. Both views and both setup pages were rendered without a graphics display; the Aero Study banked-sweep captures show a near-limit force point, legible fields and an unclipped road.

The richer envelope costs more CPU. A microbenchmark measured approximately 302 ns per linear-load call and 3245 ns per load-sensitive call, both with zero allocations. A full default-budget esses solve with longitudinal transfer and fixed brake bias took 24.23 s during concurrent development; ideal adaptive braking took 53.01 s, versus 8.00 s for the legacy model. Those full-run measurements predate the final allocation-free state change and are not frame-rate measurements. Use current-line evaluation or a reduced search budget for interactive experiments with the optional load-sensitive model; progressive solving remains cancellable. Full model search latency is a remaining performance limitation, not a physics shortcut hidden by these controls.
