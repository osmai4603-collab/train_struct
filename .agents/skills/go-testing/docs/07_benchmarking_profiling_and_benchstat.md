# 07. اختبارات الأداء والقياس المقارن (Benchmarking & Benchstat)

## 1. التشريح الدقيق للـ Benchmark في Go (`testing.B`)

توفر لغة Go محرك قياس أداء أصيل فائق الدقة. يقوم المحرك بتكرار استدعاء الدالة مع زيادة قيمة `b.N` حتى تصل العينة إلى فترة زمنية مستقرة إحصائياً (افتراضياً 1 ثانية).

### الهيكل السليم للـ Benchmark ومكافحة تحسينات المترجم

```go
package serializer_test

import (
 "runtime"
 "testing"
)

// متغير عام في الحزمة (Package-Level Sink) لمنع المترجم من حذف الكود كـ Dead Code
var benchmarkSink []byte

func BenchmarkSerializePayload(b *testing.B) {
 data := generateSamplePayload()

 // 1. تفعيل تقارير تخصيص الذاكرة (Allocations & Bytes per op)
 b.ReportAllocs()

 // 2. تصفير الميقاتي بعد انتهاء التجهيزات المكلفة
 b.ResetTimer()

 var result []byte
 for i := 0; i < b.N; i++ {
  // العملية المطلوب قياسها بدقة
  result = SerializePayload(data)
 }

 // 3. منع المترجم من إلغاء العملية
 benchmarkSink = result
 runtime.KeepAlive(result)
}
```

---

## 2. القياس المتوازي للأداء (`b.RunParallel`)

لقياس سلوك الكود عند التوسع وتعدد الأنوية وحجم الاختناق على الأقفال (`sync.Mutex Contention`):

```go
func BenchmarkConcurrentRead(b *testing.B) {
 cache := NewCache()
 cache.Set("key", "val")

 b.ReportAllocs()
 b.ResetTimer()

 b.RunParallel(func(pb *testing.PB) {
  for pb.Next() {
   _ = cache.Get("key")
  }
 })
}
```

---

## 3. التحليل الإحصائي المعتمد باستخدام `benchstat`

المقارنة البصرية اليدوية لأرقام الـ Benchmarks غير دقيقة بسبب تقلب ضغط المعالج. الأداة المعيارية الرسمية المعتمدة هي `benchstat`.

### سير العمل القياسي للتحقق من التحسين البرمجي

1. **تثبيت الأداة:**

   ```bash
   go install golang.org/x/perf/cmd/benchstat@latest
   ```

2. **تسجيل أداء الفرع الحالي (Base Branch):**

   ```bash
   go test -bench=BenchmarkSerializePayload -count=10 > old.txt
   ```

3. **التبديل إلى فرع التحسين وتسجيل الأداء (Optimized Branch):**

   ```bash
   go test -bench=BenchmarkSerializePayload -count=10 > new.txt
   ```

4. **إجراء المقارنة الإحصائية:**

   ```bash
   benchstat old.txt new.txt
   ```

### قراءة النتائج النموذجية

```text
                     │   old.txt   │               new.txt               │
                     │   sec/op    │   sec/op     vs base                │
SerializePayload-16    120.5n ± 3%   84.2n ± 1%  -30.12% (p=0.000 n=10)

                     │   old.txt   │               new.txt               │
                     │    B/op     │    B/op      vs base                │
SerializePayload-16    32.00 ± 0%    0.00 ± 0%  -100.00% (p=0.000 n=10)
```

- **Delta (-30.12%):** نسبة التحسن في سرعة المعالجة.
- **p-value (0.000):** دلالة إحصائية قاطعة على أن التحسن حقيقي وليس بسبب صدفة عتادية.
- **B/op (0.00):** تم التخلص من تخصيص الذاكرة على الـ Heap بالكامل!
