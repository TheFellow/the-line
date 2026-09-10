// Package periodic solves the cyclic continuity equations of a cubic spline.
package periodic

// Second returns second derivatives at unique periodic knots. Spans contains
// each outgoing positive parameter interval, including the last-to-first span.
// Inputs must have equal lengths of at least three. The solve is O(n).
func Second(values, spans []float64) []float64 {
	n := len(values)
	diagonal, rhs := make([]float64, n), make([]float64, n)
	for i := range values {
		prev, next := (i+n-1)%n, (i+1)%n
		left, right := spans[prev], spans[i]
		diagonal[i] = 2 * (left + right)
		rhs[i] = 6 * ((values[next]-values[i])/right - (values[i]-values[prev])/left)
	}
	// Sherman-Morrison reduces the two corner entries to two tridiagonal solves.
	alpha, gamma := spans[n-1], -diagonal[0]
	diagonal[0] -= gamma
	diagonal[n-1] -= alpha * alpha / gamma
	solve := func(b []float64) []float64 {
		upper, x := make([]float64, n), make([]float64, n)
		pivot := diagonal[0]
		upper[0], x[0] = spans[0]/pivot, b[0]/pivot
		for i := 1; i < n; i++ {
			pivot = diagonal[i] - spans[i-1]*upper[i-1]
			if i < n-1 {
				upper[i] = spans[i] / pivot
			}
			x[i] = (b[i] - spans[i-1]*x[i-1]) / pivot
		}
		for i := n - 2; i >= 0; i-- {
			x[i] -= upper[i] * x[i+1]
		}
		return x
	}
	x := solve(rhs)
	u := make([]float64, n)
	u[0], u[n-1] = gamma, alpha
	z := solve(u)
	factor := (x[0] + alpha*x[n-1]/gamma) / (1 + z[0] + alpha*z[n-1]/gamma)
	for i := range x {
		x[i] -= factor * z[i]
	}
	return x
}
