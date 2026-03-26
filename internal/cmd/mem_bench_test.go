package cmd

import (
	"testing"
)

func BenchmarkCLIInit(b *testing.B) {
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		_ = NewRootCmd()
	}
}
