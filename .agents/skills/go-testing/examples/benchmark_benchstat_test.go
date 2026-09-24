package examples_test

import (
	"crypto/sha256"
	"runtime"
	"testing"
)

// Sink لحفظ المخرجات ومنع تحسينات المترجم
var benchmarkByteSink [32]byte

// BenchmarkHashCalculation يقيس سرعة حساب التجزئة التشفيرية
func BenchmarkHashCalculation(b *testing.B) {
	data := []byte("payload-to-be-hashed-for-benchmark-evaluation")

	// 1. تقرير استهلاك وتخصيص الذاكرة (B/op & allocs/op)
	b.ReportAllocs()

	// 2. تصفير الميقاتي قبل البدء بحلقة القياس
	b.ResetTimer()

	var hash [32]byte
	for i := 0; i < b.N; i++ {
		hash = sha256.Sum256(data)
	}

	benchmarkByteSink = hash
	runtime.KeepAlive(hash)
}

// BenchmarkHashCalculation_Parallel يقيس الأداء عند تعدد الأنوية والـ Goroutines
func BenchmarkHashCalculation_Parallel(b *testing.B) {
	data := []byte("parallel-payload-for-multi-core-stress")

	b.ReportAllocs()
	b.ResetTimer()

	b.RunParallel(func(pb *testing.PB) {
		var localHash [32]byte
		for pb.Next() {
			localHash = sha256.Sum256(data)
		}
		runtime.KeepAlive(localHash)
	})
}
