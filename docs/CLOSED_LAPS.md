# Closed laps

Set `"closed": true` on a version 2 scene, or choose the fictional `club-loop`
preset. Store each control point once: the closing span runs from the last point
back to the first. At least three distinct controls are required. Width, bank,
elevation, surfaces and kerbs follow the same rules as open roads, including the
closing span. Entry and exit speed caps remain serialized for switching back to
an open sequence but do not constrain a steady-state lap.

The centreline is a periodic C2 cubic in chord length. Bank and widths retain
bounded C1 interpolation. Sampled roads and exported trajectories include one
repeated endpoint at the full lap station/time. Position, curvature, speed,
offset and force state match exactly across that seam. The endpoint force state
is the outgoing state of the next lap, rather than an open-road stopping state.

Speed propagation is cyclic: each forward/backward pass identifies the duplicated
seam speed and propagates reductions around the loop until convergence. The same
40-pass bound and dense segment-force verification apply. Search bumps wrap at
the seam, and coarse offsets refine through a periodic latent cubic. Supplied
and manual offsets must have identical first and final values. Geometry and
manual editors display unique handles; the final manual offset repeats the first.

`Result.At` and `AtStation` still clamp to a single lap for chart analysis and
exports. `LapAt(t)` and `CenterLapAt(t)` wrap elapsed time independently by each
trajectory's own lap duration. A centreline ghost therefore keeps circulating
when the faster line crosses its finish; it does not wait at the seam. The chart's
same-station delta compares the two one-lap profiles, not cumulative race gaps.
Pinned references use their own trajectory duration. Road digests distinguish
open geometry from closed geometry even when control points match.

The numerical validation includes an analytical constant-speed lap, periodic
spline derivative continuity and circle curvature, exact seam states, periodic
manual handles, control-start rotation, refinement, all three default cars, and
an independently checked ice transition at the seam. With one search sweep,
Club Loop's GT line takes 19.617771 s against a 20.705367 s centreline; reevaluation
at 0.25 m takes 19.616441 s. Rotating the control start changes the baseline by
less than 1e-7 s; sampled heuristic solutions vary by 0.063%, below the declared
1% refinement tolerance. This is bounded heuristic search, not a proof of a
unique global optimum or exact optimizer invariance. The synthetic vehicles and
quasi-static force limits retain their existing limitations.
