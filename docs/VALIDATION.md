# Validation record

Measured on 2026-09-09 with Go 1.24.0, macOS/amd64, Intel Core i5-1038NG7. All values refer to the implemented synthetic, quasi-static vehicle model, not real-car measurements.

## Build and headless operation

- `make build`: both executable binaries built successfully.
- `go test ./...`: all packages passed, including CLI image/animation workflows, editor transactions, analytical physics and independent trajectory verification.
- `go vet ./...`: passed.
- `CGO_ENABLED=0 go build -o bin/the-line-headless ./main/cli`: passed. The CLI has no graphics-driver initialization or native game-engine requirement.
- CLI creation → validation → JSON/CSV solve → PNG in both views → timed GIF passed in `internal/cli`. Failed exports preserve existing files.

## Solver results — C2 roads, 2026-09-10

Default vehicle for each scene, 3 m search spacing, 0.5 m final validation spacing, four search sweeps, 0.25 m additional clearance:

| Sequence | Centreline | Verified line | Improvement | Same-path 0.5 → 0.25 m time change |
| --- | ---: | ---: | ---: | ---: |
| Hairpin | 13.1927 s | 12.7846 s | 3.09% | 0.034% |
| Esses | 15.8486 s | 14.2406 s | 10.15% | 0.010% |
| Compound | 14.2469 s | 13.4994 s | 5.25% | 0.039% |
| Banked | 10.3477 s | 10.0905 s | 2.49% | 0.009% |
| Rally | 22.5854 s | 21.4534 s | 5.01% | 0.044% |

The independent matrix covers all 15 track/vehicle pairings, at one search sweep. It checks exact segment clearance to road sides, triangle surface heights, endpoint speed caps, finite values, time/acceleration kinematics, and 129 independent force-balance evaluations per output segment. All passed; maximum longitudinal force excess was 0.000000865 m/s². See [the independent implementation review](../research/IMPLEMENTATION_REVIEW.md) for method and caveats.

After the C2 centreline change, measured same-path changes are at most 0.044%; the implemented regression tolerance is tightened from 2% to 1%. This supersedes the earlier Hermite-road table, whose maximum was 1.222%. Searching at a different coarse resolution can discover a different local solution. Neither test establishes a global optimum. The earlier Hermite-road default esses solve benchmark took 2.13 s with 20.4 MB allocated; solving happens off the UI thread.

## Rendered artifacts and live execution — initial release, 2026-09-09

The final binaries generated and the agent visually inspected:

- `artifacts/esses-2d.png`: complete plan view, speed-coloured line, control points, moving vehicle and telemetry.
- `artifacts/banked-3d.png`: elevated banked road, ground reference and height ties.
- `artifacts/rally-3d.png`: asphalt/gravel/dirt transitions and changing elevation.
- `artifacts/banked-animation.gif`: complete 10.30 s exported sequence, 206 frames at 20 FPS, 960×600. Start/middle/end frames were decoded and inspected; the vehicle and telemetry advance.
- `artifacts/live-banked.png` and `artifacts/live-banked-report.json`: actual Ebitengine framebuffer capture from the final numerical implementation.

The final continuous native run rendered **1,800 frames** and **907 updates** over **16.12 s** at 1440×900. Ebitengine reported **120.0 FPS / 59.5 TPS** at completion. This exceeded the banked sequence's 10.255 s duration, exercised a complete animated traversal, and continued into its next loop. FPS is the engine's final measurement, not total frames divided by wall time including startup/capture.

The separate GUI automation exercises 21 actions through the same dispatcher used by keyboard/mouse events: point selection, width/bank/height/position editing, insertion/deletion, undo/redo, surface changes, save/load, new scene, vehicle/preset changes, both views, scrubbing and playback. The final `make verify-gui` run passed all 21 actions without error: 4,172 Draw frames and 2,125 updates over 36.56 s, with final measurements of 109.3 FPS / 59.6 TPS. `artifacts/editor-report.json` and `artifacts/editor.png` retain that run. The saved and reloaded scenes were identical; the changed control had width 12.5 m, bank −13°, elevation 3.7 m, and y −54 m. Separate negative automation runs verified that save/load/capture errors are retained in reports and cause a nonzero exit.

Run `make verify` to regenerate exports and execute tests. Run `make verify-gui` on a graphical desktop to regenerate the editor capture/report; an automated action failure exits nonzero. Generated artifacts are intentionally ignored by Git.

## Model boundaries

These are open, non-self-intersecting road sequences. Entry/exit speeds are caps; lateral endpoint positions/headings are free. Clearance protects a horizontal circular margin, not the swept body of a steering car. The road uses a triangle mesh with sampled curvature. The vehicle model has static axle loads, fixed drive torque split, constant surface friction and a decoupled bank/grade approximation. Steering dynamics, load transfer, calibrated tyre slip, drifting, suspension, crest unloading, jumps and periodic full-lap optimization are not implemented. Those require richer models and additional validation.

## Pointer interaction follow-up

Control handles now highlight on hover, use a larger grab area, preserve an off-centre grab offset, and show amber road edges while dragging. Escape cancels a drag. The preview is uncommitted; release applies one undoable edit and starts the solver. Invalid drops retain the prior scene and display their error.

`TestPointerDragWithActualProjection` exercises both views at 1440×900 and 800×600, including off-centre grabs, click versus drag, fixed elevation, undo and invalid drops. Renderer checks cover preview visibility and cache isolation. `go test ./...` and `go vet ./...` passed. Both view previews and the actual native drag capture were inspected.

Native focused verification passed in both views using `--demo-drag --frames 240 --report ... --capture ...`: the third banked control moved from (−35, −55, 3.2) to (−33, −54, 3.2) through the same pointer handler used by mouse input. Reports are `artifacts/drag-focused.json` and `artifacts/drag-focused-2d.json`. The full demo now includes pointer drag and undo (23 actions); automated runs start unfocused and ignore unrelated keyboard input.

## Real pointer events without desktop windows

`make verify-headless` builds the Go studio for WebAssembly and runs it in isolated headless Chromium contexts. Playwright sends mouse movement, press/release, and keyboard events through Ebitengine's ordinary input path. A `browsercheck` build tag exposes immutable scene, projection, history, and solver snapshots for assertions; it provides no command or mutation API. Normal builds omit that bridge.

The baseline browser run reproduced a focus-loss defect: blurring the canvas during a drag committed the preview because Ebitengine cleared its held button. The original control at (10, −30, 4.8) unexpectedly became approximately (14.00625, −27.19562, 4.8). `artifacts/browser/before-fix-focus-loss.json` records the failed assertion. Focus loss and canvas departure now cancel the gesture. Browser departure also clears the engine's held-button state so a subsequent fresh drag works. A confirmed new press can clear the cancellation latch.

Timeline gestures now require a press on the timeline and retain ownership when moved beyond its vertical strip, clamping to the sequence endpoints. Rejected provisional edits restore an editor checkpoint, including selection and both history stacks. This avoids making a rejected edit redoable or destroying an older redo branch.

The browser checks use a 1440×900 plan view at device scale 1 and a 1000×700 elevated view at device scale 2. Expected drag positions account for Ebitengine's integer logical cursor sampling and preserve the initial grab offset. Result checks compare the edited and solved scenes, require a changed trajectory digest and finite duration, and bound force residuals. Undo/redo must reproduce the original scene and trajectory exactly. Software-rendered browser frame rates are not measurements of native desktop performance.

Final real-input run passed all **12 scenarios in each view** (24 total): click-only selection; off-centre drag preview/release; undo/redo; Escape, focus-loss, Tab and canvas-leave cancellation; invalid geometry preserving redo; solver rejection preserving redo; timeline gesture ownership; captured/clamped timeline dragging; rendered playback advancement. The elevated/DPR2 case took 95.463 s and plan case 67.658 s. Both finished with the restored original trajectory (10.25505933 s) and zero reported force residual. The suite exited successfully without browser errors.

`artifacts/browser/report.json` contains the complete assertions' scenario list and final state. `*-held.png` and `*-released.png` show actual browser-rendered drag states; `*-animation-a.png` and `*-animation-b.png` show the vehicle and telemetry advancing in both views. These images were visually inspected. `go test ./...`, `go vet ./...`, `make build`, and a normal untagged WebAssembly build also passed. No native desktop windows were opened during this follow-up.

## Camera and reference analysis release

The fresh [enthusiast review](ENTHUSIAST_REVIEW.md) selected three connected improvements: controllable camera, a retained verified centreline trajectory with a shared road coordinate, and synchronized ghost/speed comparison. These are implemented without changing the vehicle envelope or numerical search objective.

The solver and independent verification suites now check both selected and retained reference trajectories. Station queries have independent constant-acceleration, launch and braking oracles; the numerical matrix also reconstructs reference station through triangle crossings. Renderer tests use a full-budget esses result whose road station differs visibly from normalized path distance. They verify a shared chart speed scale, station seeking across three display sizes, visible ghost pixels at the separately queried reference position, and clipping under extreme camera pan/zoom. Camera tests cover an analytical projection oracle, inverse transforms, multiple quadrants/elevations/heights, finite-input validation, resource reuse and preservation across renderer rebuilds.

The complete CLI comparison animation `artifacts/analysis-esses.gif` was decoded and inspected at entry, middle and after the optimized finish: 348 frames at 20 FPS, 960×600, 17.40 s. At 7.00 s the cars are visibly separated and the same-station delta is −1.164 s. At 16.00 s the optimized car remains at the open endpoint while the reference continues. This extends through the 17.363 s reference run, rather than ending at the optimized 15.035 s finish.

A native display-independent probe measured approximately 28.56 ms per plan-camera update and 30.55 ms per elevated-camera update across 120 calls. Camera redraw reuses fonts and image resources; stationary held gestures skip repainting. These are cached-base rendering timings, not native Ebitengine FPS or browser input latency.

The real browser test exposed excessive zoom from raw browser wheel deltas: a single ordinary wheel event could jump to the maximum scale and push handles offscreen. Zoom now bounds each update before applying sensitivity. `artifacts/browser/wheel-regression.json` retains the pre-fix failure evidence. Viewport clipping and hit testing prevent hidden, panned-offscreen handles from being selected through the interface.

Final `make verify-headless` passed **18 grouped scenarios in each configuration, 36 total**, with no browser errors. The elevated 1000×700/DPR2 run took 223.036 s; the 1440×900 plan run took 127.828 s. Each preserves the original 12 interaction regressions and adds camera control, camera cancellation/transformed dragging, preset/vehicle changes, chart gesture ownership, reference-finish behavior, and playback-rate checks.

Both ended on the restored road-car esses result: optimized 15.034620715 s, reference 17.363171694 s, zero reported force residual, 16 solver requests, comparison enabled and playback rate 1×. Camera-only actions leave scene, history, trajectory and solve-request count unchanged. Actual mouse clicks switched to the GT vehicle and Undo restored the original car and both trajectories; the GT capture shows 12.76 / 14.66 s and 331 kW/t. The rate checks verified simulated elapsed time against update counts at 0.25×, 0.5×, 1× and 2×.

`artifacts/browser/report.json` contains the final combined run. The opposite-side, transformed-edit, GT-comparison, ghost-finish and comparison PNGs record actual browser output; both views were visually inspected. Measured eight-step camera gestures took 1.545–1.643 s in elevated software Chrome and 0.985–1.293 s in plan view, including input dispatch and synchronization. These do not measure native desktop responsiveness.

The final source passed `go test ./...`, `go vet ./...`, and `make build`. Both native binaries were rebuilt. Fresh CLI plan/elevated comparison images and the complete GIF were rendered and inspected. The final integration review found and corrected progress rescaling for valid sub-second sequences; no further actionable issues remained. No native desktop windows were opened for this release.

## Iteration 1: curvature-continuous roads, 2026-09-10

`track.SampleRoad` now fits a natural cubic through the original control points in spatial chord length. Position and the first two derivatives are continuous at every knot. Arc-length sampling retains every control point and categorical surface boundary. Width and bank use bounded smoothstep interpolation in each span's arc-length fraction, with zero slopes at joins. Version 1 scenes load unchanged; their between-control geometry and estimated times intentionally change. Natural cubic interpolation can influence neighboring spans when one control moves and can overshoot; authoritative ribbon validation still rejects unsupported folds and crossings.

Independent sampled-position circumcircles at 0.5 m spacing give these absolute adjacent-curvature differences:

| Preset | Maximum anywhere (1/m) | Maximum within 0.5 m of a control (1/m) |
| --- | ---: | ---: |
| Hairpin | 0.00035286 | 0.00027400 |
| Esses | 0.00047457 | 0.00044158 |
| Compound | 0.00105270 | 0.00088287 |
| Banked | 0.00009582 | 0.00007307 |
| Rally | 0.00089461 | 0.00075723 |

The fixture regression bounds are 0.0015 1/m overall and 0.001 1/m near controls. The suggested 0.0005 near-control target holds for three presets, but not compound or rally. C2 continuity removes jumps; it does not bound a road's curvature gradient. The largest sampled changes occur between control stations rather than exactly at a control. Tests separately prove C2 continuity, natural endpoint conditions, a closed-form three-point arch, exact straight grade, bounded cross sections, retained surface stations, and rejection of wide folds, self-crossings, reversals and short wide bends at multiple spacings.

The proposed “one local speed minimum per corner” criterion is **not achieved** by this change. With a 1e-6 m/s dead band, the default esses optimized road-car trace has nine minima and the GT trace has eight; their centreline traces each have five. Some additional extrema are small (the road-car rise at 59.79–62.46 m is 0.090 m/s). C2 roads do not impose curvature monotonicity on the reference or optimized line. The verified speed trace remains unsmoothed so these optimizer/model features are visible; no appearance-only filter conceals them. Addressing that stricter criterion needs explicit path-shape constraints or a richer search objective, assessed against travel time and force feasibility.

`go test ./pkg/track ./pkg/solver ./internal/verification -count=1 -v` passed. Optimized same-path refinement now stays below 0.045% on every supplied preset, and the regression threshold is tightened to 1%. The full measured table above supersedes the initial release figures.

The integration review rendered every preset in both views with the GT using `go run ./tools/review -out artifacts/iteration1-review`. All ten images in `artifacts/iteration1-review/gallery.png` were visually inspected: smooth road joins, line/car alignment with the ribbon, readable station charts and surface transitions. This matrix uses a single fixed GT configuration; the numerical fixture table uses each preset's default vehicle. Native windows were not opened.

## Iteration 4: authored studies and pinned references (2026-09-10)

`go test ./pkg/render ./internal/editor ./pkg/solver -run 'TestPinned|TestManual' -count=1`
passed. These checks independently assert zero authored offsets reproduce the
centreline time exactly, constant-offset interpolation, lateral dragging along
banked cross-sections at multiple camera angles/scales with an off-centre grab,
immutable pinned node snapshots, exact reference duration after serialization
reconstruction, exit station delta equal to the duration difference, and rejection
of station/ghost comparison on changed geometry. Track persistence tests also check
deep copies, inline study roundtrips, road-identity exclusions and nested-reference
rejection. The actual browser gesture/storage scenarios are in
`tools/browser/manual.mjs`; their final results are recorded with the integrated
headless suite below.

Rendered and visually inspected `artifacts/study-2d.png` and `study-3d.png`: cyan
lateral handles follow the banked road, authoring controls and A/B names/times are
visible, the independent pinned ghost and same-station traces agree, and the
centreline manual run remains visibly slower than the pinned optimized esses run
(15.85 versus 14.24 seconds in this fixture). These screenshots establish
presentation, not calibration against a real vehicle.

## Iteration 8: deterministic search performance, 2026-09-10

Profiled the full default road-car esses solve before changes with
`go test ./pkg/solver -run '^$' -bench BenchmarkSolveEsses -benchtime=1x -cpuprofile artifacts/iteration8-before.cpu`.
On this Intel Core i5-1038NG7 / Go 1.24 / darwin-amd64 workspace, the initial
single-run result was **4.246 s**, 67.90 MB allocated. The profile attributed
68.5% of sampled CPU to segment envelope work, including repeated trigonometry
and copies of the force-instrumentation payload.

The final benchmark command was
`go test ./pkg/solver -run '^$' -bench 'Benchmark(Solve|Evaluate).*Esses' -benchtime=1x -cpuprofile artifacts/iteration8-final.cpu`.
No other test/solve processes ran during that final measurement:

| Case | Final elapsed time | Allocated bytes |
| --- | ---: | ---: |
| Full default road-car esses search | 1.564 s | 111.22 MB |
| Centreline Evaluate, cold cache | 41.42 ms | 9.17 MB |
| Full richer-model esses search | 45.269 s | 103.05 MB |

The legacy full solve was **2.71× faster** in this before/after single-run
comparison. Cached road projections trade extra allocation for fewer repeated
calculations; a bounded worker pool and reusable segment storage limit retained
scratch space. These measurements are workload timings, not GUI frame rates or
a general benchmark guarantee. Rich-model timing uses front brake .6, CG .35 m,
wheelbase 2.8 m, load sensitivity .12, lift area 3 m² and aero balance .45. It
remains substantially more expensive than the legacy envelope. The **16 ms
provisional-evaluation target is not met**; evaluation/search remain asynchronous
and cancellable. Earlier rich timing taken under contention is not used to claim
a precise speedup.

`TestSearchWorkersPreserveFixtureTrajectories` compared every optimized and
centreline node, every offset, duration and candidate count against sequential
unprepared `Model.Limits` evaluation at the full default search budget. All five
legacy fixtures were bit-for-bit identical: hairpin 12.784581926891 s, esses
14.240646502058 s, compound 13.499439525622 s, banked 10.090487994429 s and rally
21.453428463520 s. A bounded optional polish after one esses coordinate sweep
improved 14.441873790 s to 14.352392221 s; its shortlist preserves every original
finalist and retains seed/centreline fallback.

`go test ./pkg/solver ./internal/verification -count=1` passed (71.12 s and
67.13 s when run together). The independent matrix includes the richer axle
force reconstruction. `go test ./pkg/vehicle -count=1` passed prepared/public
envelope equivalence over speed, bank, grade, grip and optional setups, existing
analytic launch/braking/downforce oracles, unchanged legacy-default bits and
positive-base power-law equivalence. `go test -race ./pkg/solver -run
'TestIndependentFinalistsWorkerDeterminism|TestSearchCancellation' -count=1`
passed. Final verification still uses all 65 envelope samples (one equivalent
evaluation on exactly constant segments), with separate denser independent
checks. New diagnostics describe **measured coarse-to-fine agreement**, never a
probabilistic optimizer confidence. See [SEARCH.md](SEARCH.md) for contracts.

## Once-over corrections, 2026-09-10

The response to [ROADMAP_ONCE_OVER.md](../research/ROADMAP_ONCE_OVER.md) is
tracked item by item in [ONCE_OVER_RESPONSE.md](../research/ONCE_OVER_RESPONSE.md).
Final product commit: `9bcc6dc`.

The independent axle boundary grid covers 8,000 signed limits, including load
sensitivity, unloading, near-saturation and near-lift-off cases. The maximum
active-constraint residual was **2.66e-11 m/s²** and maximum error against an
independently bisected root was **3.49e-11 m/s²**. These are measured cases, not
a universal convergence guarantee. The 40-iteration conservative fallback remains.

An isolated `git archive 6620b93` supplied the five original preset optimized
and centreline durations. `TestLegacyPresetDurations` requires bit-for-bit
agreement with all ten numbers and passes. Three independently captured road
hashes also anchor pre-salt study migration. Stale manual studies remain stale;
pinned references migrate against their separately preserved source scenes.

The shared `editor.SolveSetup` helper is exercised with the real solver: a rejected
seed triggers exactly one unseeded retry and produces a verified result. Invalid
car/road failures and cancellation do not retry. A replacement curvature envelope
also makes a coarse baseline feasible but its refined baseline infeasible;
that error is explicitly tested as **not** a seed failure, including a zero-budget
solve. Seed-shape rejection retains its typed error at zero budget.

The full headless browser suite passed **40 checks in each configuration, 80
total**, with no browser errors. The plan run took 973.923 s and the elevated
retina run took 1223.080 s. Both include wider-car fresh-line recovery and a
failed superseding edit preserving its valid predecessor, solved scene and
undo/redo history. The reports are
`artifacts/once-over-browser-plan/report.json` and
`artifacts/once-over-browser-elevated/report.json`.

Visual review regenerated all **36 images** (12 presets × plan, elevated and
perspective) under `artifacts/once-over-review/`. All four gallery sheets and
representative full-size images were personally inspected. Current/default GT
lines improve on their matching centreline by **2.292–13.889%**; the largest
reported force residual is **9.1354e-7 m/s²**. These remain heuristic estimates
under the declared quasi-static model. Road outlines, kerbs, corner markers,
vehicle occlusion and both speed traces are legible. The speed chart now uses
its complete comparison range while the color legend retains an absolute scale.

`club-loop-animation.gif` contains **800 frames, 40 seconds at 20 FPS**, at
960×600 in perspective view. Its decoded eight-frame contact sheet and full-size
frames around the 19.333-second lap boundary were inspected; position, clock and
chart cursor wrap consistently. This export demonstrates animation correctness,
not a claim of native-window FPS. Software-rendered browser frame rates are
likewise used for interaction correctness, not hardware performance claims.

Reproduction:

```sh
go test -count=1 ./...
go vet ./...
make build
CGO_ENABLED=0 go build -o bin/the-line-headless ./main/cli
GOOS=js GOARCH=wasm go build -o artifacts/once-over-release.wasm ./main/gui
go test -race ./pkg/solver -run 'TestIndependentFinalistsWorkerDeterminism|TestSearchCancellationAndOptions|TestEvaluate' -count=1
node tools/browser/verify.mjs --scope=all --case=plan --artifacts=artifacts/once-over-browser-plan
node tools/browser/verify.mjs --scope=all --case=elevated-retina --artifacts=artifacts/once-over-browser-elevated
go run ./tools/review --out artifacts/once-over-review
go run ./main/cli animate --preset club-loop --vehicle gt --view perspective --duration 40 --fps 20 --width 960 --height 600 --out artifacts/once-over-review/club-loop-animation.gif
```

## Two-car racecraft (2026-09-10)

The four checked-in experiments use the existing solver with an increased road
margin enclosing a 4.4 m × vehicle-width body. `pkg/racecraft` checks a conservative
relative-speed bound over the full shared-time interval, recursively subdividing
ambiguous intervals. A pair is returned only when every interval is certified;
resolution/work-budget exhaustion rejects the pair. No interpolation between
render frames can bypass that check.

Default outcomes are recorded in [RACECRAFT.md](RACECRAFT.md): the over-under ends
with B ahead, the pass/repass restores A's lead, defence holds with a larger gap,
and the esses demonstrate trading nose-ahead advantage without a completed pass.
Independent tests verify segment kinematics and force limits, exact planar
segment-to-segment body clearance against all road edges, dense car-to-car
clearance, deterministic replay, an occupied-line fallback, changed outcomes,
input errors, cancellation, persistence and failed-export preservation. The
between-frame collision fixture crosses two cars inside a 10 ms interval despite
safe interval endpoints.

The focused real Ebitengine browser checks passed at 1000×700 / DPR 2 and
1440×900 / DPR 1. They exercise entering/exiting racecraft, preserving the
qualifying scene and trajectory, gap/overspeed/separation controls, safe rejection,
complete experiment save/load, scenario changes, shared timeline scrubbing,
keyboard frame stepping, restart and both views. Captures
`artifacts/browser/elevated-retina-race-repass.png` and
`artifacts/browser/plan-race-over-under.png` were visually inspected.

`go run ./tools/preview` generates the public GIF from the normal qualifying and
racecraft CLI exports. The 391-frame, 960×600 result contains qualifying on Club
Loop, an over-under in plan view and a pass/repass in elevated view. Decoded
composited frames from all three clips were visually inspected. Transparent
unchanged pixels reduce the combined preview to approximately 1.5 MiB without
adding an external animation tool dependency.

Final Go validation passed: `go test ./...`, `go vet ./...`, and `make build`.
A `CGO_ENABLED=0` CLI build also exported the pass/repass CSV successfully. After
chart rescaling and save-path presentation updates, the focused browser suite
passed again in both viewports (six workflow checks total), recorded in
`artifacts/browser-race-final/report.json`. A 40 m starting-gap export was also
visually inspected to verify the adaptive position-chart scale.

The complete `make verify-headless` run passed all **86 workflow checks**:
43 at 1000×700 / DPR 2 and 43 at 1440×900 / DPR 1. The final
`artifacts/browser/report.json` has no failed cases or browser errors. Both final
racecraft captures were inspected. This exercises the actual Ebitengine renderer
and real mouse/keyboard input without opening desktop windows.

## Independent racecraft critique follow-up (2026-09-10)

The [research brief](../research/RACECRAFT_IMPLEMENTATION.md) was reviewed against
source through the Claude CLI. Its [verbatim critique](../research/CLAUDE_RACECRAFT_CRITIQUE.md)
and [finding-by-finding response](../research/RACECRAFT_CRITIQUE_RESPONSE.md) record
what was inspected, what was reproduced, and how fresh implementation agents
addressed the findings in focused commits.

After the fixes, `go test ./...`, `go vet ./...` and `make build` passed. A
`CGO_ENABLED=0` CLI build exported pass/repass CSV. Added tests cover zero and
fractional entry caps, meaningful failure causes, road-distance placements on
nonuniform geometry, all three vehicle presets on a custom banked road,
first-finish behavior, the certified lower-bound property, analytic pass/repass
thresholds, tangent contact, interval-budget exhaustion and failed/aliased exports.
The cap and station-placement regressions were checked against the old code and
failed for their intended reasons.

The final actual Ebitengine browser runs passed **26 workflow checks** with no
browser errors: **20 racecraft** checks and **six authoring** checks across
1000×700 / DPR 2 and 1440×900 / DPR 1. They cover ordinary controls and save/load,
failed-load and failed-plan preservation, held gesture cancellation, perspective
return, delayed CSV reads across mode boundaries, custom example replacement,
cancelling a pending plan, startup flag interactions, normal CSV import, rejected
imports and undo/redo preservation. Reports are retained at:

- `artifacts/browser-racecraft-review-final/report.json`
- `artifacts/browser-authoring-review-final/report.json`

The final elevated pass/repass, plan over-under and both CSV import captures were
inspected. These post-review runs target the modified workflows; the 86-check
full pre-review baseline above remains separate evidence.

The README animation was regenerated with `go run ./tools/preview`, decoded and
visually inspected across all three clips: **391 frames, 960×600, 1,601,432 bytes**.
Both standalone PNG views were inspected, including 6.25 m separation precision.
The corrected station mapping retains the intended pass/repass outcomes but
changes timing; current measurements and the 11 m no-pass gap fixture are in
[RACECRAFT.md](RACECRAFT.md).

Reproduce the focused browser checks without desktop windows:

```sh
node tools/browser/verify.mjs --scope=racecraft --artifacts=artifacts/browser-racecraft-review-final
node tools/browser/verify.mjs --scope=authoring --artifacts=artifacts/browser-authoring-review-final
```
