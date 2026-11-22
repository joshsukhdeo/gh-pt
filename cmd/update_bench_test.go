package cmd

import (
	"fmt"
	"testing"
)

func BenchmarkBuildBatchReleaseQuery(b *testing.B) {
	sizes := []int{10, 30, 50}
	for _, size := range sizes {
		repos := make([]string, size)
		for i := 0; i < size; i++ {
			repos[i] = fmt.Sprintf("owner%d/repo%d", i, i)
		}
		b.Run(fmt.Sprintf("repos_%d", size), func(b *testing.B) {
			b.ReportAllocs()
			b.ResetTimer()
			for i := 0; i < b.N; i++ {
				q, m := BuildBatchReleaseQuery(repos)
				if q == "" || len(m) != size {
					b.Fatalf("unexpected query result")
				}
			}
		})
	}
}
