# Optional axle dynamics

The vehicle remains an illustrative quasi-static model, not calibrated telemetry from a real car. Existing road/GT/rally presets have all new effects disabled and retain their previous force arithmetic exactly. The separate [Aero Study](../examples/vehicles/aero-study.json) configuration demonstrates the additions without changing those presets.

The setup workbench's second page exposes these optional JSON fields:

| Field | Units and behavior |
| --- | --- |
| `front_brake` | Front brake-force fraction in (0,1]. Zero retains the legacy ideal distribution between available axle residuals; it does **not** mean rear-only brakes. |
| `lift_area` | Positive downforce coefficient × area, m²; zero disables aero load. |
| `aero_balance` | Fraction of downforce on the front axle, [0,1]. |
| `wheelbase` | Metres; set before enabling CG height, at least 0.5 m when transfer is active. |
| `cg_height` | Metres above the road contact plane; zero disables longitudinal transfer. |
| `load_sensitivity` | Exponent in [0,0.5]; zero preserves load-independent friction. |

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

With no transfer, the axle residuals are explicit. With transfer and zero load sensitivity, fixed-split axle bounds use stable quadratic roots. The remaining concave load-sensitive capacities produce convex force constraints; a bracketed Newton solve uses at most 40 steps, falling back to bisection whenever a proposed step leaves its bracket. It returns a verified feasible lower bound. Force-bound termination uses 1e-10 m/s² residual and a conservative 1e-9 m/s² inset. No unconstrained fixed-point loop is used.

The force widget queries this same frozen axle model. Its shape can depart from an ellipse because the axle loads vary with longitudinal force. Exported capacity is the sum of the two capacities at the **actual** tyre demand; utilization is the maximum axle-circle demand/capacity ratio, so a single overloaded axle cannot be concealed by unused capacity on the other. Legacy presets retain their previous combined-circle utilization. These remain model tyre forces, never pedal positions.

Analytical tests cover FWD/RWD launch transfer, fixed brake bias, downforce cornering, separate drag effects and load sensitivity. Independent trajectory verification reconstructs both axle forces from exported geometry and kinematics, without calling the production envelope. The geometry, banking, heuristic search and numerical-refinement limitations in [the implementation review](../research/IMPLEMENTATION_REVIEW.md) continue to apply.
