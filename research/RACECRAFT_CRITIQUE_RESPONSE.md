# Racecraft critique response

The [implementation brief](RACECRAFT_IMPLEMENTATION.md) freezes the original
`a887689` implementation. The [Claude CLI critique](CLAUDE_RACECRAFT_CRITIQUE.md)
was produced against `2e99f2b` using read-only source tools; its numerical
counterexamples were not executed by the reviewer. Three fresh implementation
agents handled planner, export, and GUI corrections. The primary agent reviewed
the patches, added independent collision/event tests, updated presentation and
documentation, and committed each scope separately.

## Finding dispositions

| Claude finding | Disposition and evidence |
| --- | --- |
| 1. All failures blamed on collisions | Fixed. Candidate/car-specific wrapped errors retain placement, road-fit, solver-feasibility or unresolved body-clearance causes. Regressions cover separation 6.75, narrow and short roads, excessive bank on ice, and initially overlapping bodies. Unknown/duplicate controls fail explicitly. `Experiment.Validate` remains structural: saving inputs does not run a potentially expensive planner; loading into playback still requires a certified plan. |
| 2. Cap delta differs from shared-start speed advantage | Clarified while retaining the existing boundary model. The control is now “B entry cap delta”; the panel and CLI summary show realized A/B starting speeds. A has already traveled the gap when the shared clock starts, so changing gap can change A's speed. Tests exercise that relation on an unconstrained custom road with all three vehicle presets. The user guide explains the coupling and cap reductions. |
| 3. Index fractions versus road-distance fractions | Fixed. Each authored fraction selects the nearest cumulative road station. A deliberately nonuniform road puts the old quarter-road control at 29 m instead of its 50.75 m target; the new regression fails against the old implementation. Short roads that cannot represent distinct controls are explicitly rejected. Default event times were remeasured and the GIF regenerated. |
| 4. Loose certified clearance bound | Retained the sound conservative bound and its `>=` label. Added a separate approximate minimum from 401 uniformly spaced shared-time samples to the panel. Tests require the certified bound to be no greater than independently sampled disc clearance on every example and custom-road plan. The sampled value is not a continuous safety guarantee. CSV now calls its instantaneous column `sampled_body_clearance_m`. |
| 5. Next replaces custom road and vehicle | Kept complete-example loading as an intentional action. The button now says “Load next example”; successful loading explicitly reports that road, vehicle and controls were reset. The guide advises saving custom inputs first. Browser coverage loads a translated custom road and modified car/controls, then verifies the complete preset replacement. |
| 6. Startup flag interactions | Documented existing semantics in flag help and the guide: `--race-file` implies racecraft even when `--mode qualifying` is present, and `--vehicle` overrides both the retained qualifying car and the loaded race car. A real-browser startup check loads a saved race with those flags, saves the GT override, then returns to the matching qualifying vehicle. GUI scene/preset flags still initialize the separate qualifying study; a custom race road belongs in the complete experiment. |
| 7. Hidden quarter-metre separation steps | Fixed. Separation and clearance display two decimal places. A 6.25 m separation render was inspected in plan view, together with the elevated pass/repass view. |
| 8. Planning cannot be cancelled | Fixed. Each race plan has a cancellable context and its own buffered reply channel. R cancels pending planning, detaches stale replies and returns to qualifying. A browser regression begins a larger custom plan and exits while busy, checking that neither its result nor later reply replaces the restored study. Edits remain serialized; superseding a running plan with another edit is not required for cancellation and was not added. |
| 9. Qualifying keys silently swallowed | Fixed. Blocked qualifying actions report that qualifying controls are inactive and explain how to return. Browser coverage checks an arrow edit leaves the scene unchanged and displays that status. |
| 10. Initial leader always A | Kept and documented the nominal initial-A convention, including zero gap. The claimed example of B being 4 m ahead with no completed pass is correct under the advertised >4.4 m threshold; the UI separately reports the actual nose-ahead car. There is no exported “leader of record” field falsely claiming A leads physically. A synthetic side-by-side test lets B gain 5 m, then A gain 5 m: it records B's pass and A's repass at independently derived threshold times. Excursions below a body length remain uncounted. |
| 11. Unknown placement scenario could panic | Fixed. The placement switch returns an explicit error for an unknown scenario. Tests cover every listed scenario, both cars and all three candidate variants, plus an unimplemented name. |
| 12. Missing non-preset and boundary coverage | Expanded. Planner tests now cover short/narrow/infeasible roads, a banked nonuniform custom road with all three vehicles, zero/fractional caps, first-finish/clamping and the lower-bound property. Independent tests cover analytic pass thresholds, final-instant sampling, tangent contact, and budget exhaustion. CLI tests cover road/car overrides, conflicting input selectors, invalid animation ranges/budgets, aliases and failed writes. Browser tests cover failed loads, custom preset replacement, perspective return, startup flags, cancellation and asynchronous imports. |

## Additional bugs found while reproducing the critique

1. **A minimum 1 m/s floor violated entry caps.** On a 200 m straight with a zero
   scene cap and zero delta, B's reported cap and speed were both 1 m/s. The floor
   is now zero. Five public-API cases cover standstill, fractional caps, negative
   deltas, and a third-candidate give-room reduction below zero. All five failed
   before the fix. The four default example JSON results were byte-identical
   across this isolated change.
2. **Failed exports could replace saved inputs.** An invalid GIF start time
   previously overwrote `--save-experiment` before returning an error. Export
   validation/encoding now completes before saving inputs. Predictable destination
   failures and aliases are checked first; existing paths, symlinked directories,
   hard links and newly created case aliases have regressions. Saving back to the
   loaded experiment remains supported. Each file replacement is atomic, but the
   pair is not a transaction: a later save failure can leave a successful export.
3. **A delayed CSV chooser could edit the retained qualifying study during race
   mode.** Chooser callbacks now retain their original buffered reply channel;
   entering race mode detaches pending imports. The browser test delays the real
   `File.text()` promise and completes it both during race mode and after a full
   mode round trip. Neither completion edits or queues a solve for the study.
4. **Held gestures could cross the mode boundary.** Accepted mode changes cancel
   pointer/timeline/camera gestures. Actual held-input browser regressions verify
   that moving after returning does not scrub or pan the qualifying study.
5. **Renderer construction could partially commit mode state.** Candidate
   rendering now succeeds before assigning the race, clock, view or retained
   renderer. This removes the partial-rollback path noted as an unverified
   suspicion. No practical user-triggered renderer failure was established with
   valid fixed dimensions/camera, so no artificial failure hook was added.

## Numerical outcomes after the station correction

| Scenario | First finish | Completed changes |
| --- | ---: | --- |
| Over-under | 13.274955 s | B at 12.02 s |
| Pass-repass | 13.234241 s | B at 6.16 s; A at 11.84 s |
| Defend | 12.715516 s | None |
| Esses duel | 15.687264 s | None; nose-ahead advantage trades |

The over-under 3.25 m gap / 5 m separation fixture still selects candidate two;
its first finish is 13.130093 s. Defend and esses results remain byte-identical to
the review baseline. Moving authored controls to their actual road fractions
changes crossover timing without forcing the event detector to produce a winner.
The original 7 m gap no longer suppresses the over-under pass after correcting
station placement. Re-measurement found a completed pass through 10 m and none at
11 m; the browser control regression and user guide now use 11 m.

## Validation record

Focused planner, renderer and CLI tests passed after their respective patches.
The new cap tests and nonuniform-station test were also checked against the old
implementations and failed for their intended reasons. The continuous validator
itself was not changed: tangent and visit-budget tests exercise conservative
rejection, with a nearby resolvable parallel-motion control that must succeed.
The synthetic event test derives its crossing from `10t - 5t² = 4.4` and checks
that observed events are within the 0.02 s sampling interval.

Integrated `go test ./...`, `go vet ./...` and `make build` passed after the code
changes. A `CGO_ENABLED=0` CLI build successfully exported the pass/repass CSV.
Both PNG views were inspected, including a visible 6.25 m separation control.
`go run ./tools/preview` regenerated the 391-frame, 960×600 GIF (1,601,432 bytes);
decoded composited frames from qualifying and both overtaking clips were inspected.
An additional fresh read-only agent audited the response against the planner,
renderer, CLI and GUI patches and found no new actionable correctness issue.
This follow-up was separate from the preserved Claude CLI review of the baseline.

The final real-browser racecraft run passed **20 workflow checks**: ten each in
1000×700 / DPR 2 and 1440×900 / DPR 1. Both cases have no failures or browser
errors in `artifacts/browser-racecraft-review-final/report.json`; their elevated
pass/repass and plan over-under captures were inspected. The changed CSV-authoring workflow also passed **six checks**, three per viewport,
with no failures or browser errors in
`artifacts/browser-authoring-review-final/report.json`. Its import captures were
inspected. These targeted post-review runs cover the changed GUI workflows; the
pre-review full suite passed 86 checks and is retained separately.

## Remaining limits and optional suggestions

The safety proof still encloses each rectangular body in a horizontal disc. It
can reject feasible close passes. The finite authored candidate set is an offline
teaching model and does not infer corners, react online, or model sporting rules.
Event detection is sampled at 0.02 s and can miss shorter threshold excursions;
collision certification is continuous and independent of those event samples.

Successful race edits still restart the shared clock so the changed arrival and
placement can be replayed together. Shift-comma/period remains a time step in
race mode, as documented. PNG exports use `--time`; duration controls GIF output.
Event-label packing for very early or tightly spaced custom events remains a
presentation improvement; no supplied example exhibits the suspected clipping.
The expanded tests establish specific invariants and input interleavings, not
exhaustive coverage of every custom road or operating-system filesystem race.
