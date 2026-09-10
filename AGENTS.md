# Working on The Line

Use small, idiomatic Go packages, explicit validation/errors, and standard-library tests. Follow the adjacent `fluid` project for Ebitengine integration and keep simulation independent of graphics initialization.

- `pkg/track`: versioned geometry, road sampling, persistence and track presets.
- `pkg/vehicle`: parameters and replaceable force envelope; SI units are documented at the API.
- `pkg/solver`: bounded deterministic optimization and time-indexed trajectories.
- `pkg/racecraft`: open-road two-car tactical planning, complete experiment inputs and continuous body-clearance certification.
- `pkg/render`: display-independent renderer shared by CLI exports and live GUI.
- `internal/editor`: transactional edits and undo/redo.
- `internal/cli`, `main/cli`, `main/gui`: command wiring and interactive presentation.

Read `research/CRITIQUE.md` and `research/IMPLEMENTATION_REVIEW.md` before changing numerical assumptions. Positive bank raises the left edge. Entry/exit speeds are caps. Open sequences are not periodic laps. Report heuristic estimates and actual validation evidence; do not describe synthetic vehicle defaults as calibrated data.

Validate with `go test ./...`, `go vet ./...`, and `make build`. For graphical changes, render both views and inspect the images. Exercise the actual Ebitengine renderer with `--demo --frames 240 --file artifacts/edited.json --capture artifacts/editor.png --report artifacts/editor-report.json`; check the JSON for any ERROR action entries. For actual mouse/keyboard regression checks without opening desktop windows, use `make verify-headless`; inspect both browser captures and `artifacts/browser/report.json`. This is the preferred alternative when native windows would interrupt the user. CLI output must remain usable without a display. Keep generated artifacts under ignored `artifacts/` and binaries under ignored `bin/`.

Numerical tests should use analytical or independently reconstructed invariants: force balance, clearance, kinematics, refinement, and improvement over an identical-condition baseline. Avoid tests that simply duplicate implementation expressions. Keep focused commits and update public examples/docs when serialized fields or controls change.
