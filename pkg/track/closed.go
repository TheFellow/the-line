package track

import (
	"fmt"
	"github.com/TheFellow/the-line/internal/periodic"
	"math"
)

func (s Scene) validateClosed() error {
	if !s.Closed {
		return nil
	}
	if s.Version < 2 {
		return fmt.Errorf("track: closed laps require version 2")
	}
	if len(s.Points) < 3 {
		return fmt.Errorf("track: closed laps need at least three unique control points")
	}
	for i, p := range s.Points {
		for j := 0; j < i; j++ {
			if math.Hypot(p.X-s.Points[j].X, p.Y-s.Points[j].Y) < 1 {
				return fmt.Errorf("track: closed controls %d and %d must be unique and at least 1 metre apart", j, i)
			}
		}
	}
	a, b := s.Points[0], s.Points[len(s.Points)-1]
	if math.Abs(a.Z-b.Z)/math.Hypot(a.X-b.X, a.Y-b.Y) > .5 {
		return fmt.Errorf("track: closing segment grade exceeds 50%%")
	}
	return nil
}

func newPeriodicCenterSpline(points []Point) centerSpline {
	n := len(points)
	s := centerSpline{points: make([]Vec3, n+1), second: make([]Vec3, n+1), spans: make([]float64, n)}
	values := [3][]float64{make([]float64, n), make([]float64, n), make([]float64, n)}
	for i, p := range points {
		s.points[i] = p.Position()
		s.spans[i] = points[(i+1)%n].Position().Sub(p.Position()).Length()
		values[0][i], values[1][i], values[2][i] = p.X, p.Y, p.Z
	}
	for j := range values {
		values[j] = periodic.Second(values[j], s.spans)
	}
	for i := 0; i < n; i++ {
		s.second[i] = Vec3{values[0][i], values[1][i], values[2][i]}
	}
	s.points[n], s.second[n] = s.points[0], s.second[0]
	return s
}

func clubLoop() Scene {
	s := Scene{Version: Version, Name: "Club Loop", Vehicle: "gt", Closed: true, EntrySpeed: 38, ExitSpeed: 55, KerbsCountAsRoad: true}
	for _, v := range [][2]float64{{-125, -65}, {-35, -85}, {65, -75}, {130, -25}, {110, 50}, {40, 85}, {-40, 65}, {-115, 35}} {
		s.Points = append(s.Points, Point{X: v[0], Y: v[1], WidthLeft: 6, WidthRight: 6, Surface: "asphalt", KerbLeft: Kerb{Width: 1, Surface: "wet", Grip: .85}, KerbRight: Kerb{Width: 1, Surface: "wet", Grip: .85}})
	}
	return s
}
