package track

import (
	"math"
	"testing"
)

func closedCircle() Scene {
	s := Scene{Version: Version, Name: "circle", Closed: true}
	for i := 0; i < 12; i++ {
		a := 2 * math.Pi * float64(i) / 12
		s.Points = append(s.Points, Point{X: 80 * math.Cos(a), Y: 80 * math.Sin(a), Width: 10, Surface: "asphalt"})
	}
	return s
}
func TestPeriodicSplineContinuity(t *testing.T) {
	s := closedCircle()
	for _, fixture := range []Scene{s, clubLoop()} {
		fixture.Points = append([]Point(nil), fixture.Points...)
		for i := range fixture.Points {
			fixture.Points[i].Z = 4 * math.Sin(float64(i))
		}
		curve := newPeriodicCenterSpline(fixture.Points)
		for i := range fixture.Points {
			p, d, dd := curve.at(i, 1)
			q, e, ee := curve.at((i+1)%len(fixture.Points), 0)
			if p.Sub(q).Length() > 1e-10 || d.Sub(e).Length() > 1e-10 || dd.Sub(ee).Length() > 1e-10 {
				t.Fatalf("C2 discontinuity at %d: %g %g %g", i, p.Sub(q).Length(), d.Sub(e).Length(), dd.Sub(ee).Length())
			}
		}
	}
	road, err := SampleRoad(s, .5)
	if err != nil {
		t.Fatal(err)
	}
	a, b := road[0], road[len(road)-1]
	b.S = 0
	if a != b {
		t.Fatal("lap seam sample differs")
	}
	if math.Abs(road[len(road)-1].S-160*math.Pi)/(160*math.Pi) > .001 {
		t.Fatal("circle spline length outside 0.1% oracle tolerance")
	}
}
func TestClosedValidationAndIdentity(t *testing.T) {
	s := closedCircle()
	open := s
	open.Closed = false
	if RoadDigest(s) == RoadDigest(open) {
		t.Fatal("open and closed digests match")
	}
	s.Points = append(s.Points, s.Points[0])
	if s.Validate() == nil {
		t.Fatal("repeated source seam accepted")
	}
	s = closedCircle()
	s.Points = s.Points[:2]
	if s.Validate() == nil {
		t.Fatal("two-point lap accepted")
	}
}
