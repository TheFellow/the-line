# Independent pre-implementation critique

Reviewed 2026-09-09 against `research/README.md`, before implementation. This review supports the proposed first slice **after the constraints below are made explicit**. It is a design review, not evidence that this application already works. No implementation was written for this review.

## What the sources support

The bibliography is relevant. [Kapania, Subosits and Gerdes](https://arxiv.org/abs/1902.00606) really do alternate a forward/backward speed computation and a constrained path update, and explicitly disclaim both convergence and global optimality. Their path update uses vehicle dynamics constraints and convex optimization; coarse offset coordinate search is a new, simpler heuristic and does not inherit their experimental validation or computational results.

[TUM's global optimization repository](https://github.com/TUMFTM/global_racetrajectory_optimization) distinguishes shortest-path, minimum-curvature and minimum-time approaches. That supports evaluating candidate travel time instead of claiming minimum curvature solves minimum time. It does not establish the accuracy of this project's discretization.

[Rowold et al.](https://arxiv.org/abs/2304.10954) use a quasi-steady point-mass model with gg diagrams and three-dimensional road effects. Their work supports the general abstraction, but their results do not validate an independently simplified bank/grade formula. [Ögretmen et al.](https://arxiv.org/abs/2403.18643) study a substantially richer sampling planner and online racing-line generation; keep this as future context. The README abbreviates that paper's title; its full title is “Sampling-Based Motion Planning with Online Racing Line Generation for Autonomous Driving on Three-Dimensional Race Tracks.”

The publisher DOI for Heilmeier et al. did not open through this review's browser. I therefore do not independently certify detailed claims about that article from its full text. The linked repository and other primary sources suffice for the architectural proposal. Source links and publication experiments must remain separate from this application's future measured acceptance results.

## Required numerical decisions

### 1. Freeze the coordinate and bank conventions

Use right-handed world coordinates, z up, positive planar curvature left, and positive bank raising the left edge. Freeze this convention in serialized data and tooltips before fixtures are written; “whichever convention” is too loose for interchange.

The following is an independently derived **flat, constant-bank circular-path oracle**, not a transcription from a paper. Let `beta` be bank, `kappa` signed planar curvature, `v` horizontal path speed, and `g` gravity. Resolve acceleration minus gravity into the road lateral and normal axes:

```text
lateral tyre force / mass = v² kappa cos(beta) + g sin(beta)
normal load / mass        = g cos(beta) - v² kappa sin(beta)
```

Require positive normal load and the absolute lateral demand to fit `mu * normal_load`. With positive curvature, positive bank increases demand and reduces load. Negating both bank and curvature must preserve capability. Banking also consumes lateral grip on a straight; a signed additive “corner-speed bonus” misses this. A simplified model that omits the curvature contribution to normal load must say so explicitly and quantify disagreement with this oracle.

On sufficiently steep banking, feasibility need not be a speed interval starting at zero: a stationary vehicle can slide. A solver that only stores upper speed caps should reject combinations with `abs(tan(beta)) > mu`, or explicitly implement lower as well as upper admissible speeds. Do not obtain imaginary square roots and then clamp them into feasibility.

For elevation, distinguish reference station, horizontal path distance and actual three-dimensional candidate distance. If `s` is actual path distance, `dz/ds = sin(grade_angle)`, not the angle itself. Use actual candidate distance in time integration, and actual candidate grade in gravity. A changing lateral offset on a banked road changes elevation too. Do not leave the car at centreline z while drawing raised road edges. General 3D curvature and banking cannot be obtained by substituting three-dimensional speed into the flat oracle without a documented approximation.

**Acceptance:** analytic flat-circle check; bank/curvature reflection symmetry; adverse bank worse than flat for a left circle; straight cross-slope consumes grip; zero bank recovers the flat envelope; uphill/downhill constant-grade force signs; candidate positions lie on the same surface rendered in both views.

### 2. Make the road and path geometry authoritative

Station offset bounds alone do not guarantee road containment, even if offset controls are smooth. A narrow hairpin can produce intersecting normals or folded ribbon cells. In a planar smooth ribbon, `1 - kappa_reference * offset` approaching zero is a local singularity warning; it is not a complete global intersection test.

Using discrete ribbon triangles as the authoritative road is a defensible first decision. Validate nondegenerate, consistently oriented cells and reject unsupported foldovers/self-intersections. Check the entire candidate segment against its intended road cells, not only endpoints or arbitrary nearby overlapping road. Interpolate candidate height from the actual surface. Render and export that validated path without subsequently adding an unchecked smoothing spline.

Define whether width and clearance are horizontal or measured across the banked surface. A half-vehicle-width centre offset margin protects only a simplified footprint. For a physical body, heading relative to the road matters: a rectangular vehicle can put a front corner outside the road while its centre fits. Either check its swept footprint or explicitly call clearance a circular/point-mass safety margin and avoid asserting full-body containment. Continuity of the discrete line does not imply steerability; bound excessive heading jumps/curvature and disclose the lack of steering-rate dynamics.

**Acceptance:** adversarial narrow hairpin, sharply changing width, high-bank segment, short segment and spline overshoot cases; continuous segment containment against the authoritative ribbon with the declared margin; rejection of degenerate/folded geometry; distance/time and feasibility stability when station spacing is halved.

### 3. Specify boundary conditions as caps or exact states

Entry/exit **speed caps** are acceptable for an open-sequence design. Name them caps in schema, CLI, GUI and output. A backward braking pass may reduce actual entry speed below its cap; that means the computed run assumes a slower arrival. It does not solve an already moving vehicle's exact initial state.

Report requested caps and actual endpoint speeds for both optimized and baseline paths. Identical caps do not ensure identical realized speeds. A time comparison is fair for the same cap-constrained problem, but is not necessarily an equal-state comparison. Define whether entry/exit lateral offsets and headings are fixed or free too; otherwise the optimizer may exploit its endpoint freedom.

If exact initial/final speeds are later supported, independently verify them after convergence. An unreachable high terminal speed is as relevant as insufficient braking for an exact initial speed. Do not silently lower either exact boundary. Keep open-loop animation explicitly separate from periodic lap physics.

**Acceptance:** a high entry cap is visibly reduced before an immediate tight corner; a restrictive exit cap causes upstream braking; baseline/candidate carry matching requested conditions and their actual endpoint states; any exact-state mode rejects incompatible boundaries.

### 4. Verify segment dynamics, not just station speed ceilings

`v_next² = v² + 2 a ds` assumes the represented acceleration over the segment. With speed-dependent power, drag and grip, evaluating force at one endpoint can overestimate feasibility. Use bounded iteration or conservative substeps, an explicit convergence tolerance and iteration limit, then independently recompute residuals from the final trajectory.

The friction circle applies to **tyre forces**, not net acceleration after gravity and drag. Check longitudinal tyre demand inferred from net acceleration plus resistance and slope, together with lateral tyre demand, against the chosen envelope. Drag can assist braking without using tyre grip. Braking capability must not be forced positive on an extreme descent that the model cannot brake down.

Power-limited acceleration is `P / (m v)`, not `P / m`; define watts, mass in kg and speed in m/s. At zero speed, finite tyre/drive force must bound launch force without division by zero. Define aerodynamic density and `CdA` units if computing `0.5 rho CdA v²`.

Place a station at every surface boundary and treat grip as a categorical segment property. Minimum adjacent grip is conservative but spacing-dependent; record that approximation and check refinement. Do not interpolate surface identities or smear low grip forward/backward into a falsely precise result.

The time formula `2 ds / (v_i + v_next)` is appropriate for the represented constant-acceleration segment. Two zero endpoint speeds over positive distance require resolution of acceleration and braking inside that interval; they do not by themselves prove the physical problem impossible. Subdivide or report insufficient discretization, never manufacture finite time with an epsilon denominator.

**Acceptance:** independent straight constant-acceleration/braking oracle; finite launch and stop; downhill braking sign; conservative high-to-low-grip transition; final per-segment force residual check; bounded convergence failure; refinement changes representative times by a documented tolerance (initial target: below 1%).

### 5. Give the vehicle abstraction a force contract

Use a consumer-owned envelope interface whose input contains speed, grip and road state, and whose output defines available positive drive force, braking force and lateral feasibility with units. Keep rendering, vehicle names and serialization out of numerical force evaluation. Fictional parameter sets are fine if presented that way.

Static axle loads can distinguish drivetrains, but a fixed torque split is not the same as an optimally variable AWD split. If front torque fraction is `q` and axle longitudinal capacities after lateral demand are `Rf` and `Rr`, total drive force must satisfy `q F <= Rf` and `(1-q) F <= Rr`; for `0 < q < 1` this gives `F <= min(Rf/q, Rr/(1-q))`. Treat the pure FWD/RWD endpoints separately. Summing capacities assumes the split can adapt; taking a weighted average of axle loads generally represents neither case. State how lateral force is allocated to axles and how brake force is limited.

Do not require FWD and RWD to differ with equal static axle loads and no transfer: this approximation predicts equality there. Compare unequal axle loads in a traction-limited case. Higher power need not help an entirely grip-limited route. Neither test should assert artificial changes simply because a parameter changed.

**Acceptance:** equal-load FWD/RWD equality; unequal-load launch difference; fixed AWD split constrained by its weaker allocated axle; finite power/traction crossover; braking stays within the declared shared envelope; replacement envelope usable without modifying geometry/search.

## Release evidence and search claims

Preserving the feasible centreline fallback and accepting only reduced measured time guarantees non-regression only under the same numerical objective, boundaries and feasibility rules. It does not establish a local mathematical optimum, still less a global optimum. Label results “heuristically optimized estimates.” Reject infeasible candidates before comparing times. Record objective improvement, candidate count, termination reason and resolution; impose a finite search budget for editing responsiveness.

Test corner sequences as sequences: include one fixture where optimizing a first corner alone harms the next corner's entry. Require improvement on selected fixtures, not every scene (a straight may already be optimal). Verify with a denser independent evaluation so the search cannot win by exploiting coarse curvature samples. Report centreline and optimized times, spacing and force/containment residuals in measured validation documentation. GUI captures prove rendering and interaction, not physics. The acceptance list in the research README remains a checklist until actual outcomes are recorded.
