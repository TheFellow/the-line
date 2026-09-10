# Next steps: from a corner viewer to a line-and-setup laboratory

Implementation status: see [the roadmap release record](../docs/ROADMAP_RELEASES.md) for delivered features, measured validation and unmet targets. The proposal below is preserved as the original review.

Reviewed 2026-09-10 against commit `77f5d7c` (camera and reference-analysis release). This document records what the studio does today, what a sim racer and a car enthusiast still cannot do with it, and a prioritized sequence of releases that close that gap. It is written to be consumed by the same iterative process that produced `docs/ENTHUSIAST_REVIEW.md`: each iteration has a motivation, a design that fits the existing Go structure, and acceptance evidence. Nothing below is implemented; measured numbers refer only to the current code.

The product goal used for every judgement: **let the user experiment with different lines and different car setups at the edge of adhesion, and see honestly where time is gained or lost.**

## 1. Where the application stands

### What works and is verified

- Versioned open-road scenes with width, bank, elevation and categorical surfaces; validated ribbon geometry; five fictional presets and three fictional vehicles.
- A quasi-static friction-circle vehicle envelope with signed bank, grade, drag, power, fixed static axle loads and fixed drive torque split (`pkg/vehicle`). Its flat constant-bank circle behaviour is verified against an independent oracle.
- A deterministic smooth-offset coordinate search with dense re-verification, retained centreline reference, shared road-station coordinate and station/time queries (`pkg/solver`). Fixture improvements run 3.7% to 13.4% over the centreline; same-path refinement error is within a declared 2%.
- A display-independent renderer with plan and elevated orthographic views, orbit/pan/zoom camera, speed-coloured line, same-time ghost, same-station speed chart with signed delta, and a transactional editor with undo/redo (`pkg/render`, `internal/editor`, `main/gui`).
- CLI parity for solve/render/animate, and real-input headless browser verification of 18 grouped scenarios in two display configurations.

This is a solid, honest foundation. The remaining work is not repair; it is building the experimental loop on top of it.

### Findings from this review

**F1. The road centreline is only C1, so curvature jumps at every control point.** `track.SampleRoad` uses chord-scaled cubic Hermite segments with Catmull-Rom style tangents. Position and tangent are continuous; curvature is not. The three-point curvature estimator in `solver/geometry.go` faithfully reports the jump, and the speed profile inherits it. Evidence from `bin/the-line solve --preset esses --vehicle gt --format csv`: the ten largest adjacent-node curvature jumps sit at stations 44.7, 89.9, 144.5, 187.3, 237.6 and 285.0 m, which are the control-point stations (chord positions 45.0, 89.7, 143.8, 185.0, 234.5, 281.7 m). The largest jump is 0.0142 1/m at 237.6 m where the car runs about 29 m/s; that is a step of roughly 12 m/s² (1.2 g) in lateral demand across a single 0.5 m segment. The result is the sawtooth speed trace visible in every esses screenshot (`artifacts/browser/plan-gt-comparison.png`). A sim racer reads a speed trace as a fingerprint of the corner; a trace with teeth at the designer's control points says the geometry, not the driving, shaped the profile. This also biases the optimizer, because a candidate can gain time by placing its own curvature against the base curvature step rather than by finding a better line. Fixing this is the precondition for everything else being believable.

**F2. Nothing exposes "the edge of adhesion."** The envelope already computes lateral utilization (`vehicle.Envelope.Utilization`), but the trajectory exports only speed, curvature and net longitudinal acceleration. The telemetry bar shows velocity, elapsed time, net acceleration and path distance. A user cannot see how much grip is used, where the car is grip-limited versus power-limited, or where braking begins and ends. The previous review correctly refused to display pedal percentages inferred from net acceleration. Tyre-force phases derived from the model's own force balance are a different matter and are available for free.

**F3. The vehicle cannot be tuned in the studio.** The vehicle button cycles three presets. Custom vehicle JSON works only through the CLI. The stated goal centres on setup experimentation, and today that requires editing files and re-running commands.

**F4. The only reference is the centreline.** A user cannot compare their previous edit, another car, or another setup at the same station. The renderer's comparison path is hard-wired to `Result.CenterNodes`.

**F5. The user cannot author a line.** The studio shows one optimizer line per scene. There is no way to say "I would brake later and apex later here; how much does that cost?" That question is the core of a sim racer's practice loop, and the profile evaluator that would answer it already exists as an unexported method (`evaluator.run`).

**F6. Solve latency blocks a tuning loop.** A default esses solve takes about 2.1 to 3.1 s. That is fine for a geometry edit and far too slow for dragging a grip or mass slider. There is no warm start, no cancellation, no progressive result.

**F7. Roads are open sequences of constant 12 m width without kerbs, asymmetric limits or laps.** These are documented limits. They matter for enthusiasts because real corner discussions revolve around kerbs, run-off and the compromise between one corner and the next around a lap.

**F8. Small presentation gaps.** Speed colours are relative per run, so two cars' lines are not colour-comparable. The chart has one channel (speed). Playback is top-down or elevated orthographic only; there is no chase or trackside perspective. None of these is urgent, but all shape the "awesome" feeling.

## 2. Principles for the next releases

1. **Show the limit, not just the line.** Every view should make it visible where grip is saturated and which force is saturating it.
2. **Let the user try things and get an honest answer fast.** Lines and setups are hypotheses; the studio should evaluate them in well under a second, then refine in the background.
3. **Compare anything with anything, always at the same road station.** The centreline is one reference among many.
4. **Stay honest about the model.** Every new readout must be derived from the verified trajectory and the vehicle envelope, be labelled with what it assumes, and never imply calibrated behaviour of a real car.
5. **Keep the Go structure.** Pure numerics in `pkg/solver` and `pkg/vehicle`, drawing in `pkg/render`, input translation in `main/gui`, edit history in `internal/editor`, commands in `internal/cli`. Focused files, standard-library tests, analytical oracles.

## 3. Iterations in recommended order

Each iteration is sized to be one release with its own validation record. Dependencies are noted; the order is chosen so that every release makes the studio more useful on its own.

### Iteration 1: Curvature-continuous roads

**Why first.** F1 undermines the speed chart, the delta readout and the optimizer's incentives. Everything later draws more attention to the speed trace, so it must be right.

**Design.**

- Replace the per-segment Hermite interpolation in `track.SampleRoad` with a C2 centreline through the same control points. Recommended: a natural cubic spline in x, y and z parameterized by cumulative chord length, then reparameterized by arc length as today. This keeps every control point as a station and keeps the code standard-library only. Clothoid fitting would give even nicer curvature ramps but is more work and can come later.
- Width and bank interpolation should also become at least C1 along station (cubic or smoothstep between control points) so that bank-induced lateral demand does not step either.
- Keep `Version: 1` scenes loading unchanged. Because geometry between control points changes slightly, treat this as a numerical change: rerun the fixture table and update `docs/VALIDATION.md` and `research/IMPLEMENTATION_REVIEW.md` figures. The fixtures themselves need no edits.
- Optionally compute analytic curvature from the spline for the centreline, and derive the offset path's curvature analytically from the reference curvature and the offset's first and second station derivatives. The three-point estimator can remain as an independent check in tests.

**Acceptance.**

- Curvature difference between adjacent 0.5 m nodes on all five presets falls below a declared threshold (target 5e-4 1/m at control points), and the largest jumps no longer coincide with control-point stations.
- On esses, the optimized speed trace has one local minimum per corner and no local extrema between apexes except at surface boundaries.
- Same-path 0.5 to 0.25 m refinement difference improves; the original 1% target should now be reachable on hairpin and compound and, if so, the declared tolerance is tightened.
- Ribbon fold and self-intersection validation still reject the adversarial cases in `pkg/track` tests; the independent matrix in `internal/verification` still passes.
- Render both views for every preset and visually confirm no visible kink at control points.

### Iteration 2: Instrument the edge of adhesion

**Why.** F2. This is the cheapest release with the biggest change in how the studio feels: the user sees the car working, not just moving.

**Design.**

- Extend `solver.Node` with per-node tyre-force channels computed by the same force balance the envelope uses: lateral tyre demand (m/s²), longitudinal tyre demand (net acceleration plus drag and grade resistance), total capacity, and combined utilization (sqrt of the squared demands over capacity, 0 to 1). Add `phase`: `drive`, `brake` or `coast` from the sign of longitudinal tyre demand with a small dead band, and a `limit` flag for whether the node is grip-limited, power-limited, speed-cap-limited or brake-limited. Export all of these in JSON and CSV; document them in `docs/TRAJECTORIES.md`.
- Add a live **friction-circle widget** in the telemetry bar: the current combined-demand dot inside the model's envelope, which is an ellipse when brake capability differs from drive capability, or a circle clipped by the power curve. Draw the reference car's dot as an outline when the ghost is enabled.
- Add a **line colour mode** control: speed (current), grip utilization, lateral g, longitudinal g. Use a fixed absolute scale per mode so two cars' lines are comparable, and print the scale legend beside the viewport.
- Add **road markers** derived from the trajectory: braking point (first `brake` node after a `drive` run), apex (local speed minimum), and throttle point (first sustained `drive` node after the apex). Draw them as small labelled ticks on the road and on the chart. Sim racers talk in exactly these terms.
- Add **chart channels**: a second row or overlay for utilization and the two g channels, toggleable, sharing the road-station axis and cursor with the speed chart.
- Label the section **Model tyre forces** and keep the existing "illustrative vehicle" and "quasi-static" wording next to it. Do not show throttle or brake percentages, slip, or tyre temperature.

**Acceptance.**

- For every node, recomputing `vehicle.Limits` at the node's speed, curvature, bank, grade and grip yields a capacity and lateral demand that match the exported channels within 1e-9, and utilization never exceeds 1 plus the solver's residual tolerance.
- At the apex of the hairpin, utilization is at or near 1 and the phase transitions brake to drive within a few nodes; on the entry straight the limit flag reads power-limited then speed-cap-limited as the GT approaches its cap.
- Friction-circle dot, chart cursor and car position agree at entry, apex and exit in headless browser checks; colour modes render legibly in both views and both DPRs.
- Markers are stable under refinement (positions move by less than the spacing) and are absent on a straight.

### Iteration 3: Setup workbench with a fast solve path

**Why.** F3 and F6. This is the release that delivers "experiment with car setup."

**Design.**

- **In-studio vehicle editing.** A sidebar section with steppers or drag-to-adjust fields for mass, power, brake, top speed, tyre grip, drag area, front weight fraction, front drive fraction and width. Add a **Custom** entry to the vehicle cycle, a **Save car / Load car** pair using the existing vehicle JSON format, and a **Reset to preset** action. Vehicle edits go through the same transactional pattern as scene edits so undo/redo covers setup changes. Persist the custom vehicle either as a separate file or, with a schema bump, as an inline `vehicle_config` object in the scene.
- **Progressive solving.** Add `solver.Options.Seed []float64` (initial offsets on the search road) and a `solver.Evaluate(scene, model, offsets, opts)` function that runs only the profile on the supplied line. The GUI then reacts to a setup change in two steps: immediately evaluate the current line under the new car (tens of milliseconds) and display it as **provisional**, then run the full search seeded from that line in the background and replace the result. Make solves cancellable with a `context.Context` so a rapid sequence of slider changes only completes the last one.
- **Sensitivity table.** A small panel or details view that reports the time effect of unit changes on the current road: per 50 kg, per 25 kW, per 0.05 grip, per 0.1 m² drag. Compute via `Evaluate` on the current line rather than full searches, label it **on this line**, and compute lazily when idle. Add a CLI `sweep` command that varies one parameter over a range and writes CSV; that is the enthusiast's "how much is grip worth here?" question.
- **Honesty text.** Fixed axle loads mean front weight and drive split change only static traction. Say so beside those controls until Iteration 5 lands.

**Acceptance.**

- Evaluating the optimizer's own final offsets with `Evaluate` reproduces `Result.Duration` exactly.
- A seeded full search never returns a slower result than its seed's evaluation, and with `Iterations: 0` returns the seed evaluation itself.
- Changing grip in the studio updates provisional results within one frame budget on the esses preset and replaces them with a full result without any visual regression to the centreline in between; cancelling supersedes stale results deterministically (headless check).
- Setup changes are undoable; an invalid configuration reports the failing field and preserves the previous car and both trajectories.
- The sensitivity table's entries match independent `Evaluate` calls in tests.

### Iteration 4: Pinned references and a user-authored line

**Why.** F4 and F5. This completes the experimental loop: change something, compare against what you had, and test your own hypothesis about the line.

**Design.**

- **Generalize the reference.** Introduce a `Reference` value in the presentation layer: a name, a trajectory, the vehicle config, and a digest of the road it was solved on. Sources: centreline (default), **Pin current** (keeps the current optimized line as reference), and later the manual line. The chart, ghost and delta readout consume the reference, not `CenterNodes` directly. If the road digest differs, show the reference as **stale, different road** and disable same-station comparison rather than aligning incompatible stations.
- **Manual line mode.** A toggle that shows lateral-offset handles at a modest number of stations (for example one per control point plus midpoints). Dragging a handle moves the line across the road; the smooth latent cubic already used in `interpolateOffsets` produces the full offset vector. Release evaluates the line with `solver.Evaluate` and shows its time and delta against the reference. Infeasible lines report the failing station and reason (clearance, insufficient braking, stationary slide) and highlight it on the road. Offer **Optimize from here** to seed the search with the manual line, and **Adopt** to make the manual line the pinned reference. Persist manual offsets with the scene under a versioned field so a saved study includes the user's line.
- Provide **A/B labelling** in the comparison panel: the reference's name, car and time next to the current line's.

**Acceptance.**

- Pinning and then changing the car produces a same-station delta whose exit value equals the difference of the two durations.
- A manual line whose handles all sit at zero offset evaluates identically to the centreline reference.
- A deliberately infeasible manual line reports the station and the reason; the previous valid manual line and both histories are preserved.
- Handle dragging follows the existing pointer gesture rules under rotated cameras, at both DPRs, and cannot select a geometry handle simultaneously.
- Saving and loading a scene restores the manual line and the pinned reference name.

### Iteration 5: A richer quasi-static model for setup work

**Why.** Once setup is tunable, the flat static-load model becomes the limiting factor. Sim racers and enthusiasts expect brake bias, aero and weight transfer to mean something. This remains quasi-static and stays honest about it.

**Design.** All additions are optional fields with defaults that reproduce the current envelope exactly.

- **Brake bias.** Add `front_brake` fraction. Braking capacity becomes axle-limited like drive already is: `min(front_residual/q, rear_residual/(1-q))`, capped by the existing `brake` value. This turns the current flat brake cap into a setup knob.
- **Aerodynamic downforce.** Add `lift_area` (negative lift coefficient times area) and `aero_balance` (front fraction). Normal load gains `0.5 rho ClA v²`, split by balance. Drag already exists.
- **Longitudinal load transfer.** Add `cg_height` and `wheelbase`. Axle loads change with longitudinal acceleration; because the envelope then depends on the acceleration being solved for, `Limits` iterates to a fixed point with a bounded count and a documented tolerance, or solves the closed form for the straight-line case and verifies it.
- **Tyre load sensitivity.** Add an optional coefficient by which friction falls with normal load. Without it, downforce and transfer only redistribute; with it, they change total grip and lateral transfer starts to matter.
- Keep lateral load transfer, slip angles, suspension and drifting out of scope; state this next to the new controls.

**Acceptance.**

- With all new fields at their defaults, every existing fixture reproduces its recorded times bit-for-bit.
- Analytic oracles: straight-line launch with transfer for FWD and RWD (closed form), constant-speed cornering with downforce, brake bias at the axle limit.
- Independent verification in `internal/verification` re-derives axle loads and both axle residuals per node without importing solver internals.
- Sensitivity checks: adding downforce to the GT raises apex speed on the banked sweep and lowers top speed on the hairpin straight in the way the model predicts.

### Iteration 6: Track authoring enthusiasts care about

**Why.** F7. Better corners to study, and the lap-level compromise that makes racing lines interesting.

**Design, in sub-steps that can ship separately.**

- **Asymmetric width.** Schema version 2 with `width_left` and `width_right`; migrate v1 `width` to equal halves. Enables kerbs and run-off.
- **Kerbs and track limits.** A per-side `kerb` width with its own surface and grip, and a per-scene rule for whether kerbs count as road for clearance. Visualize kerbs with the familiar red-white pattern.
- **Corner archetype presets.** Fictional, recognizable shapes: decreasing-radius, banked bowl, compression at the bottom of a crest, chicane, long double-apex, blind crest with a downhill braking zone. Keep the "not surveyed" wording.
- **Centreline import.** CSV import of x, y, z, left width, right width. Publicly licensed track centreline datasets exist in this format; importing them provides real geometry while the vehicles remain fictional, and the documentation must say exactly that.
- **Closed laps.** Periodic geometry, periodic offsets, and cyclic speed-profile convergence (iterate forward and backward passes around the loop until the start speed converges). This is the largest solver change in this document and should follow, not precede, Iterations 1 through 4. The animation then becomes a real lap rather than a restart.

**Acceptance.**

- Every v1 fixture loads and solves identically after migration.
- Kerb grip changes the solver's line where kerbs are enabled and leaves it unchanged where they are excluded from clearance.
- Closed-lap fixture: start and end speeds agree within tolerance, duration is independent of the chosen start station within the refinement tolerance, and the independent matrix covers it.

### Iteration 7: Presentation that rewards study

**Why.** F8. These are polish items that become worthwhile once the analysis content exists.

- Absolute, shared colour scales for speed and grip with a legend; the same scale across cars.
- A chase or trackside **perspective camera** as a third view. The current orthographic camera stays for editing; the perspective view is watch-only, so its inverse projection is never used for dragging.
- A live **delta bar** in the telemetry strip, in the style of sim racing delta timers, driven by the same-station delta.
- **Sector splits** at control points, with per-sector time gain against the reference, so the user sees which corner of a sequence paid off.
- Frame step and scrub by station as well as by time; export a comparison telemetry CSV containing both trajectories side by side at shared stations.

Acceptance is visual inspection in both views and both DPRs, plus unit tests for the sector arithmetic and shared-scale mapping.

### Iteration 8: Search quality and speed

**Why.** Faster and better solves make every other feature better, and the seeding work in Iteration 3 makes this safe to attempt.

- Profile the solver on esses; the current cost is dominated by repeated `Limits` evaluation across 65 samples per segment. Cache per-station road state, reduce sample counts adaptively where the envelope is flat, and evaluate a sweep's candidate set concurrently with goroutines while choosing the winner in a fixed order so results stay deterministic.
- Add a local refinement stage after coordinate search, such as a bounded derivative-free polish of the latent controls, and report coarse-to-fine agreement as an **optimizer confidence** figure in the sidebar.
- Expose search diagnostics already present in `Result` (candidates, termination, coarse duration) in a details view.

Acceptance: identical results to the sequential implementation for the existing fixtures, a measured speedup recorded in `docs/VALIDATION.md`, and no fixture time regression.

## 4. Dependencies and sizing

| Iteration | Depends on | Relative size | Primary packages |
| --- | --- | --- | --- |
| 1 Curvature-continuous roads | none | medium | `pkg/track`, `pkg/solver` tests, docs |
| 2 Edge-of-adhesion instrumentation | 1 (for clean traces) | medium | `pkg/solver`, `pkg/render`, `internal/cli` |
| 3 Setup workbench and fast solve | none; better after 2 | large | `pkg/solver`, `main/gui`, `internal/editor`, `pkg/render` |
| 4 Pinned references and manual line | 3 (`Evaluate`, seeding) | large | `pkg/solver`, `pkg/render`, `main/gui`, `pkg/track` schema |
| 5 Richer quasi-static model | 3 (controls to expose it) | large | `pkg/vehicle`, `internal/verification` |
| 6 Track authoring | 1; laps also need 3 | very large, splittable | `pkg/track`, `pkg/solver`, `pkg/render` |
| 7 Presentation | 2 and 4 | medium | `pkg/render`, `main/gui` |
| 8 Search quality and speed | 3 | medium | `pkg/solver` |

If only two releases can be built next, build 1 and 2: they turn the existing verified mathematics into something a sim racer trusts and can read. If four, add 3 and 4: they turn the viewer into a laboratory.

## 5. Cross-cutting requirements

- **Schema.** Any new serialized field lands as an optional field, or as `Version: 2` with a v1 loader. Update `examples/` and `README.md` when fields change, per `AGENTS.md`.
- **Honesty.** New readouts are labelled with their derivation ("model tyre forces", "on this line", "quasi-static"). Fictional vehicles stay fictional in every label. Imported real geometry is credited and distinguished from the vehicles.
- **Testing.** Analytical oracles for every new force term; independent reconstruction in `internal/verification`; renderer tests on a full-budget result; real-input headless checks for every new gesture or control, in both existing display configurations. Keep the existing 18 grouped scenarios green.
- **Performance budgets.** Provisional evaluation under one frame at 60 Hz on esses; full solve stays off the UI thread and cancellable; camera redraw remains under the currently measured 30 ms.
- **Evidence.** Each release records commands run, measured figures and remaining limits in `docs/VALIDATION.md`, as the last two releases did.

## 6. Evidence gathered for this review

- Read `AGENTS.md`, `README.md`, `research/*.md`, `docs/*.md`, `pkg/vehicle/vehicle.go`, `pkg/solver/{solver,profile,geometry,doc}.go`, `pkg/track/{track,presets}.go`, the editor API, the GUI action dispatcher and the renderer function inventory.
- Inspected `artifacts/browser/plan-gt-comparison.png` and `artifacts/browser/elevated-retina-gt-comparison.png` (sawtooth speed traces on the esses), and rendered `artifacts/review-hairpin-2d.png` with the GT car (clean outside-inside-outside line, single braking phase).
- Ran `go build ./main/cli`, then `solve --preset esses --vehicle gt --format csv` (12.760 s versus 14.662 s centreline, 3.1 s wall time) and `solve --preset hairpin --vehicle gt` (10.864 s versus 11.309 s). Analysed the CSV: 1,527 nodes, maximum absolute offset 4.64 m against a 4.78 m bound, so the search does use the road width; adjacent-node curvature jumps up to 0.0142 1/m located at control-point stations, which is the source of Finding F1.
- Did not open native windows and did not modify source or fixtures. Generated files live under the ignored `artifacts/` directory.
