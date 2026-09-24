# 06. اختبارات التزامن، كاشف السباق، و synctest

## 1. كاشف سباق البيانات المعياري (`go test -race`)

في أنظمة Go الموزعة، تقع كوارث الإنتاج بسبب قراءة وكتابة مشتركة على نفس الذاكرة دون حماية (`data race`). يوفر مترجم Go أداة مدمجة تعتمد على محرك ThreadSanitizer.

### قواعد إلزامية في المشاريع الكبيرة

1. **فحص الـ CI الإلزامي:** يجب أن تفشل أي عملية بناء في GitHub Actions أو GitLab CI إذا لم يُمرر فحص:

   ```bash
   go test -race -v ./...
   ```

2. **فحص الإجهاد التكراري (Stress Testing):** لرصد سباقات البيانات النادرة أو المتقطعة:

   ```bash
   go test -race -count=50 ./internal/service/...
   ```

---

## 2. كشف تسريب الـ Goroutines عبر `uber-go/goleak`

عندما تنتهي دالة الاختبار وتبقى Goroutine عالقة تنتظر قراءة من قناة أو اتصال شبكي، تتراكم هذه الخيوط وتؤدي في النهاية لانهيار الخدمة (Out of Memory).

### التطبيق القياسي

```go
package worker_test

import (
 "testing"
 "go.uber.org/goleak"
)

// فحص شامل للحزمة بأكملها بعد اكتمال جميع الاختبارات
func TestMain(m *testing.M) {
 goleak.VerifyTestMain(m)
}

// أو فحص دقيق لاختبار محدد
func TestWorkerShutdown(t *testing.T) {
 defer goleak.VerifyNone(t)

 w := NewWorker()
 w.Start()
 w.Stop() // إذا كان Stop يحتوي على تسريب، سيفشل الاختبار هنا
}
```

---

## 3. الثورة الهندسية في Go 1.24+: حزمة `testing/synctest`

أحد أكبر تحديات اختبارات Go تاريخياً كان اختبار التراجع الأسي (Exponential Backoff) والمهل الزمنية (Timeouts). المطورون كانوا يضطرون إما لوضع `time.Sleep(5 * time.Second)` مما يجعل الاختبارات بطيئة وهشة، أو حقن واجهات ساعة مخصصة (`Clock interface`).

في Go 1.24+، تقدم حزمة `testing/synctest` مفهوم **"فقاعة الوقت الافتراضي" (Virtual Time Bubble)**:

1. ينفذ الكود داخل دالة `synctest.Run(func() { ... })`.
2. عندما تدخل كل الـ Goroutines داخل الفقاعة في حالة انتظار (`time.Sleep` أو `chan recv`)، يقفز المحرك بالوقت الافتراضي فوراً للحظة الاستيقاظ التالية دون إهدار ثانية واحدة في العالم الحقيقي!

```go
package backoff_test

import (
 "testing"
 "testing/synctest"
 "time"
)

func RetryWithDelay(attempts int, delay time.Duration, fn func() error) error {
 for i := 0; i < attempts; i++ {
  err := fn()
  if err == nil {
   return nil
  }
  time.Sleep(delay) // انتظار ظاهري
 }
 return nil
}

func TestRetryWithDelay_Instant(t *testing.T) {
 // تفعيل ميزة synctest في Go 1.24
 synctest.Run(func() {
  calls := 0
  start := time.Now()

  _ = RetryWithDelay(3, 10*time.Minute, func() error {
   calls++
   return nil
  })

  // ينتهي الاختبار خلال جزء من الملي ثانية رغم أن الانتظار الكلي كان 20 دقيقة افتراضية!
  if calls != 1 {
   t.Errorf("calls = %d, want 1", calls)
  }
 })
}
```

> [!NOTE]
> لتشغيل الاختبارات المعتمدة على `synctest`، يتم تمرير متغير البيئة:
> `GOEXPERIMENT=synctest go test ./...`
