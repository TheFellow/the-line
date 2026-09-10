package solver

import "testing"

func BenchmarkEvaluateEsses(b *testing.B) {
	s, v := fixture(b, "esses")
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		if _, err := Evaluate(s, v, nil, DefaultOptions()); err != nil {
			b.Fatal(err)
		}
	}
}
func BenchmarkSolveRichEsses(b *testing.B) {
	s, v := fixture(b, "esses")
	v.FrontBrake = .6
	v.CGHeight = .35
	v.Wheelbase = 2.8
	v.LoadSensitivity = .12
	v.LiftArea = 3
	v.AeroBalance = .45
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		if _, err := Solve(s, v, DefaultOptions()); err != nil {
			b.Fatal(err)
		}
	}
}
