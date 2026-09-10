# Model tyre-force instrumentation

Implemented 2026-09-10. This release adds force channels, a live friction-envelope widget, fixed line-color scales, selectable station-chart channels, and model-derived corner markers. It preserves the speed solver's physical assumptions and does not infer pedal percentages, slip or tyre temperature.

The default envelope exposes the exact signed bank/grade/drag force balance and fixed-axle drive/brake capacities it uses internally. The reusable `vehicle.TyreEnvelope` contract permits richer models to provide their own `Forces` and `LongitudinalBounds` behavior. Opaque replacement models can explicitly leave channels unavailable.

Validation run on the implementation:

- `go test ./pkg/vehicle ./pkg/solver -run 'TestTyre|TestOpaque|TestExportedForces|TestMarker' -count=1 -v` passed. Analytical tests distinguish coasting from braking and constant-speed uphill driving, check power and axle clipping, and ensure opaque models do not invent force channels.
- Export/query checks cover hairpin, banked and rally selected and reference trajectories. Every node's capacity/lateral demand matches the model within 1e-9; longitudinal demand reconstructs acceleration plus drag and gravity; utilization remains within the solver tolerance. Interpolated time and station force queries agree.
- Hairpin marker locations remain within 0.5 m when the same chosen line is reevaluated at 0.25 m. A straight has no corner markers.
- `go test ./pkg/render ./internal/cli -count=1` passed. Tests cover both views at 1440×900 and 1000×700, all four modes, fixed scales, shared station cursors, and force-dot agreement. CSV retains the original first ten columns and preserves new force values at float64 precision.
- Rendered and inspected `artifacts/instrumentation-plan.png` (hairpin GT, utilization) and `artifacts/instrumentation-elevated.png` (banked road car, lateral line color and longitudinal chart). The clipped/asymmetric envelope, reference outline, fixed legends and signed chart channels are visible in both views.
- After visual review, the color control and legend moved to a dedicated strip outside the draggable road. The final instrumentation tests assert that no clickable HUD control overlaps the road viewport and that out-of-scale cursor values remain inside the chart. Both images were regenerated and inspected. `go vet ./pkg/vehicle ./pkg/solver ./pkg/render ./internal/cli` passed.

`tools/browser/instrumentation.mjs` supplies real-input checks for both display configurations: all mode controls must preserve the numerical trajectory and clock, then chart inspection at entry/apex/exit verifies the force dot and chart cursor against the displayed channels. It captures all four modes and three inspection stations. The final integrated headless validation record belongs in [VALIDATION.md](VALIDATION.md).

The original roadmap's example “power-limited then speed-cap-limited on the entry straight” is not guaranteed on every preset: an upcoming corner can require braking before maximum speed is reached. The active-bound flag reports the actual model state. Likewise, additional local minima in an optimized profile remain visible; the marker detector does not manufacture one idealized apex per drawn corner. Force channels on archived JSON queries without a live model are interpolated estimates; reevaluate saved line offsets to recover exact force queries.
