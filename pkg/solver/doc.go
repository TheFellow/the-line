// Package solver estimates travel time and a useful racing line on open roads.
//
// Solve searches smooth lateral offsets over the entire corner sequence. It uses
// a finite deterministic coordinate search, not a globally optimal control
// solver. Endpoint offsets and headings are free. Entry and exit speeds are
// upper bounds: backward braking can reduce the actual arrival speed. Result
// reports the baseline's realized endpoints; the optimized endpoints are the
// first and last Nodes.
//
// # Geometry and refinement
//
// Options.Spacing selects the search road. The best accepted search incumbents
// are compared again on a road sampled at min(Spacing, 0.5) metres. Result.Spacing
// reports this final resolution, SearchSpacing the provisional search resolution,
// and CoarseDuration the lowest provisional time. The best feasible refined line
// survives, with the refined centreline available as a fallback. A natural cubic
// interpolation of bounded lateral controls supplies the refined geometry before
// any final forces, speeds, or times are computed; output is never smoothed after
// validation. Tests measure fixed-line 0.5-to-0.25-metre changes against a declared
// 2% tolerance. These convergence measurements are not physical calibration.
//
// The road's triangle mesh is authoritative. Paths include a vertex where they
// cross each cell diagonal, so banked surface heights match the renderer. Signed
// plan curvature is estimated at road stations and interpolated at triangle
// crossings. Distances are three-dimensional; grades come from candidate segments.
// Road widths and clearance are horizontal. Segment clearance protects a circle
// of half vehicle width plus Margin against the road sides, not the swept outline
// of a steered rectangular vehicle. Open entrance and exit edges are not obstacles.
//
// # Dynamics
//
// The supplied vehicle envelope defines the quasi-static physical approximation.
// Speed profiles use bounded forward/backward passes, safeguarded force roots,
// and adaptive envelope sampling. The final constant-acceleration segments are
// checked at 65 points each. Adjacent grip minima are conservative near surface
// transitions. Cases that slide at rest are rejected because this implementation
// represents speed upper bounds, not disconnected admissible speed intervals.
// Two stopped nodes spanning positive distance produce a resolution error rather
// than a manufactured finite travel time. Steering rate, transient load transfer,
// crest unloading, jumps, and drifting require richer models.
package solver
