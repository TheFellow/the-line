# Laboratory roadmap implementation

This records implementation of [the original roadmap](../research/NEXT_STEPS.md).
The roadmap is a design proposal; its target figures are not acceptance evidence.

## 1. Curvature-continuous roads

Implemented in `357be17`: natural cubic centreline splines and bounded C1 bank
and width interpolation, preserving source points and surface boundaries.
Analytical spline and adversarial ribbon tests pass. Measured same-path
refinement differences are 0.009–0.044%; the regression tolerance is now 1%.
The complete numerical table is in [VALIDATION.md](VALIDATION.md).

Root rendered and inspected all five presets in both views with:

```sh
go run ./tools/review --out artifacts/iteration1-review
node tools/browser/verify.mjs --scope=interaction
```

All 24 original interaction checks passed, across plan/DPR1 and elevated/DPR2.
The retained report is `artifacts/browser/iteration1-interaction-report.json`.
The gallery shows smooth joins, banked road alignment and substantially cleaner
speed traces. The proposed 0.0005/m near-knot curvature threshold is not met by
compound/rally; their measured bound is below 0.001/m. Some secondary optimized
speed minima remain. Verified trajectories were not visually smoothed to hide
them.

## 2. Model tyre forces

Exported signed tyre demands, available capacity, combined utilization, phases
and limiting constraints. Fixed scales, a live force-envelope widget, chart
channels and sustained braking/apex/drive markers use those verified channels.
See [instrumentation](INSTRUMENTATION.md) and [trajectory contracts](TRAJECTORIES.md).

## 3. Setup and progressive solving

Two setup pages, atomic car persistence, inline study cars, undo/redo and lazy
same-line sensitivities are implemented. `Evaluate` and cancellable seeded
search preserve the current path for provisional results. WASM explicitly yields
to the event loop so provisional states are visible and stale generations cannot
replace newer edits. Earlier actual-browser checks observed both properties at
both DPRs (`artifacts/browser/iteration3-provisional-report.json`).

The proposed one-frame latency is **not met**: cold native evaluation measured
41.42 ms, above 16 ms. Work stays asynchronous; no claim of a one-frame solve.

## 4. Pinned references and manual lines

Pin a complete verified scene/car/line, drag lateral diamonds, evaluate, adopt,
optimize from the authored line, and save the study. Road digests disable stale
same-station comparisons. Rejected lines report a station and preserve history.
CLI saved-study evaluation and exports share these contracts. The active line
selection is persisted separately from the retained manual hypothesis, so choosing
optimization survives save/load, car changes and undo/redo. A wider-car regression
also verifies that an infeasible old search seed falls back to a fresh solve.
See [line studies](LINE_STUDIES.md).

## 5. Optional axle dynamics

Added brake bias, downforce/aero balance, longitudinal load transfer and tyre
load sensitivity. Zero defaults preserve legacy outcomes exactly. Analytical
launch, cornering and brake oracles plus independent axle reconstruction cover
the model. Downforce alone does not increase drag: the roadmap's unconditional
lower-top-speed expectation was physically incorrect and was not implemented.
See [model limits and verification](VEHICLE_MODEL.md).

## 6. Road authoring and laps

Version 2 supports asymmetric asphalt widths, per-side kerbs/grip and explicit
track-limit rules. Version 1 migrates compatibly. Six new fictional archetypes,
CSV import and a closed club circuit join the original five scenes. Periodic C2
geometry and offsets feed a cyclic speed profile; cars wrap at their own periods.
The final fresh audit fixed kerb contact at tapering asphalt edges using actual
circular footprint-to-edge distances, including adjacent cells. See
[authoring](TRACK_AUTHORING.md), [laps](CLOSED_LAPS.md) and the
[fresh correctness review](../research/ROADMAP_CORRECTNESS_REVIEW.md).

## 7. Study presentation

Trackside perspective, fixed-scale delta, control-point sector splits, frame and
station stepping, and shared-station comparison CSV are implemented. Perspective
is watch-only; orthographic views retain inverse-projected editing. See
[presentation](STUDY_PRESENTATION.md).

## 8. Deterministic search and refinement

Prepared geometry/car state and exact reuse of dense feasibility checks reduced
the measured legacy esses solve from 4.246 to 1.564 seconds (**2.71×**). Five
legacy fixture results remain bit-for-bit identical. Bounded native workers
select finalists in fixed order; custom models and WASM remain serial. Optional
local polish retains the prior verified best candidate. The sidebar reports
candidates and measured same-path coarse/fine agreement, not confidence in a
global optimum. Rich all-option model search still measured 45.27 seconds.
See [search contracts](SEARCH.md) and [measurements](VALIDATION.md).

## Final integration acceptance

`go test ./...`, `go vet ./...`, `make build`, a CGO-disabled CLI build and the
normal WebAssembly studio build pass. The complete independent numerical suite
includes the final kerb correction and active-study selection regressions.

Real-input browser acceptance passes **76 distinct case/check groups**: 38 in
plan/DPR1 and 38 in elevated/DPR2. This exercises actual click/drag, orbit,
cancel, undo/redo, setup, saved studies, CSV file selection, force inspection,
perspective controls, refinement, and periodic seam editing. The elevated run
passed the full suite. The plan run passed 23 groups before a test clicked the
timeline's excluded right edge; after correcting the test to capture inside the
bar and drag to the endpoint, all 20 roadmap groups passed (five overlap).
The original report is retained, and every failed group has a passing rerun.
The consolidated evidence is `artifacts/browser/final-report.json`.

```sh
node tools/browser/verify.mjs --scope=all --case=elevated-retina --artifacts=artifacts/browser-elevated
node tools/browser/verify.mjs --scope=all --case=plan --artifacts=artifacts/browser-plan
node tools/browser/verify.mjs --scope=roadmap --case=plan --artifacts=artifacts/browser-plan-recheck
node tools/browser/verify.mjs --scope=closed --case=plan --artifacts=artifacts/browser-plan-closed
```

The last focused run repeats three passing lap checks with Fit after the view
switch, keeping the full road visible in the final seam-drag capture. Ordinary
view switching deliberately preserves the user's camera zoom.

The visual matrix covers all twelve presets in plan, elevated and perspective
views (36 PNGs), with GT vehicle settings and the default search. Every line
improves on its identical-condition centreline reference by 2.292–13.889%; the
maximum force-constraint residual is 9.14×10⁻⁷ m/s². The 40-second closed
animation contains 800 frames at 20 FPS, 960×600, including independently phased
cars through lap boundaries. Perspective car markers preserve physical footprints and respect foreground
occlusion; decoded frames show their independent progress through the seam. Evidence is under
`artifacts/final-roadmap-review/`; generated files remain ignored by Git.
All 36 matrix images, eight decoded animation frames, and 32 browser feature
captures were visually inspected. Browser contact sheets are
`artifacts/browser/visual-review-01.png` through `visual-review-08.png`.

The final native display-independent renderer probe advances 240 frames through
two laps in each view, with sectors enabled at 1440×900. It measured 2.26 ms/frame
(plan), 2.66 ms/frame (elevated), and 4.69 ms/frame (perspective), while other
validation tasks ran. This measures CPU frame composition, not native-window
presentation FPS. Software-rendered headless Chrome is slower and is used for
input/correctness validation. Evidence: `artifacts/final-render-throughput.json`.
