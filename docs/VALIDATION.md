# Validation record

Measured on 2026-09-09 with Go 1.24.0, macOS/amd64, Intel Core i5-1038NG7. All values refer to the implemented synthetic, quasi-static vehicle model, not real-car measurements.

## Build and headless operation

- `make build`: both executable binaries built successfully.
- `go test ./...`: all packages passed, including CLI image/animation workflows, editor transactions, analytical physics and independent trajectory verification.
- `go vet ./...`: passed.
- `CGO_ENABLED=0 go build -o bin/the-line-headless ./main/cli`: passed. The CLI has no graphics-driver initialization or native game-engine requirement.
- CLI creation → validation → JSON/CSV solve → PNG in both views → timed GIF passed in `internal/cli`. Failed exports preserve existing files.

## Solver results

Default vehicle for each scene, 3 m search spacing, 0.5 m final validation spacing, four search sweeps, 0.25 m additional clearance:

| Sequence | Centreline | Verified line | Improvement | Same-path 0.5 → 0.25 m time change |
| --- | ---: | ---: | ---: | ---: |
| Hairpin | 13.3835 s | 12.8881 s | 3.70% | 1.222% |
| Esses | 17.3632 s | 15.0346 s | 13.41% | 0.958% |
| Compound | 15.1315 s | 14.1882 s | 6.23% | 1.038% |
| Banked | 10.6991 s | 10.2551 s | 4.15% | 0.370% |
| Rally | 24.0995 s | 23.1780 s | 3.82% | 0.574% |

The independent matrix covers all 15 track/vehicle pairings, at one search sweep. It checks exact segment clearance to road sides, triangle surface heights, endpoint speed caps, finite values, time/acceleration kinematics, and 129 independent force-balance evaluations per output segment. All passed; maximum longitudinal force excess was 0.000000976 m/s². See [the independent implementation review](../research/IMPLEMENTATION_REVIEW.md) for method and caveats.

The initial research target was less than 1% refinement error. Measured same-path changes reach 1.222%; the implemented regression tolerance is explicitly 2%. Searching at a different coarse resolution can discover a different local solution. Neither test establishes a global optimum. A default esses solve benchmark took 2.13 s with 20.4 MB allocated; solving happens off the UI thread.

## Rendered artifacts and live execution

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
