package main

import (
	"image"
	"math"

	"github.com/TheFellow/the-line/pkg/render"
	"github.com/hajimehoshi/ebiten/v2"
	"github.com/hajimehoshi/ebiten/v2/inpututil"
)

type cameraGesture struct {
	start        render.Camera
	x, y         float64
	lastX, lastY float64
	button       ebiten.MouseButton
	orbit        bool
}

// cameraMouse owns its gesture until release, even after leaving the road area.
// The shared cancellation path handles focus loss, Escape and canvas departure.
func (g *game) cameraMouse(x, y float64) bool {
	if g.opts.view == "perspective" {
		return false
	}
	if g.drag != nil || g.manualDrag != nil || g.scrubbing || g.charting {
		return false
	}
	if x < 0 || y < 0 || x >= float64(g.opts.width) || y >= float64(g.opts.height) {
		if g.cameraDrag != nil {
			g.cancelPointer()
		}
		return true
	}
	left := inpututil.IsMouseButtonJustPressed(ebiten.MouseButtonLeft)
	right := inpututil.IsMouseButtonJustPressed(ebiten.MouseButtonRight)
	middle := inpututil.IsMouseButtonJustPressed(ebiten.MouseButtonMiddle)
	if g.pointerCancelled {
		if left || right || middle {
			g.pointerCancelled = false
		} else if ebiten.IsMouseButtonPressed(ebiten.MouseButtonLeft) || ebiten.IsMouseButtonPressed(ebiten.MouseButtonRight) || ebiten.IsMouseButtonPressed(ebiten.MouseButtonMiddle) {
			return true
		}
	}
	if g.cameraDrag != nil {
		drag := g.cameraDrag
		c := drag.start
		displayScale := math.Min(float64(g.opts.width)/1440, float64(g.opts.height)/900)
		dx, dy := (x-drag.x)/displayScale, (y-drag.y)/displayScale
		if drag.orbit {
			c.Yaw += dx * .008
			c.Elevation += dy * .005
		} else {
			c.PanX += dx
			c.PanY += dy
		}
		if (x != drag.lastX || y != drag.lastY) && c != g.renderer.Camera() {
			if err := g.renderer.SetCamera(c); err != nil {
				g.recordError(err)
			}
		}
		drag.lastX, drag.lastY = x, y
		if inpututil.IsMouseButtonJustReleased(drag.button) || !ebiten.IsMouseButtonPressed(drag.button) {
			g.cameraDrag = nil
		}
		g.hover = -1
		return true
	}
	if !image.Pt(int(x), int(y)).In(g.renderer.RoadViewport()) {
		return false
	}
	pan := middle || left && ebiten.IsKeyPressed(ebiten.KeyShift)
	orbit := right || left && ebiten.IsKeyPressed(ebiten.KeyAlt)
	if pan || orbit {
		if orbit && !pan && g.opts.view != "3d" {
			g.setStatus("Orbit is available in elevated view · Tab switches view")
			return true
		}
		button := ebiten.MouseButtonLeft
		if middle {
			button = ebiten.MouseButtonMiddle
		} else if right {
			button = ebiten.MouseButtonRight
		}
		g.cameraDrag = &cameraGesture{start: g.renderer.Camera(), x: x, y: y, lastX: x, lastY: y, button: button, orbit: orbit && !pan}
		g.hover = -1
		return true
	}
	_, wheelY := ebiten.Wheel()
	if wheelY != 0 {
		c := g.renderer.Camera()
		// Browser wheels can report tens of pixels per event while native wheels
		// report steps. Bound each update before applying the zoom sensitivity.
		c.Zoom *= math.Exp(math.Max(-4, math.Min(4, wheelY)) * .12)
		if err := g.renderer.SetCamera(c); err != nil {
			g.recordError(err)
		}
		return true
	}
	return false
}
