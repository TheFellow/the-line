# The Line

A racing-line playground for curious driving enthusiasts. Reshape a corner, tweak the car, and watch how the line, braking points and lap time change. Explore twelve fictional tracks, from hairpins and rally sections to a continuous club circuit.

![A GT car and its reference ghost lapping the club circuit, with a speed chart below](docs/media/club-loop.gif)

## Get running

Install [Go 1.24 or later](https://go.dev/dl/) and Git. You’ll need a graphical desktop.

<details>
<summary>Platform setup: macOS and Linux</summary>

On macOS, install Apple’s command-line tools if you haven’t already:

```sh
xcode-select --install
```

On Ubuntu or Debian, install the build and graphics dependencies:

```sh
sudo apt-get update
sudo apt-get install build-essential libgl1-mesa-dev libx11-dev libxcursor-dev libxi-dev libxinerama-dev libxrandr-dev libxxf86vm-dev
```

Other Linux distributions need the equivalent OpenGL and X11 development packages.

</details>

Then open a terminal and run:

```sh
git clone https://github.com/TheFellow/the-line.git
cd the-line
go run ./main/gui
```

The first launch downloads dependencies and builds the app. It opens on the esses, ready to edit.

To start on the circuit shown above:

```sh
go run ./main/gui --preset club-loop --vehicle gt
```

## Try this first

1. **Reshape a corner.** Drag a numbered road handle and release. The line recalculates; use the sidebar to change width, banking or elevation.
2. **Change the car.** Open **Setup** and try more power, different grip or extra weight. Watch where the new setup gains or loses time.
3. **Compare your changes.** Click **Pin current** before a setup edit to keep the old run as a reference ghost. Drag the speed chart to inspect a corner; a negative time delta means your current line is ahead.
4. **Try your own line.** Click **Author line**, then drag the cyan handles. See how your choice compares with the computed line.
5. **Sit back and watch.** Press **Space** to pause or resume, **Tab** to switch views, or **Watch** for the trackside camera. **Next scene** cycles through the tracks.

Scroll to zoom, Shift-drag to pan, and right-drag to orbit the elevated view. **Fit / Reset** brings the track back into view.

Use **Cmd/Ctrl+Z** to undo. Click the file path to choose a save location, then **Cmd/Ctrl+S** to save your study or **Cmd/Ctrl+O** to load it.

For more, see [line studies](docs/LINE_STUDIES.md), [car setup](docs/SETUP.md), and [track editing](docs/TRACK_AUTHORING.md).

The cars and tracks are fictional, and the results are estimates from a simplified vehicle model. See [model assumptions](docs/VEHICLE_MODEL.md) for the details.
