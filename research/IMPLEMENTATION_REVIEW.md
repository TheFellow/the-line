# Independent implementation review

Reviewed 2026-09-09 against [the pre-implementation critique](CRITIQUE.md). Scope: `pkg/track`, `pkg/vehicle`, `pkg/solver`, their tests, and independent probes through public APIs. Rendering and GUI interaction are reviewed separately by the root agent.

Outcome: the consequential issues found during review have been corrected. The implementation is suitable for the documented interactive, quasi-static planning scope, with a measured 2% discretization tolerance on the supplied fixtures. This is not a claim of real-vehicle accuracy or globally optimal trajectories.

## Consequential findings

1. **Circular clearance requires distance to the actual side edges.** Shrinking each sampled cross-section by a vehicle radius only gives an offset bound. It does not guarantee that radius clears sloping/tapered longitudinal edges. The solver agent added signed perpendicular side-edge distance checks for candidate endpoints; the signed distance is linear along each path segment in its convex cell. Independent verification measures exact distances from every path segment to every road side segment, including neighboring cells. Open entry/exit cross-sections intentionally do not act as walls.
2. **Coarse candidate evaluation must not reject useful candidates for a finer force residual.** Early default preset solves could fail or discard candidates because a denser post-check detected an interior force extremum missed in the profile passes. The corrected solver increases the affected segment's force sampling and recomputes the profile. Independent final residual checks pass.
3. **Search discretization sensitivity must be measured on optimized results too.** Baseline convergence alone did not detect substantial changes in rally and esses optimized times. An early rally probe produced 22.721889 s at 3 m versus 20.892259 s at 1.5 m, an 8.757% difference relative to the fine result. The corrected search uses distance-based supports and smooth bounds, tries all scheduled decreasing amplitudes, and reevaluates a bounded shortlist on a road sampled at most every 0.5 m. It retains the best verified candidate or the verified baseline. Keeping several candidates avoids losing a useful earlier line when the most optimistic coarse winner performs poorly under dense evaluation.

The original refinement test became ineffective when both nominal search spacings exported the same dense baseline. That test now explicitly compares 0.5 m and 0.25 m baseline sampling. Optimized-path checks also refine the same chosen offsets, rather than comparing two separately optimized local solutions and calling their difference integration error. Search spacing can still change which local solution is discovered; this is distinct from the measured time error on a fixed line.

## What checks out

The vehicle model implements the signed constant-bank circle force balance from the critique, including the curvature contribution to normal load. Its fixed torque split respects both static axle capacities. Gravity and aerodynamic resistance act on net acceleration separately from the tyre friction circle. Spatial speed is projected horizontally for planar curvature; actual candidate segment grade enters gravity. The combined bank/grade model is explicitly a decoupled approximation, not full 3D dynamics.

The solver retains the original road stations, inserts crossings of the authoritative triangle diagonal, and adjusts those crossing heights to the road surface. It integrates actual 3D segment lengths. Its endpoint fields are expressly speed caps, and it records both baseline realized endpoint speeds. A feasible baseline is retained, candidates must improve the same objective, and the finite heuristic makes no global-optimality claim. A constant-envelope test model demonstrates that the dynamics interface can be replaced without changing the solver.

## Independent verification

`internal/verification/trajectory_test.go` imports only public packages. It does not reuse the solver's private geometry, interpolation or envelope-checking helpers. Its matrix solves all five road presets with all three vehicle presets, using one full search iteration at the default 3 m spacing. Full default-budget search and refinement are checked separately by the solver's own tests and review probes.

Each matrix result is checked for:

- no slower time than its identical-cap centreline baseline;
- realized speed within both requested endpoint caps;
- exact planar distance between every path segment and every road side segment, enforcing the declared circular clearance;
- triangle containment and barycentric surface height at every segment endpoint and midpoint;
- finite output positions, speeds, curvature, acceleration and times;
- exported acceleration and time consistent with segment lengths and squared-speed kinematics;
- 129 samples per segment, reconstructing bank from road station anchors and independently resolving longitudinal/lateral tyre forces and static axle limits.

The settled implementation's final matrix outcome is recorded below. Its independent force tolerance is 0.0005 m/s². This calculation uses the actual outgoing surface; conservative adjacent-surface handling can only make the production solver stricter at transitions. These are synthetic-model consistency checks, not calibration against a physical vehicle.

## Final validation

The default four-sweep search, with at most 16 shortlisted candidates for final verification, produced these results. All exported trajectories use 0.5 m verification sampling. The last column reevaluates the selected path at 0.25 m.

| Preset | Verified time (s) | Centreline time (s) | Improvement | Same-path refinement difference |
| --- | ---: | ---: | ---: | ---: |
| Hairpin | 12.8881 | 13.3835 | 3.70% | 1.222% |
| Esses | 15.0346 | 17.3632 | 13.41% | 0.958% |
| Compound | 14.1882 | 15.1315 | 6.23% | 1.038% |
| Banked | 10.2551 | 10.6991 | 4.15% | 0.370% |
| Rally | 23.1780 | 24.0995 | 3.82% | 0.574% |

The original critique proposed an initial 1% target. Hairpin and compound exceed it, so the accepted tolerance is explicitly 2%; no claim that every preset achieved 1% remains. Baseline refinement differences on hairpin, banked and rally are 0.063%, 0.088% and 0.074%, respectively. These figures bound the observed fixture differences, not all possible user-created roads.

Final command: `go test ./internal/verification ./pkg/solver -count=1 -v` passed. All 15 independent matrix cases passed in 16.23 s, with maximum sampled force excess 0.000000976 m/s². Every tested trajectory cleared the actual side edges and lay on its authoritative road triangles. The solver suite passed constant-acceleration, launch/stop, restrictive-cap, default improvement, deterministic replay, circular curvature, banking symmetry, refinement, invalid-input and braking-before-surface-transition checks; the table above was reproduced in that run.

`go test ./internal/verification ./pkg/track ./pkg/vehicle -count=1 -v` also passed after adding the independent triangle-height checks. Track and vehicle checks cover geometry rejection, persistence, signed banking, drag/grade, torque distribution and power limits.

The solver requires a feasible centreline baseline and may reject a scene even if a different path could make it feasible. It also deliberately rejects stationary cross-slope infeasibility because its speed representation assumes feasible intervals beginning at zero. Remaining model limits are fixed axle loads, no steering-rate/tyre-slip dynamics, no suspension or crest unloading, and an approximate combination of bank and grade. The circular clearance model does not certify a rectangular vehicle's swept body. These limits are consistent with the stated first-slice scope.
