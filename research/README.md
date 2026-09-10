# Racing-line research

Collected 2026-09-09 before implementation. These are primary sources and original design notes, not copied implementations. Links document provenance; external code is not vendored.

## Prior art

| Source | Relevance | Caution |
| --- | --- | --- |
| [TUM global racetrajectory optimization](https://github.com/TUMFTM/global_racetrajectory_optimization) | Compares shortest path, minimum curvature, and minimum time; track reference points, normal offsets, widths, and spatial friction maps are useful representations. | Minimum curvature is not minimum time. Python/CasADi machinery is not a Go dependency for this project. |
| [Heilmeier et al., Minimum curvature trajectory planning and control](https://doi.org/10.1080/00423114.2019.1631455) | Iterative curvature optimization and forward/backward speed planning with vehicle acceleration limits. | Linearized curvature needs iteration; an attractive spline alone is not validation. |
| [Kapania, Subosits, Gerdes, sequential two-step algorithm](https://arxiv.org/abs/1902.00606) | Alternate speed-profile computation and path updates, evaluate the resulting time, repeat. | Authors explicitly do not guarantee global optimality. Our simpler numerical search must make the same distinction. |
| [Rowold et al., Online Time-Optimal Trajectory Planning on Three-Dimensional Race Tracks](https://arxiv.org/abs/2304.10954), [implementation](https://github.com/TUMRT/online_3D_racing_line_planning) | Point-mass/gg-envelope methods with road geometry; banking and elevation have physical effects, not just visual ones. | Full spatial road-frame dynamics contain more than a bank correction to friction. Initial implementation must label its quasi-static approximation. |
| [Ögretmen et al., Sampling-Based Motion Planning on Three-Dimensional Race Tracks](https://arxiv.org/abs/2403.18643) | Useful future direction for local planning, changing conditions, and interactions. | Multi-vehicle online control is beyond this first application. |
| [TUM trajectory planning helpers](https://github.com/TUMFTM/trajectory_planning_helpers) | Geometry, velocity profiles, and acceleration tools provide independent conceptual reference points. | License must be checked before any copying; implementation here will be original. |
| [OpenLAP](https://github.com/mc12027/OpenLAP-Lap-Time-Simulator), [vehicle construction](https://github.com/mc12027/OpenLAP-Lap-Time-Simulator/blob/master/OpenVEHICLE.m) | Separation of vehicle data, track data, and solver; friction-envelope limits combine with power and aerodynamic resistance. | GPL-3.0 source is studied conceptually, not translated or copied. Defaults here will be illustrative, not measured vehicle data. |
| [Fastest-lap](https://github.com/juanmanzanero/fastest-lap) | Higher-fidelity vehicle optimal-control reference and future comparison candidate. | A full nonlinear vehicle simulator is a different fidelity tier from a quick interactive planning tool. |

## Local conventions and engine decision

Adjacent `fluid/go.mod` uses `github.com/hajimehoshi/ebiten/v2` v2.8.7, `main/` for application wiring, and `pkg/fluid` for CPU simulation. `go-modular-monolith` separates `main/cli`, `main/gui`, and public packages; its CLI uses urfave/cli/v3, while smaller projects use standard-library facilities. `ai-rigidbody` is Rust with wgpu/egui; `ai-4d` is Godot/C++. Ebitengine is the existing Go game engine and the least disruptive match. Pin the known adjacent version initially. Use `main/cli` and `main/gui`, domain packages under `pkg/`, standard-library flags and JSON, explicit errors and deterministic tests.

Ebitengine documentation: [official site](https://ebitengine.org/), [Game API](https://pkg.go.dev/github.com/hajimehoshi/ebiten/v2#Game). Keep graphics initialization out of the CLI and model. A software renderer shared with the GUI permits PNG/GIF generation without a display; the GUI uploads its road image and animates the car using the same trajectory clock. The 3D view is a projected elevated and banked road mesh, not a claim of a separate high-fidelity physics engine.

## Proposed first complete slice (subject to independent critique)

1. Versioned JSON scene: named ordered control points with x/y/z in metres, road width, signed bank in degrees, and surface identity. Open corner complexes first, with explicit entry and exit speed conditions. Smooth centreline interpolation creates a road ribbon, samples tangent/normal and road attributes; retain banking in plan view. Presets cover a hairpin, esses, a compound corner, a banked sweep, and a rally surface transition.
2. Vehicle data: mass, power, braking limit, maximum speed, tyre grip scale, drag area, front weight fraction and driven axle or front drive fraction. A small consumer-owned dynamics interface permits alternative acceleration envelopes. Defaults are fictional illustrative road, GT, and rally cars; do not imply manufacturer calibration.
3. Solver: keep a lateral offset per road station, bounded by road half-width minus vehicle clearance. Use smooth coarse offset controls or multiscale perturbations to avoid zigzag solutions. Seed with centreline and a curvature-smoothed candidate. Recompute path curvature, segment lengths and vehicle-specific speed profile for every candidate. Accept only measured time improvement; preserve the centreline fallback. Search across an entire sequence so exits can trade against the next entry.
4. Speed profile: lateral demand reduces available longitudinal tyre force (friction circle). Forward acceleration and backward braking use v_next² = v² + 2 a ds and power/mass, driven-wheel load, drag, slope and local grip. Include bank sign with signed curvature so adverse banking reduces capability. Use conservative transition handling and iterate speed passes if forces depend on speed. Integrate time as 2 ds/(v_i + v_next), with explicit infeasibility instead of NaN/Inf or infinite loops.
5. GUI: elegant dark instrument-panel design, large road canvas, speed-coloured optimal line, ghost centreline, moving vehicle and trail, speed/time telemetry, 2D/3D toggle. Editor must create, add/delete/drag control points, change width/bank/elevation/surface, choose vehicle, load/save files, and recompute after edits. Put edit logic in a headless controller package. Show computation/error state and preserve the last valid model on bad edits.
6. CLI: list presets, create/save a scene, validate, solve to JSON/CSV, render PNG, animate GIF. Use the same model, editor operations, renderer and animation state as the GUI. GUI automation mode renders frames and exits after a finite run for verification.

## Numerical conventions to resolve and test

World x/y form a right-handed plan, z is up. Positive signed curvature turns left. Define positive bank as left edge raised (surface z increases toward the left normal); this is adverse for a left turn. Document whichever consistent convention the implementation finally adopts. Road bank is a physical property independent of display mode. Grade comes from dz/ds. Surface changes must not be smoothed into optimistic grip ahead of braking zones.

Open sequences need stated speed boundary conditions. Entry speed cannot silently decrease when braking feasibility fails; distinguish a speed cap from an exact initial state. Looped animation of an open sequence visibly restarts at the start and is not a periodic lap solution. If closed circuits are added, use periodic geometry and cyclic speed convergence, not duplicated endpoints with open-boundary logic.

A simple static load split distinguishes FWD/RWD/AWD traction but does not reproduce transient weight transfer, slip angles, differential behaviour, load sensitivity or suspension. Constant friction on gravel does not simulate drifting or loose-surface tyre mechanics. Elevation-aware distances, slope and bank are worthwhile initial support; crest unloading, jumps and fully coupled 3D inertial terms require a richer dynamics model.

## Acceptance evidence

- JSON round trip; invalid numbers, unknown surface, tiny/duplicate geometry and unusable widths return useful errors.
- Straight-line acceleration/braking checks; circular curvature check; signed banking symmetry; lower-grip speed reduction and braking before a transition; higher power/changed drive distribution affects results.
- Every output station fits the road with vehicle margin, speeds/times finite, time monotonic, and combined force demand respects the implemented envelope. Inspect interpolated geometry too; station-only bounds do not justify arbitrary smoothing of the final line.
- Optimized result no slower than identical-boundary centreline baseline, representative presets measurably improve, repeated solves deterministic. Results are locally optimized estimates, not a mathematical proof of globally shortest time.
- Headless editor create/edit/save/load exercise, CLI workflow, PNG and animated GIF inspection in both views, GUI finite-frame capture and actual real-time execution. Record measured outcomes and limitations in repository documentation.

## Two-car racecraft

[RACECRAFT_IMPLEMENTATION.md](RACECRAFT_IMPLEMENTATION.md) records the implemented
tactical planner, continuous body-clearance argument, model boundaries and
validation evidence for the independent Claude CLI critique of PR #1.
The [preserved critique](CLAUDE_RACECRAFT_CRITIQUE.md) and
[finding-by-finding response](RACECRAFT_CRITIQUE_RESPONSE.md) document the review,
fixes, additional regression tests and remaining model limits.
