# Once-over of the laboratory roadmap implementation

Reviewed 2026-09-10 against commit `6620b93`, the final commit of the eight-iteration roadmap from [NEXT_STEPS.md](NEXT_STEPS.md) as recorded in [docs/ROADMAP_RELEASES.md](../docs/ROADMAP_RELEASES.md). This is a post-delivery review: it re-verifies the headline claims independently, then lists concrete defects for the next iteration. Nothing here was fixed by this review; the working tree was left unchanged apart from ignored artifacts.

**Verdict.** The roadmap was delivered in full and the numerical work holds up. Nothing found blocks use of the current build. The remaining defects are GUI state handling under rapid edits, one numerical-method overclaim, a few unphysical defaults and several presentation and consistency rough edges. Items 1 through 4 below are the ones to schedule first.

## 1. Independently verified

- `go vet ./...` passes. `go test -count=1 ./...` passes across all packages in about 70 s. `go test -race` on the concurrent solver tests is clean.
- **Curvature continuity (Iteration 1).** Re-ran `solve --preset esses --vehicle gt --format csv` and analysed adjacent-node curvature. The largest jump fell from 0.0142 to 0.0014 per metre, the largest jumps no longer sit at control-point stations, and the rendered speed trace is smooth. Same-path refinement error dropped from up to 1.222% to at most 0.044%, and the declared tolerance was tightened from 2% to 1%. The release record honestly notes that the proposed 0.0005 per metre near-knot threshold is not met on compound and rally.
- **Evaluate contract (Iterations 3 and 4).** `Evaluate` on a result's own offsets reproduces `Duration` bit-exactly, including on the closed club loop. A zero-budget seeded solve equals the seed evaluation. A full seeded solve was never slower than its seed. Offsets are unambiguously defined on the dense result road; seeds on the coarse search road; a length check rejects mixing them.
- **Closed laps (Iteration 6).** The periodic spline is a true cyclic tridiagonal solve, C2 across the seam. Seam handling in sampling has no gap or double count. The seam trajectory node repeats node 0's physical state with end time and station.
- **Determinism and cancellation (Iteration 8).** Finalists are stored by index and consumed in fixed order with strict less-than; caches are per-run; context is checked inside every long loop; workers are joined on cancel; skipped finalists are filtered.
- **Stale generations (Iteration 3).** Every mutation bumps a generation counter; replies with a stale generation are dropped; provisional results keep the busy flag.
- **Legacy equivalence (Iteration 5).** The legacy branch of `Config.Limits` is arithmetically untouched in the diff; the axle-dynamics gate is false for all three presets and every example car except the aero study.
- **Performance.** Default esses solve wall time is about 1.6 s, consistent with the documented 2.71x speedup. Cold `Evaluate` is documented at 41 ms, above the one-frame target; the release record says so rather than claiming it.
- **Documentation.** README, examples and eleven docs are updated and match the code. The visual matrix and browser reports exist under `artifacts/`.

## 2. Defects for the next iteration, ranked

Each item names the file, the failing scenario and a suggested fix. Severity reflects user impact in the current build.

### 2.1 Medium: rollback reverts the wrong edit when solves are superseded

`main/gui/setup.go` captures `rollback`, `oldResult` and `oldScene` only when no solve is busy (around lines 105 to 110; the same pattern is in `main/gui/manual.go` around lines 184 to 187). Scenario: edit A starts a solve; before any reply, edit B supersedes it and A is cancelled; B's solve fails. The reply handler (around lines 194 to 208) calls A's rollback and restores the result from before A, then reports "edit reverted". A valid edit A is silently lost.

Fix: capture a checkpoint per queued edit, or when superseding, promote the current editor state to the new checkpoint if A had already been committed to the editor. Add a browser scenario: two rapid edits, second one invalid, assert the first survives.

### 2.2 Medium: seed-fallback path in the GUI has no automated coverage

`main/gui/setup.go` around lines 157 to 163 retries `SolveContext` without `opts.Seed` when a seeded solve errors. Only the solver-level behaviour is tested (`pkg/solver/setup_transition_test.go`): the seeded solve errors and an unseeded one succeeds. No GUI test or browser scenario exercises the branch; the commit titled "Verify wider car setup recovers with a fresh racing line" changed only the solver test. The branch also retries the full solve when the failure was unrelated to the seed (for example an infeasible centreline), doubling the cost of a failing solve before the error appears.

Fix: distinguish seed errors from other errors in the solver (a typed or prefixed error already exists: `seed: ...`), retry only on those, and add a browser scenario that widens the car until the old line is infeasible and asserts a fresh solve is displayed.

### 2.3 Medium: axle-model Newton solve degrades to bisection and misses its documented tolerance

`pkg/vehicle/axles.go` lines 229 to 231: `gapDerivative` returns `(+Inf, +Inf)` whenever an axle's lateral demand exceeds its capacity. That is the common binding regime once load sensitivity is enabled (front unloading under drive for RWD; rear unloading under braking with high front bias). With no finite gap on the infeasible side, every Newton step lands in the infinite region and only the upper bracket shrinks, so progress is one halving per two iterations. After the 40-iteration cap the returned bound is 4e-6 to 6.5e-5 m/s² below the true boundary; `docs/VEHICLE_MODEL.md` line 46 states a 1e-10 residual. Results remain feasible and conservative, and `Limits` and `BoundsOn` agree bitwise, so this is not a correctness hole.

Fix: when `|lr| > cr` return the finite residual `|lr| - cr` with derivative `-dr` (and the front equivalent) so Newton converges from both sides. Add a test that asserts the residual at the returned bound is within the documented tolerance across a random grid.

### 2.4 Medium: downforce with default aero balance gives no cornering benefit

`pkg/vehicle/axles.go` lines 168 to 171 split lateral demand by static weight but add downforce by `AeroBalance`, which defaults to 0, so all downforce goes to the rear axle and the front axle's capacity, which binds first, is unchanged. Scenario: GT preset, step Downforce area up, run the banked sweep; apex speed does not change and the user concludes downforce is broken. The GUI seeds `front_brake` to 0.5 on first use (`main/gui/setup.go` line 68) but has no equivalent for aero balance, and the doc table (`docs/VEHICLE_MODEL.md` line 11) does not say that 0 means all-rear.

Fix: seed aero balance to the static front weight fraction when lift area is first made non-zero; state the all-rear meaning of 0 in the setup panel and the doc.

### 2.5 Low: phantom kerb friction where a kerb starts or ends

An absent kerb (`track.Kerb{}`) reports a default friction of 0.8 (`pkg/track/width.go` lines 14 to 22). Two places charge it: the road sampler copies the outgoing knot's kerb surface and grip while blending only width (`pkg/track/track.go` lines 210 to 215), so a ramping kerb mid-segment can carry surface "" and grip 0 yet friction 0.8; and the solver's kerb-contact check indexes any cell where either end has kerb width and charges the minimum of both ends' friction (`pkg/solver/kerbs.go` lines 37 to 40), so a cell running from no kerb to an asphalt-grip kerb is charged 0.8. Conservative, but unphysical and inconsistent with the `GripAcross` helper, which guards on width greater than zero.

Fix: treat zero-width kerbs as contributing no friction in both places; blend surface identity categorically like the road surface.

### 2.6 Low: fixed 0 to 300 km/h chart axis compresses typical traces

`pkg/render/instrumentation.go` line 62 returns a fixed speed scale of 0 to 300 km/h. A 70 to 150 km/h esses trace occupies a thin band and the sawtooth-versus-smooth difference that Iteration 1 fixed is hard to see. Shared scales between A and B are correct; the range should not be fixed.

Fix: choose the scale from the union of both traces plus headroom, rounded to a tidy step, and keep it fixed for the life of a comparison so scrubbing does not rescale. Keep the absolute scales for grip and g channels.

### 2.7 Low: road digest is not salted with the interpolant version

`pkg/track/study.go` lines 106 to 124 emit the legacy digest for symmetric roads so that v1 scenes and their v2 migrations match. Because study persistence landed after the spline change, no study file exists whose offsets were authored on the old Hermite road, so this is not a live bug. It is a fragility: the next change to `SampleRoad` would silently validate stale studies. Separately, the digest hashes full points including kerb metadata (`study.go` lines 126 to 131), and the GUI sets kerb grip regardless of width (`main/gui/authoring.go` line 52), so nudging grip on a zero-width kerb stales a study for a physically unchanged road.

Fix: include an interpolant version constant in the digest input and bump it on geometry-affecting changes; hash only physically meaningful kerb fields (skip surface and grip when width is zero).

### 2.8 Low: hard clearance is per-cell while kerb contact is global

`pkg/solver/geometry.go` lines 77 to 84 check each path point against the edge lines of its own two adjacent cells only; `pkg/solver/kerbs.go` lines 20 to 65 search neighbouring grid cells with radius padding. The uncovered case is a footprint landing in the interior of a non-adjacent edge segment on a road whose edge bends away and back, sub-centimetre at 0.5 m spacing. Practical impact is small, but the two footprint rules differ.

Fix: reuse the kerb-contact spatial index for the hard clearance check so both use the same footprint geometry.

### 2.9 Low: closed-lap convergence tolerance and failure mode undocumented

`pkg/solver/profile.go` line 159 uses a speed change below 1e-6 m/s with a 40-pass bound, and a non-converging lap returns an error (lines 182 to 184). `docs/CLOSED_LAPS.md` lines 16 to 18 say "until convergence" without the number and do not say that failure is explicit.

Fix: document both.

### 2.10 Low: brake marker requires a preceding drive run

`pkg/solver/markers.go` lines 89 to 98 emit a brake marker only after a sustained drive run. A sequence that enters already braking (high entry cap into an immediate corner) gets an apex marker but no brake marker. `docs/INSTRUMENTATION.md` documents the 2 m run length but not this precondition.

Fix: allow a brake marker at the entry when the first sustained phase is braking, or document the rule.

### 2.11 Cosmetic and consistency

- `omitempty` on the kerb struct fields is a no-op in Go, so every saved point serializes `"kerb_left":{},"kerb_right":{}` (`pkg/track/track.go` lines 28 to 29). Use `omitzero` (Go 1.24) or pointer fields.
- CSV import rejects a UTF-8 BOM header with `unknown column "﻿x"` and is case-sensitive (`pkg/track/import.go` lines 31 to 38); geometry failures lose the `import centreline` prefix (line 74).
- Two different quantities are both named `Utilization`: `vehicle.Envelope.Utilization` is lateral-only, `TyreForces.Utilization` is combined. Both are documented but easy to confuse; rename one.
- `Result.Candidates` omits the two seed evaluations, and the seed is evaluated twice in the fine shortlist (`pkg/solver/solver.go` lines 146 to 151, 160, 296 to 300). A retained provisional reports `Iterations: 0` with search-derived candidate counts (`main/gui/setup.go` lines 164 to 172).
- `solvedScene` is taken from the editor at reply time rather than from the scene captured for the solve (`main/gui/setup.go` line 212 versus 119). Safe today because every road or car mutation bumps the generation, but the invariant is implicit. Pass the captured scene in the reply.
- `pkg/vehicle/axles.go` lines 94 to 101: the no-transfer closed form omits the one-ulp inset used by the other branches, so the feasibility check rejects it by one ulp in about 15% of samples. Harmless downstream.
- `pkg/vehicle/axles.go` lines 109, 128, 214: `CGHeight > 0` with `Wheelbase = 0` produces infinite transfer and a zero bound instead of an explicit failure. Only reachable by bypassing `Validate`.
- `pkg/track/track.go` lines 189 to 190: two exactly coincident dense samples would produce a NaN parameter that passes the later guards. Requires an exact cusp; practically unreachable.
- The legacy-equivalence test in `pkg/vehicle/axles_test.go` lines 119 to 134 sets only fields that do not flip the axle-dynamics gate, so it proves the gate rather than legacy equivalence. Bit-for-bit legacy times currently rest on the git diff. Add golden durations for the five original fixtures so future changes are caught.

## 3. Suggested order for the next iteration

1. Fix 2.1 and 2.2 together: they are both GUI orchestration under superseded solves, and one browser scenario can cover both.
2. Fix 2.3 and 2.4 together in `pkg/vehicle`, with the residual test and the balance seed.
3. Fix 2.5 and 2.7 together in `pkg/track` and `pkg/solver` kerb handling and digest salting.
4. Fix 2.6 in the renderer.
5. Sweep 2.8 through 2.11 as a cleanup commit with the documentation updates.

Release gate stays as before: `go test ./...`, `go vet ./...`, `make build`, the headless browser suite in both configurations, and an updated `docs/VALIDATION.md` entry recording what was measured.

## 4. Evidence

- Commands run: `go vet ./...`; `go test -count=1 ./...`; `go test -race ./pkg/solver -run` on the finalist, worker, closed-circle, evaluate and seed tests; `go build ./main/cli`; `solve --preset esses --vehicle gt --format csv`; `render` of the esses plan view and the club-loop perspective view; timing of a default esses solve.
- Curvature and speed-extrema analysis of the exported CSV before and after the spline change (before: largest jump 0.0142 per metre at control-point stations; after: 0.0014 per metre, not at control points).
- Three bounded read-only code reviews of `pkg/vehicle`, `pkg/track` and `pkg/solver` plus `main/gui`, each with throwaway probes through public APIs; no repository files were modified.
- Rendered images inspected: `artifacts/review2-esses-2d.png`, `artifacts/review2-club-persp.png`. Generated files remain under ignored `artifacts/`.
