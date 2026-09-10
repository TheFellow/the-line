package solver

import (
	"context"
	"fmt"
	"math"
	"testing"

	"github.com/TheFellow/the-line/pkg/track"
)

func BenchmarkSolveEssesWorkers(b *testing.B) {
	scene, car := fixture(b, "esses")
	for _, workers := range []int{1, 2, 4} {
		b.Run(fmt.Sprint(workers), func(b *testing.B) {
			opts := DefaultOptions()
			opts.Workers = workers
			b.ReportAllocs()
			b.ResetTimer()
			for range b.N {
				result, err := Solve(scene, car, opts)
				if err != nil {
					b.Fatal(err)
				}
				b.ReportMetric(float64(result.Candidates), "candidates/op")
				b.ReportMetric(float64(result.FineCandidates), "finalists/op")
			}
		})
	}
}

// finalistFixture keeps road preparation out of the timed worker comparison.
// Sixteen smooth, distinct offsets exercise the full geometry/force/profile
// pipeline on the same fine mesh used by actual search finalists.
func finalistFixture(tb testing.TB) (evaluator, [][]float64) {
	tb.Helper()
	scene, car := fixture(tb, "esses")
	road, err := track.SampleRoad(scene, .5)
	if err != nil {
		tb.Fatal(err)
	}
	clearance := car.Width/2 + .25
	eval := evaluator{ctx: context.Background(), road: road, model: car, entry: scene.EntrySpeed, exit: scene.ExitSpeed, clearance: clearance, edges: indexRoadEdges(road, clearance)}
	offsets := make([][]float64, 16)
	length := road[len(road)-1].S
	for i := range offsets {
		offsets[i] = make([]float64, len(road))
		amplitude := .05 * float64(i+1)
		if i%2 == 0 {
			amplitude = -amplitude
		}
		for j, sample := range road {
			offsets[i][j] = amplitude * math.Sin(2*math.Pi*sample.S/length)
		}
	}
	return eval, offsets
}

func BenchmarkFinalistBatchWorkers(b *testing.B) {
	eval, offsets := finalistFixture(b)
	// Validate the fixture once outside every timed sub-benchmark.
	for i, result := range evaluateCandidates(context.Background(), eval, offsets, 1) {
		if result.err != nil {
			b.Fatalf("candidate %d: %v", i, result.err)
		}
	}
	for _, workers := range []int{1, 2, 4} {
		b.Run(fmt.Sprint(workers), func(b *testing.B) {
			b.ReportAllocs()
			b.ResetTimer()
			b.ReportMetric(float64(len(offsets)), "candidates/op")
			for range b.N {
				results := evaluateCandidates(context.Background(), eval, offsets, workers)
				for i, result := range results {
					if result.err != nil {
						b.Fatalf("candidate %d: %v", i, result.err)
					}
				}
			}
		})
	}
}

func BenchmarkRoadEdgeIndex(b *testing.B) {
	scene := track.Scene{Version: 2, Name: "Two kilometre kerb straight", KerbsCountAsRoad: true, Points: []track.Point{
		{WidthLeft: 6, WidthRight: 6, Surface: "asphalt", KerbLeft: track.Kerb{Width: 1, Grip: .8}},
		{X: 2000, WidthLeft: 6, WidthRight: 6, Surface: "asphalt", KerbLeft: track.Kerb{Width: 1, Grip: .8}},
	}}
	road, err := track.SampleRoad(scene, .5)
	if err != nil {
		b.Fatal(err)
	}
	const radius = 1.15
	b.Run("Build", func(b *testing.B) {
		b.ReportAllocs()
		b.ResetTimer()
		b.ReportMetric(float64(len(road)-1), "cells/op")
		for range b.N {
			index := indexRoadEdges(road, radius)
			if len(index.legal.cells) == 0 {
				b.Fatal("empty index")
			}
		}
	})
	index := indexRoadEdges(road, radius)
	points := make([]track.Vec3, len(road))
	for i, s := range road {
		points[i] = s.AtOffset(5.5)
	}
	b.Run("SweptQueries", func(b *testing.B) {
		b.ReportAllocs()
		b.ResetTimer()
		b.ReportMetric(float64(2*(len(points)-1)), "queries/op")
		for range b.N {
			for i := 0; i < len(points)-1; i++ {
				legal := index.legal.contactGrip(points[i], points[i+1], radius)
				kerb := index.kerbs.contactGrip(points[i], points[i+1], radius)
				if !math.IsInf(legal, 1) || kerb != .8 {
					b.Fatalf("unexpected contact: legal %g kerb %g", legal, kerb)
				}
			}
		}
	})
}
