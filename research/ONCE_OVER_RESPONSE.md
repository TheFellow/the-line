# Once-over follow-up

This implements the actionable feedback in [ROADMAP_ONCE_OVER.md](ROADMAP_ONCE_OVER.md).
The scope remains an interactive racing-line playground: useful setup experiments,
editable corners, understandable telemetry and attractive playback. These changes
improve that experience without adding suspension, tyre-temperature or race-management
systems outside the current model.

| Review item | Result |
| --- | --- |
| 2.1 Superseded-edit rollback | Each committed edit owns its checkpoint. If B supersedes A and fails, only B is removed; A resumes and its eventual result matches the editor, car and undo history. |
| 2.2 Seed fallback | A typed `solver.SeedError` identifies line-specific failures. Road, car, refined-baseline and cancellation errors do not trigger retries. The GUI uses the graphics-independent `editor.SolveSetup` path, exercised with real solver failures; browser input verifies the wider-car experience. |
| 2.3 Axle root accuracy | Finite constraint residuals let bracketed Newton approach unloading-axle boundaries from either side. An independent 8,000-case grid measures maximum active residual 2.66e-11 m/s² and root error 3.49e-11 m/s². The bounded conservative fallback remains documented. |
| 2.4 Downforce defaults | First enabling downforce in setup seeds aero balance from static front weight when balance is unset. The panel and model docs explicitly identify 0% as all rear. Explicit serialized balances retain their meaning. |
| 2.5 Phantom kerb friction | A taper uses the present kerb's material; zero-width ends cannot lower contact grip. Changes between present materials remain categorical. |
| 2.6 Chart scale | The speed plot fits both complete compatible traces with rounded bounds and headroom, held stable while playing/scrubbing. Force axes and line-color scales remain absolute. |
| 2.7 Digest identity | Road identities include an interpolant version and ignore absent-kerb material. Exact pre-salt identities migrate on load; stale hypotheses stay stale. Three archived-release hashes independently anchor compatibility. |
| 2.8 Global clearance | Legal-edge clearance and kerb contact share spatial indexing and exact swept circular-footprint segment tests, including neighboring cells. Open entry/exit planes remain unwalled. |
| 2.9 Closed-lap convergence | CLOSED_LAPS.md states the 1e-6 m/s change threshold, 40-pass bound and explicit failure behavior. |
| 2.10 Entry braking | A first detected corner can have an entry brake marker when braking already lasts at least 2 m. Straight endpoint stops still do not invent corner markers. |
| 2.11 Empty kerbs | Go 1.24 `omitzero` removes empty kerb objects from saved points. |
| 2.11 CSV tolerance | Headers accept UTF-8 BOM and case variations; geometry failures retain the import prefix. |
| 2.11 Utilization names | `Envelope.LateralUtilization` distinguishes lateral demand from combined `TyreForces.Utilization`. This is a Go source-field rename; persisted force-channel names remain unchanged. |
| 2.11 Search diagnostics | Candidate totals include seed evaluation work. The verified seed is retained directly instead of being evaluated again in the shortlist. Retained successful-search results carry actual iteration counts; failed searches preserve evaluation diagnostics and report their failure in termination text. |
| 2.11 Captured scene | Solver replies carry their captured source scene rather than reading current editor state at delivery. |
| 2.11 Axle roundoff | Closed-form and iterative bounds are inset and checked against direct feasibility, with a bounded correction for shallow boundaries. |
| 2.11 Invalid transfer | Calls bypassing validation with invalid transfer geometry report infeasibility; normal Validate/load/edit paths provide the descriptive error. |
| 2.11 Degenerate samples | Zero/nonfinite arc-length denominators and nonfinite tangents are rejected before interpolation can produce NaNs. |
| 2.11 Legacy-equivalence evidence | Five optimized and five centreline durations are checked bit for bit against an isolated archive of commit 6620b93, in addition to analytical model tests. |

The numerical and GUI work used focused implementation agents. An independent
follow-up review of the track, renderer and solver changes identified an overly
broad seed-error classification; that was corrected and covered by a replacement
model whose coarse baseline passes but whose refined baseline fails. The same
review found no blocking issue in digest migration, shared edge geometry or chart
scaling. Long edge queries now periodically yield and check cancellation.

Final commands, browser results and personally inspected visual artifacts are
recorded in [docs/VALIDATION.md](../docs/VALIDATION.md).
