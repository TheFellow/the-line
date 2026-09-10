# Two-car racecraft: implementation and review brief

Written 2026-09-10 against `a887689`, the first racecraft implementation in
[PR #1](https://github.com/TheFellow/the-line/pull/1). This document records the
implemented model and its evidence for an independent implementation critique.
The user-facing design and controls are in [RACECRAFT.md](../docs/RACECRAFT.md).
This is the frozen review baseline. See the [Claude CLI critique](CLAUDE_RACECRAFT_CRITIQUE.md)
and [response](RACECRAFT_CRITIQUE_RESPONSE.md) for subsequent changes, including
station-based placement mapping and updated measured event times.

## Question and scope

An empty-road line study cannot explain why a driver sacrifices an entry or exit
when another car occupies useful space. The racecraft experiment asks how two
specified tactical intentions, starting positions and arrival-speed caps change
who comes out ahead through an open corner sequence.

The implementation is an offline planner with a small authored candidate set.
It is not a reactive racing agent or a search for optimal competitive strategies.
Both cars use the same fictional vehicle configuration and the existing
quasi-static force model. No drafting, driver reaction, tyre temperature, contact
response, steering-rate model or sporting rules are added. The earlier
[numerical critique](CRITIQUE.md) and [implementation review](IMPLEMENTATION_REVIEW.md)
remain the contract for the underlying solver.

## Data and package boundaries

- `pkg/racecraft/racecraft.go`: versioned controls, scenarios, planning, time queries
  and completed-pass detection. `Experiment` stores the complete road, car and
  race controls; native persistence is atomic, and browser persistence uses local
  storage with a separate race namespace.
- `pkg/racecraft/collision.go`: graphics-independent continuous separation check.
- `pkg/solver`: evaluates each supplied lateral path and supplies time-indexed
  trajectories; it is unchanged by the initial racecraft implementation.
- `pkg/render/racecraft.go`: the two solid car bodies, fixed identity colors,
  speed/gap charts, pass markers and controls, shared by GUI, PNG and GIF.
- `main/gui/racecraft.go`: asynchronous planning and commit-on-success state
  changes, with the prior qualifying renderer retained across mode switches.
- `internal/cli/racecraft.go`: `race` command, complete input round trips and
  JSON/CSV/PNG/GIF export. `tools/preview` reproduces the README animation.

`Config` version 1 contains a scenario name, starting station gap in metres,
B's entry-cap advantage in m/s, nominal lateral separation in metres, and extra
body clearance in metres. A starts at the requested station gap; B starts at
station zero. Each result retains both complete evaluated paths and A's starting
time within its path. Consequently an exported car node's path time is not
necessarily the shared race time; `Result.At` adds each car's start-time offset.

## Tactical path construction

Each named intention supplies seven sparse lateral controls. Their positions are
currently mapped from fractions of the sampled-road index range, rounded to
integer indices. This is approximately a fraction of road distance on the
supplied roads, but not exactly so when retained geometry/surface boundaries make
sampling nonuniform. `solver.ManualOffsets` expands the controls with the existing
bounded smooth interpolation; `solver.EvaluateContext` checks the path and its
force-limited speed profile on a road refined to at most 0.5 m spacing.

The preferred pair is evaluated first. If it is infeasible or cannot be certified
collision-free, two alternatives move B farther toward its chosen lane by 0.25
and 0.5 m. For crossover scenarios, its two transition controls move later by
2% and 4% of the sample-index range. Its entry cap is also reduced by 2 and 4 m/s.
All altered paths and speed profiles are evaluated again. A's intention is fixed.
The first safe pair is returned; candidates are not ranked by winning margin or
time, and no desired winning car is supplied to the event detector.

The displayed separation is a nominal input: actual lateral separation changes
through the crossover and when a give-room alternative is selected. Entry speeds
are caps, not exact imposed arrival states. Braking feasibility may reduce them,
and give-room alternatives may voluntarily lower B's effective cap. Results
include candidate count, intent text, effective caps and realized trajectory
states; the GUI exposes the realized speeds during playback.

## Full-body safety model

Both car bodies are 4.4 m long with the configured width. Their circumscribed
horizontal disc radius is `hypot(length, width)/2`. The solver's normal half-width
margin is increased so that the entire disc plus 0.25 m clears road sides.
Existing solver clearance checks cover swept path segments against physical side
edges. Open road entry/exit cross sections intentionally do not act as walls.

For two cars, required center distance is `radiusA + radiusB + extraClearance`.
The separation check uses XY distance, so elevation cannot permit an overlap in
plan. A pair of disjoint enclosing discs guarantees disjoint rectangular bodies
at every heading, at the cost of rejecting some feasible close rectangular passes.
The reported minimum clearance is a certified lower bound, not the sampled or
exact minimum physical body gap.

For a time interval `[a,b]`, let `V` be the sum of both paths' maximum node speeds.
The existing interpolation has constant longitudinal acceleration on each
straight path segment, so each segment's speed is bounded by its endpoint speeds.
XY displacement cannot exceed spatial travel. Relative center distance is
therefore Lipschitz with constant at most `V`, including segment transitions.
A lower bound valid throughout the interval is:

```
min(distance(a), distance(b)) - V * (b-a)/2
```

Every time is within half the interval length of one endpoint. If this lower
bound clears the required distance, the interval is accepted. Otherwise the
interval is bisected; actual endpoint overlap rejects it immediately. Unresolved
intervals smaller than one microsecond or exceeding 250,000 interval visits reject
the candidate. Context cancellation is checked during this recursion.

The shared experiment ends when the first car reaches the finish. The other car
remains at its own simultaneous position. This avoids an artificial parked car
at the endpoint and prevents rendering an uncertified continuation. Replay resets
both cars together.

## Race order and presentation

The current nose-ahead car is computed from signed reference-station gap. A
completed pass requires the other car to gain more than one body length in
station, with hysteresis retaining the current completed leader until that
threshold is crossed in the other direction. The initial leader is A. The first
change is labeled a pass and later changes are labeled repasses.

The initial event detector samples shared time every 0.02 s and includes the
final instant. This event sampling is separate from continuous collision
certification. It does not currently compute analytic threshold roots.

The GUI has separate race inputs and a retained qualifying study. A failed race
plan leaves the last accepted result visible. Race mode disables qualifying edits
through its action handler. Camera, playback and time-scrubbing controls remain
available. Saving persists complete inputs, not computed result caches. The
qualifying renderer (including camera and analysis channels) is restored when
returning from race mode. The native command-line entry point can start directly
in race mode or load an experiment.

## Measured evidence at the reviewed commit

Default road car and scenario inputs:

| Scenario | Completed order changes | First finish |
| --- | --- | ---: |
| Over-under | B at 12.92 s | 13.280 s |
| Pass-repass | B at 6.18 s; A at 12.70 s | 13.239 s |
| Defend | None; A remains ahead | 12.716 s |
| Esses duel | None; nose-ahead advantage trades | 15.687 s |

Increasing the over-under starting gap from 6 to 7 m prevents a completed pass
before the finish. A 3.25 m gap with 5 m nominal separation requires the second
candidate, providing evidence that opponent occupancy can alter the selected
path and arrival cap. The first three examples share the same hairpin geometry.

`go test ./...`, `go vet ./...` and `make build` passed. Racecraft tests independently
reconstruct distance/time kinematics, query the public vehicle force envelope,
measure swept segment-to-segment road clearance, and densely sample car separation.
A crossing inside a 10 ms interval with safe endpoints must fail certification.
Other checks cover deterministic output, parameter effects, fallback selection,
invalid inputs, cancellation, strict persistence and failed-output preservation.
Existing solver tests provide the broader analytical force and refinement oracles.

The complete real-browser Ebitengine run passed 86 workflow checks across plan
and Retina elevated configurations. A focused rerun of the final racecraft code
passed six additional checks. The race tests cover mode restoration, controls,
unsafe-edit rejection, save/load, scenario changes, shared-time scrubbing, frame
stepping, restart and both views. They do not constitute proof of every possible
input interleaving or parameter combination. The 391-frame README GIF was decoded
and inspected across qualifying, over-under and pass/repass clips.

## Independent review request

Read the code as well as this document. Check whether the safety argument matches
the actual interpolation, whether inputs and boundaries retain their advertised
meaning, whether limits are bounded and failures remain visible, whether UI state
transitions are transactional, and whether exports and tests support their claims.
Consider custom roads, nonuniform sampling, different vehicle configurations,
extreme but accepted controls, cancellation and persistence paths.

Separate reproducible bugs and missing invariants from optional future features.
For each finding provide severity, precise source locations, a trigger or a
reasoned counterexample, a practical fix, and a regression test. Record what was
actually inspected or executed. Explicitly distinguish unverified suspicions from
confirmed findings, and do not treat the documented offline tactical scope as a
promise of an online AI driver. The critique will be preserved separately and
addressed with fresh implementation agents and focused commits.
