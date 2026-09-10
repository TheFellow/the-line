package track

// centerSpline is a natural cubic through every control point. Its independent
// parameter is cumulative 3D chord length; spans are strictly positive after
// Scene.Validate. Natural end conditions set second derivatives to zero.
type centerSpline struct {
	points, second []Vec3
	spans          []float64
}

func newCenterSpline(points []Point) centerSpline {
	n := len(points)
	s := centerSpline{points: make([]Vec3, n), second: make([]Vec3, n), spans: make([]float64, n-1)}
	for i, p := range points {
		s.points[i] = p.Position()
		if i > 0 {
			s.spans[i-1] = s.points[i].Sub(s.points[i-1]).Length()
		}
	}
	// Thomas elimination of the positive-definite tridiagonal continuity
	// equations, sharing scalar coefficients for all three coordinates.
	upper := make([]float64, n)
	rhs := make([]Vec3, n)
	for i := 1; i < n-1; i++ {
		left, right := s.spans[i-1], s.spans[i]
		pivot := 2*(left+right) - left*upper[i-1]
		upper[i] = right / pivot
		difference := s.points[i+1].Sub(s.points[i]).Mul(1 / right).Sub(s.points[i].Sub(s.points[i-1]).Mul(1 / left))
		rhs[i] = difference.Mul(6).Sub(rhs[i-1].Mul(left)).Mul(1 / pivot)
	}
	for i := n - 2; i > 0; i-- {
		s.second[i] = rhs[i].Sub(s.second[i+1].Mul(upper[i]))
	}
	return s
}

// at evaluates position and the first two derivatives with respect to chord
// length. The segment-local fraction t is in [0, 1].
func (s centerSpline) at(segment int, t float64) (Vec3, Vec3, Vec3) {
	h := s.spans[segment]
	a, b := 1-t, t
	p, q := s.points[segment], s.points[segment+1]
	m, n := s.second[segment], s.second[segment+1]
	position := p.Mul(a).Add(q.Mul(b)).Add(m.Mul(a*a*a - a).Add(n.Mul(b*b*b - b)).Mul(h * h / 6))
	derivative := q.Sub(p).Mul(1 / h).Add(m.Mul(1 - 3*a*a).Add(n.Mul(3*b*b - 1)).Mul(h / 6))
	second := m.Mul(a).Add(n.Mul(b))
	return position, derivative, second
}
