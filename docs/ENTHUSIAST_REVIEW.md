# Enthusiast review: the next useful release

Reviewed 2026-09-09 as a fresh reviewer after real-input headless verification. This review makes product and engineering recommendations; it does not certify unimplemented features or vehicle realism.

## Decision

Build **an interactive corner-analysis workbench** next: orbit and inspect the road, watch the optimized car against a centreline ghost, and scrub a shared-distance speed comparison to discover where time is gained. Deliver those three connected capabilities before adding another vehicle model or more track presets.

As a car enthusiast and racing-game player, I want to answer: “Where did this line win, what speed did it carry, and how does the road shape explain that?” The current studio shows a convincing road and a final time saving, but leaves that explanation mostly to imagination. Camera control directly addresses the reported inability to rotate. A ghost and synchronized telemetry make the existing mathematics tangible without pretending to simulate steering feel or actual F1 cars.

## Evidence reviewed

- Read `AGENTS.md`, `README.md`, both research critiques, the vehicle envelope, solver geometry/profile/result APIs, renderer projection, and GUI input/edit dispatch.
- Inspected `artifacts/browser/plan-released.png` and `artifacts/browser/elevated-retina-animation-b.png`. Both show a readable, coherent studio. The elevated ribbon communicates bank and height, but its fixed projection cannot reveal every corner equally well. The broad central viewport has room for a compact analysis strip if laid out deliberately.
- Read `artifacts/browser/report.json`: both configurations list all 12 completed input checks, including drag, undo/redo, cancellation, rejected edits, scrub capture and advancing playback. The configurations are 1440×900 plan at DPR 1 and 1000×700 elevated at DPR 2. This reviewer inspected existing evidence and did not rerun it.
- The code already calculates and verifies a complete centreline baseline, then exports only its duration and endpoint speeds. Retaining that trajectory is a small, valuable extension. `Node.S` measures each candidate's own path length; it cannot align two different lines at the same road location.
- The rendered line colour scale is relative to each run's minimum/maximum speed. Comparisons need numeric axes or a shared scale, otherwise similar colours can represent very different speeds.

## Exact implementation order

### 1. Make the camera controllable without disturbing editing

Implement an orthographic camera with yaw, elevation, zoom and pan in `pkg/render`. Keep camera state in the presentation layer, separate from scene geometry, undo history and solver inputs. Use one projection definition and its constant-height inverse for rendering and handle dragging. Replace the fixed transform and recompute ordering where road depth changes with camera yaw.

Use these explicit controls:

- Right-drag in the road viewport or Alt+left-drag: orbit in elevated view.
- Middle-drag or Shift+left-drag in the viewport: pan in either view.
- Wheel over the viewport: bounded zoom.
- A visible **Fit / Reset camera** button: restore a useful view.
- Keep ordinary left-drag on numbered handles for geometry and Tab for plan/elevated mode. Put a concise camera hint beside the viewport controls.

Give each gesture exclusive ownership from press to release. UI controls take priority; camera gestures must not start over the sidebar or timeline. Cancel camera gestures on Escape, focus loss, view change or canvas departure. Clamp elevation away from a grazing view so the inverse used for ground-plane dragging stays well conditioned. Preserve camera position across a successful solve and ordinary edits; reset when explicitly requested or when loading a different sequence. Avoid refitting continuously during orbit, which makes the road appear to breathe in size.

Acceptance:

- A user can rotate the banked road through a full yaw revolution, inspect its other side, zoom, pan and reset using actual browser inputs.
- Under rotated and zoomed cameras, a handle drag still follows the cursor at fixed world elevation and produces exactly one undoable edit.
- Camera-only actions leave scene, trajectory digest, solver count and undo/redo stacks unchanged. Orbit/pan do not accidentally move the selected point.
- Analytical projection/inverse checks cover multiple yaw/elevation combinations, world heights, both display sizes and letterboxing. Real-input checks cover the modified projection at DPR 2, cancellation and a subsequent fresh drag.
- Both views remain readable in CLI exports without display initialization. Camera rebuilds should reuse typography/resources where possible; measure orbit responsiveness instead of assuming repeated renderer construction is cheap.

### 2. Retain a real baseline and a common road coordinate

Extend the solver result with the already verified centreline nodes. Add a clearly named road-station coordinate to each trajectory node while preserving `S` as actual three-dimensional path distance. Carry road station through original stations and inserted triangle-diagonal crossings. Specify the coordinate's units and interpolation in public API comments and export documentation.

Provide a small display-independent interpolation helper for querying a trajectory at a road station. Position and reference station vary with segment distance; elapsed time and speed must follow the represented constant-acceleration kinematics. Reuse the stable launch/stop behavior already expected by `Result.At`. Do not align curves by node index, separate path metres, or percentage of each path's length.

Retain the baseline produced by the same final verification pass as the winner, with identical vehicle, surface, clearance and requested endpoint caps. No second search is needed. Define the displayed delta as **optimized elapsed time minus centreline elapsed time at the same road station**: negative means ahead. Keep this sign consistent everywhere.

Acceptance:

- Both trajectories start and end at the same road-station bounds, and station is strictly increasing through inserted crossings.
- Straight constant-acceleration and braking fixtures independently establish station-to-time interpolation, including zero entry or exit speed. Endpoint queries clamp consistently.
- Baseline duration equals its last node time and existing `CenterDuration`; the terminal delta equals `Duration - CenterDuration`.
- With search disabled, baseline and selected trajectories coincide and the full delta trace is zero within numerical tolerance.
- The retained baseline passes the existing independent surface, clearance, cap and force checks. Existing optimized times and refinement behavior remain unchanged.
- JSON/CSV field additions are documented and public examples updated where relevant. Existing serialized field meanings stay intact.

### 3. Expose the comparison as one coherent experience

Add a compact speed-versus-road-distance chart with optimized and centreline traces, a current-position cursor and a signed delta readout. Give both traces a shared numerical speed scale and distinguish them by line style as well as colour. Clicking or dragging the chart selects a road station, translates it to optimized time and pauses playback, using the same gesture-capture rules as the timeline. Keep the existing timeline for elapsed-time seeking.

Add a toggleable outlined centreline ghost. At a shared elapsed time, query each trajectory independently; stop the faster car at its endpoint until both finish, then restart the open sequence. Label it **Centreline reference**. Explicitly distinguish the same-time ghost from the same-station delta. With comparison enabled, extend the timeline to the longer duration and mark the optimized finish explicitly. With comparison disabled, retain the current optimized-duration loop. Clamp the current time when switching back to the shorter timeline.

Add playback rates 0.25×, 0.5×, 1× and 2× with a visible control. Slowing down a linked corner sequence is more useful here than decorative motion effects. Show a small vehicle summary using existing configuration values: power-to-weight, drivetrain and tyre-grip multiplier, labelled **illustrative vehicle**. Show requested entry/exit caps and actual optimized/reference endpoint speeds in the comparison panel or a discoverable details view. This makes vehicle switching and line improvements interpretable even when the two runs realize different entry states.

Keep telemetry honest: the current longitudinal value is net acceleration, so label it **Longitudinal acceleration**. Do not display throttle/brake percentages, gear, RPM, understeer, oversteer, tyre temperature, or “grip remaining” inferred from acceleration alone. A negative net acceleration can arise from drag or grade. Those features require additional state and a stated control/force interpretation.

Acceptance:

- At the entry, middle and exit of the esses, chart cursor, car location, speed and signed delta agree with the public trajectory query APIs.
- The ghost follows the retained centreline surface in both views and never uses optimized offsets. It remains identifiable when the two lines overlap.
- Chart dragging owns the gesture outside the strip, clamps endpoints, pauses playback and cannot select or drag a road handle. Camera gestures cannot scrub it.
- Playback advances by the selected simulated rate; pause, restart and comparison-loop behavior are deterministic. Finish handling never implies a periodic lap or an impossible connecting segment.
- Switching vehicle or accepting an edit updates both traces, the ghost and summary atomically after solving. A rejected edit preserves the entire previous comparison and history.
- At 1000×700/DPR 2 the chart labels, legend, controls and endpoint conditions remain readable without overlapping the road or sidebar. CLI PNG and GIF exports show the same comparison composition.

## Release boundaries and Go structure

Keep the numerical envelope unchanged for this release. The accepted model is quasi-static, uses fixed axle loads and torque split, and has a documented 2% fixture refinement tolerance. New insight should be derived from those validated trajectories. A centreline comparison is an identical-cap optimization comparison, not automatically a race between identical initial states; make that distinction visible where the comparison is explained.

Use focused files for camera/projection, trajectory queries and comparison drawing instead of extending the two already large GUI/render files indefinitely. Keep pure numerical queries in `pkg/solver`; camera and chart drawing belong in `pkg/render`; Ebitengine input translation belongs in `main/gui`; edit history remains in `internal/editor`. Add no general-purpose event bus, dependency-injection framework or external chart library for these changes. Validate finite camera/query inputs and return descriptive errors where inputs can fail.

Defer real F1 vehicles, aero/downforce, load transfer, drifting, full laps, manual driving, sound, custom-car tuning and a pinned previous-edit ghost. They can become valuable releases later, but each adds separate physics, fairness, persistence or interaction requirements. The current release should make the existing three fictional cars and five sequences rewarding to explore first.

## Release gate

Run `go test ./...`, `go vet ./...`, and `make build`. Render and visually inspect both views. Extend `make verify-headless` with actual orbit/pan/zoom, transformed handle dragging, chart scrubbing, playback rate and ghost-finish checks; preserve all 12 existing checks in both configurations. Inspect both browser captures and the JSON report. Keep generated evidence under ignored `artifacts/` and do not open native windows for this work. If the scripted 240-frame Ebitengine demo is exercised in the headless harness, inspect its action report for `ERROR` entries as well.

Record commands actually run, measured interaction results and any remaining limitations in `docs/VALIDATION.md`. Completion means a user can freely inspect one corner, slow it down, and identify where the line gained time using a visible, mathematically consistent reference.
