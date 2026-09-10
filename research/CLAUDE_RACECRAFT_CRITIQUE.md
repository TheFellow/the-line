<!-- Preserved reviewer response; subsequent dispositions are in RACECRAFT_CRITIQUE_RESPONSE.md. -->

Review requested 2026-09-10 using Claude Code CLI 2.1.267 against commit `2e99f2b` (implementation `a887689`). The CLI used its configured model (`claude-fable-5-1` reported in model usage), read-only Read/Glob/Grep tools, safe mode, no MCP servers and no session persistence. It completed successfully in 455 seconds with no permission denials. No tracked files changed during the review. The reviewer did not execute tests. The response below is preserved verbatim.

---

# Independent critique: two-car racecraft (branch `feat/two-car-racecraft`, commit `2e99f2b`)

## Overall assessment

The core safety argument is sound and the code matches it. I checked the constant-acceleration interpolation in `pkg/solver/trajectory.go:36-55` against the segment kinematics in `pkg/solver/profile.go:205-216` and the path construction in `pkg/solver/geometry.go:55-61`. Node `S` increments equal 3D chord lengths, position moves along the chord at the linearly varying speed, and speed is bounded by the segment endpoints. So the Lipschitz bound in `pkg/racecraft/collision.go:35-38` is valid, the half-interval argument holds, and the budget and resolution floor make rejection the default outcome. The GUI state machine is transactional for the cases that matter: a failed plan or failed load leaves the last accepted result untouched.

The substantive problems are elsewhere. Planning failures are attributed to collisions even when the real cause is road fit, road length or vehicle feasibility, and the default hairpin already triggers that misattribution at a separation the controls allow. The entry-cap "advantage" does not mean what the label says because car A accelerates over the starting gap before the race clock starts. Documentation describes the crossover delay in road length while the code uses sample index. Test coverage is strong on the four presets and nearly absent for any other road or vehicle.

Validation limitation: I had read and search tools only. I did not run `go test`, `go vet`, `make build` or the browser suite. All numeric counterexamples below are hand-derived from the source and the preset values, and should be confirmed by running them.

## Confirmed findings

**1. Medium. Every planning failure is reported as a collision problem, with advice that can be exactly wrong.**
Where: `pkg/racecraft/racecraft.go:139-143` and `:151-158` discard the solver error and set `valid = false`. The only error returned is `:181`, "no collision-free plan for these placements; increase gap or separation, or reduce overspeed/clearance".
Trigger: default hairpin (half-width 6 m), road car (width 1.8). Body radius is `hypot(4.4, 1.8)/2 = 2.377` m, so the control clearance is 2.627 m and the largest legal offset is 3.373 m. Separation 6.75 gives offsets of ±3.375 m. `ManualOffsets` rejects both cars at `pkg/solver/manual.go:33`, every variant fails identically, and the user is told to increase separation. Separation 6.75 is reachable with three GUI clicks and passes `Config.Validate`. The same message appears for a centreline-infeasible road ("bank/grade exceeds stationary grip"), a road narrower than the body disc, and any road with fewer than about seven samples, where index rounding at `:240` produces duplicate control indices.
The browser check at `tools/browser/racecraft.mjs:29-31` asserts only `/rejected/`, so it passes while confirming the wrong explanation.
Fix: keep the first non-collision error per candidate. Distinguish three outcomes: inputs do not fit the road, solver infeasibility, and collision. Return a wrapped error naming the cause and the station. Consider adding a road-fit check to `Experiment.Validate` in `pkg/racecraft/experiment.go:32-44` so Save also rejects unplannable inputs.
Regression test: plan the over-under example with separation 6.75 and assert the error mentions clearance or the road edge and does not mention collisions. Plan a two-point 8 m scene and assert the error mentions road length.

**2. Medium. B's entry-cap advantage is measured against A's cap at station 0, not against A's speed where the race starts.**
Where: `pkg/racecraft/racecraft.go:146` sets B's cap to `EntrySpeed + Overspeed`. A's path uses the unmodified scene cap at station 0 and A's race start is `AtStation(c.Gap)` at `:161-162`.
Trigger: defend example, gap 14, overspeed 0. Both caps are 38 m/s, but A has 14 m of straight to accelerate before the shared clock starts. A's realized start speed exceeds B's cap, so "0 m/s advantage" is a disadvantage of unknown size that grows with gap. The docs at `docs/RACECRAFT.md:20-22` only warn that the solver can brake below a cap, not that A accelerates above the label.
Fix: either apply A's cap at its start station, or report A's realized start speed next to the overspeed control and in the CLI summary line at `internal/cli/racecraft.go:116`, and state the coupling in the docs.
Regression test: for the defend defaults assert `r.At(0)[0].Speed` versus `Scene.EntrySpeed` and record the expected relation. If the semantics change, assert the difference is bounded.

**3. Low. Documentation and implementation disagree on how the crossover is delayed, and nonuniform sampling changes placements.**
Where: `docs/RACECRAFT.md:75` says "2 / 4 percent of road length". `pkg/racecraft/racecraft.go:231-232, 240` shift sample indices. `pkg/track/track.go:179-185` samples each control span with `ceil(length / 2)` points, so index fraction departs from distance fraction whenever spans differ in length.
Trigger: a scene with one 3 m span followed by one 200 m span. The 3 m span holds two of the samples, so all seven fractions land roughly 1 percent of road length later than intended, and the give-room delay is distance-dependent.
Fix: map control fractions through cumulative `S` and pick the nearest sample. Correct the user doc.
Regression test: build such a scene, plan, and assert each control's station is within one sample spacing of `fraction * total length`.

**4. Low. The certified clearance figure is a loose bound whose value depends on subdivision structure, not geometry.**
Where: `pkg/racecraft/collision.go:35-38` records `lower - radii` for each accepted interval.
Trigger: a wide interval accepted at the top of the recursion contributes `min(d0, dD) - V*D/2 - radii`, which can be near the requested clearance even when the cars never come within metres. Two plans with identical geometry but different speeds report different "certified" gaps. The panel at `pkg/render/racecraft.go:48` shows this as the headline body gap.
Fix: report the sampled minimum alongside the certified bound, or refine accepted intervals until their bound is within a fixed tolerance of the endpoint minimum before recording.
Regression test: assert `MinClearance <= sampled minimum clearance` on every example, and assert the two differ by less than some tolerance if the bound is tightened.

**5. Low. Cycling the scenario discards a custom road and vehicle without notice.**
Where: `main/gui/racecraft.go:115-122` replaces the entire experiment with `Example(next)`.
Trigger: start with `--race-file` pointing at a custom scene, press Next. The scene, vehicle and all four controls revert to the preset.
Fix: replace only `Config` with `DefaultConfig(next)` and keep scene and vehicle, or say so in the status line.
Regression test: browser check that loads a saved experiment with a modified gap, presses Next, and asserts the scene points are unchanged.

**6. Low. Startup flags interact silently.**
Where: `main/gui/main.go:185` starts race mode whenever `--race-file` is set, regardless of `--mode`. `:193-195` overwrite the loaded experiment's vehicle with the qualifying editor's vehicle when `--vehicle` is set.
Trigger: `--race-file x.json --vehicle gt` plans and then saves a GT car into a file that stored the road car. The flag help at `:113-116` does not mention either effect.
Fix: only override when `--vehicle` is explicitly meant for the race, or reject the combination. Make `--race-file` set the path only unless `--mode racecraft` is also given, or document the implied mode.
Regression test: extract startup experiment selection into a function and unit-test the four flag combinations.

**7. Low. Control display precision hides the separation step.**
Where: `pkg/render/racecraft.go:44` prints `%.1f`. Separation steps by 0.25, so 6.25 shows as 6.2 and 6.75 as 6.8.
Fix: `%.2f` for separation and clearance.
Regression test: render a frame with separation 6.25 and assert the panel text.

**8. Low. Planning cannot be cancelled and blocks mode exit.**
Where: `main/gui/racecraft.go:32` uses `context.Background()`. `:70-73` refuses to leave race mode while busy.
Trigger: a long custom road at 0.5 m evaluation with several candidates. The user can only wait or quit. `Plan` already accepts a context, so this is wiring only.
Fix: store a cancel function, cancel on `race-mode` exit and on a superseding edit, and ignore stale replies.
Regression test: unit test that a cancelled context returns promptly from `Plan` mid-candidate, which already exists; add a GUI-level check that a second edit supersedes the first.

**9. Low. Silent key swallowing in race mode.**
Where: `main/gui/racecraft.go:152-153` returns true for any unrecognised action, so undo, redo, new, closed, arrows, delete and add do nothing with no status.
Fix: set a status such as "Qualifying controls are inactive in race mode" for known qualifying keys.

**10. Low. The initial leader is always A.**
Where: `pkg/racecraft/racecraft.go:255`. `Config.Validate` allows gap 0, and gap 0 with separation 6 is collision-free.
Trigger: gap 0, side by side. B finishes 4 m ahead and the summary reports zero completed passes and "A" as the leader of record. The nose-ahead readout is correct, but the event summary is misleading for gaps under one body length.
Fix: initialise the leader from the sign of the initial gap when its magnitude exceeds the hysteresis, otherwise record "undecided" until the first threshold crossing.
Regression test: synthetic straight-line paths with gap 0 where B pulls 5 m ahead. Assert one pass event with leader B.

**11. Low. Latent panic if scenarios and placements diverge.**
Where: `Config.Validate` at `pkg/racecraft/racecraft.go:75-81` accepts anything in `Scenarios()`, while `placements` at `:190-227` has no default case. A name added to one list but not the other leaves `values` nil and `:236` indexes it.
Fix: return an error from `placements` for unknown scenarios and let `Plan` propagate it.
Regression test: table-drive `Scenarios()` through `placements` for both cars and assert seven controls each.

**12. Low. Test coverage stops at the four presets.**
Where: `pkg/racecraft/racecraft_test.go`, `pkg/racecraft/collision_test.go`, `internal/cli/racecraft_test.go`, `tools/browser/racecraft.mjs`.
Gaps I confirmed by reading:
- No test plans a custom, short, narrow, banked or nonuniformly sampled road, or a vehicle other than "road".
- No test checks first-finish semantics. Nothing asserts that at `Duration` one car is at its own endpoint and the other is at its simultaneous position, not parked.
- No test asserts the lower-bound property `MinClearance <= sampled minimum`.
- No synthetic event test for hysteresis, repass labelling or the 0.02 s quantisation.
- No test exercises the 250,000-interval budget or the 1 µs floor, for example a grazing tangent contact that must be rejected.
- CLI tests do not cover `--scene`, `--vehicle`, `--experiment` with `--scenario`, or `--time` beyond the duration.
- Browser tests do not cover a failed load, scenario cycling from a custom experiment, startup flags, or entering race mode from the perspective view and returning.

## Unverified suspicions, verified strengths and optional improvements

**Suspicions I could not confirm without execution.**
- `main/gui/racecraft.go:59-63`: if `rebuild` fails on entering race mode, `g.race` is restored to nil but `g.opts.view` may already have been changed from "perspective" to "3d" while the live renderer is still the perspective renderer. `render.New` only fails on size or camera validation, so I could not construct a realistic trigger.
- Event labels at `pkg/render/racecraft.go:104` are drawn at `x - 65`, which will clip for an event in roughly the first 1 percent of the run and overlap for two events within about a second. No preset produces either case.

**Verified strengths.**
- The continuous separation proof matches the interpolation exactly, including inserted triangle-crossing nodes, and rejection is the default on budget exhaustion or cancellation.
- Road clearance is consistent end to end: `Margin = radius - Width/2 + 0.25` at `racecraft.go:121` makes the solver clearance `radius + 0.25`, the same value passed to `ManualOffsets`, and the test at `racecraft_test.go:74` independently checks 0.2499 against every side segment.
- The initial gap is exact because `AtStation` and `At` share the same constant-acceleration kinematics.
- GUI edits, loads and scenario changes plan on a copy and commit only on success. Rejected edits keep the clock, the camera and the last result. Native saves are atomic and JSON decoding is strict with a size bound.
- Duration is the first finish, so no car is ever rendered beyond its certified trajectory.
- The between-frame tunnelling test is a genuine adversarial case with safe endpoints.

**Optional improvements.**
- Successful race edits reset the clock to zero at `main/gui/racecraft.go:57`, whereas qualifying edits preserve the playback fraction at `main/gui/setup.go:245-250`. Preserving the fraction would make gap tweaks at the crossover easier to compare.
- The CSV column `body_clearance_m` is the sampled physical gap while the panel shows the certified bound. Naming both consistently would avoid confusion.
- `--duration` is accepted but ignored for PNG output in `internal/cli/racecraft.go`.
- Shift with comma or period is labelled "station step" in qualifying but acts as a frame step in race mode at `main/gui/racecraft.go:100-107`.
