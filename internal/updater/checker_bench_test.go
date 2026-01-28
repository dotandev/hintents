package updater

import (
	"testing"
)

// BenchmarkCheckForUpdates benchmarks the update checker
// Should be very fast because it runs asynchronously
func BenchmarkCheckForUpdates(b *testing.B) {
	checker := NewChecker("v1.0.0")

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		checker.CheckForUpdates()
	}
}

// BenchmarkVersionComparison benchmarks version comparison
func BenchmarkVersionComparison(b *testing.B) {
	checker := NewChecker("v1.0.0")

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		checker.compareVersions("v1.0.0", "v2.0.0")
	}
}

// BenchmarkCacheCheck benchmarks cache checking
func BenchmarkCacheCheck(b *testing.B) {
	checker := NewChecker("v1.0.0")

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		checker.shouldCheck()
	}
}

// BenchmarkNewChecker benchmarks checker creation
func BenchmarkNewChecker(b *testing.B) {
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = NewChecker("v1.0.0")
	}
}
