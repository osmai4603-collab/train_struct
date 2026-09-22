# استراتيجيات الاختبار المتقدمة مع Context (Testing Strategies)

## 1. المعيار الحديث في Go 1.24+: استخدام `t.Context()`

في Go 1.24، وفّر محرك الاختبارات تابعاً مدمجاً `t.Context()` (وللـ Benchmarks عبر `b.Context()`). هذا السياق يُلغى تلقائياً وفوراً بمجرد انتهاء حالة الاختبار، مما يضمن تنظيف كافة الـ Goroutines والعمليات الشبكية التابعة:

```go
package service_test

import (
    "testing"
)

func TestOrderService_Create(t *testing.T) {
    // Go 1.24+: سياق مربوط بدورة حياة الاختبار وتُنظف موارده تلقائياً
    ctx := t.Context()

    svc := NewOrderService(testDB)
    order, err := svc.CreateOrder(ctx, validOrderReq)

    if err != nil {
        t.Fatalf("unexpected error: %v", err)
    }
    if order.ID == "" {
        t.Errorf("expected generated order ID")
    }
}
```

---

## 2. نمط ما قبل Go 1.24: التنظيف الصريح عبر `t.Cleanup`

إذا كان المشروع يستخدم إصداراً أقدم من Go 1.24، يجب استخدام `t.Cleanup` مع دالة الإلغاء لضمان استدعائها حتى في حالات الفشل الفوري عبر `t.FailNow()`:

```go
func TestLegacyService(t *testing.T) {
    ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
    t.Cleanup(cancel) // استدعاء مضمون عند نهاية الاختبار

    err := executeService(ctx)
    if err != nil {
        t.Fatalf("service execution failed: %v", err)
    }
}
```

---

## 3. اختبار سيناريوهات الإلغاء والفشل السريع (Pre-Canceled Context)

للتحقق من أن دوالك ترفض البدء وتفشل سريعاً عند استلام سياق ملغى مسبقاً دون إهدار موارد المعالجة:

```go
func TestService_FailFastOnCanceledContext(t *testing.T) {
    // إنشاء سياق وإلغاؤه فوراً قبل تمريره
    ctx, cancel := context.WithCancel(context.Background())
    cancel()

    svc := NewOrderService(testDB)
    _, err := svc.CreateOrder(ctx, validOrderReq)

    if !errors.Is(err, context.Canceled) {
        t.Errorf("expected context.Canceled error, got: %v", err)
    }
}
```

---

## 4. اختبار انتهاء المهل دون إبطاء دورة الاختبارات (Fast Timeout Testing)

لا تستخدم `time.Sleep` في اختبارات المهل؛ بل استخدم مهلة زمنية بالغة القصر (مثلاً 5 إلى 10 ملي ثانية) للتأكد من سلوك معالجة الخطأ بسرعة:

```go
func TestService_TimeoutHandling(t *testing.T) {
    // مهلة فائقة القصر لتحفيز تجاوز المهلة فوراً
    ctx, cancel := context.WithTimeout(context.Background(), 10*time.Millisecond)
    defer cancel()

    svc := NewSlowService()
    _, err := svc.HeavyTask(ctx)

    if !errors.Is(err, context.DeadlineExceeded) {
        t.Errorf("expected DeadlineExceeded error, got: %v", err)
    }
}
```
