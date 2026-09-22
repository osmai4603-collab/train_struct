# إدارة الإلغاء وحتمية `defer cancel()` ومكافحة تسريب الموارد

## 1. ميكانيكا الإلغاء في Go وكيف تعمل داخلياً

السياق في Go لا يستخدم مقاطعات أنظمة التشغيل الإجبارية (Signals/Interrupts) ولا يقوم بقتل الـ Goroutines عنوة، بل يعتمد على **التعاون الطوعي (Cooperative Cancellation)** عبر قنوات Go القياسية (Channels):

1. توفر الدالة `ctx.Done()` قناة قراءة فقط `<-chan struct{}`.
2. ما دامت العملية جارية ولم تُطلب إشارة الإلغاء، تكون القناة مفتوحة ولكن فارغة (تُعطل القراءة Blocking).
3. عند استدعاء `cancel()` أو بلوغ المهلة الزمنية (Deadline)، تقوم Go بـ **إغلاق القناة (Close Channel)**.
4. إغلاق القناة يؤدي فوراً إلى تفعيل كافة تعبيرات `case <-ctx.Done():` في كل الـ Goroutines المستمعة لنفس الشجرة.

---

## 2. حتمية `defer cancel()` وتفادي تسريب الذاكرة والمؤقتات

أحد أكثر الأخطاء شيوعاً في كود Go هو إهمال استدعاء دالة الإلغاء:

```go
// ❌ خطأ تسريب الذاكرة والمؤقت (Timer Leak):
func FetchRemoteData(ctx context.Context) (*Data, error) {
    ctx, _ = context.WithTimeout(ctx, 30*time.Second) // إهمال cancel!
    return httpClient.GetData(ctx)
}
```

### ماذا يحدث عند إهمال `cancel()`؟
1. **في `WithTimeout` و `WithDeadline`:**
   تُسجل Go مؤقتاً داخلياً في محرك الـ Runtime (`time.Timer`). إذا اكتملت الدالة بعد 10 ملي ثانية فقط، يظل كائن الـ Timer محجوزاً في الذاكرة ومجدولاً في نظام التشغيل حتى تمر الـ 30 ثانية كاملة!
2. **في `WithCancel`:**
   يتم تسجيل عقدة السياق الابن داخل خريطة أبناء السياق الأب (`parent.children`). طالما لم يُستدعَ `cancel()`، يظل الابن معلقاً برباط في الذاكرة يمنع الـ Garbage Collector من تحرير موارده حتى يتم إلغاء الأب.

### التصحيح المعياري الإلزامي:
```go
// ✅ استدعاء defer cancel() فور إنشاء السياق مباشرة
func FetchRemoteData(ctx context.Context) (*Data, error) {
    ctx, cancel := context.WithTimeout(ctx, 30*time.Second)
    defer cancel() // يحرر مؤقت النظام وعقدة الشجرة فور خروج الدالة

    return httpClient.GetData(ctx)
}
```

---

## 3. مكافحة تسريب الـ Goroutines (Goroutine Leaks)

> مبدأ المهندس Dave Cheney الشهير:  
> **"Never start a goroutine without knowing how and when it will stop."**  
> (لا تبدأ أبداً Goroutine دون أن تعرف بالضبط كيف ومتى ستتوقف).

### أ. تسريب الكتابة في القنوات غير المؤقتة (Channel Write Leak)
```go
// ❌ تسريب Goroutine: إذا خرجت الدالة الرئيسية بسبب الإلغاء، يعلق الـ worker للأبد
func QueryFirstResponse(ctx context.Context) string {
    ch := make(chan string) // قناة غير مبطنة (Unbuffered)

    go func() {
        res := slowQuery()
        ch <- res // إذا لم يعد أحد يستمع لـ ch، سيعلق هذا الـ Goroutine في الذاكرة للأبد!
    }()

    select {
    case <-ctx.Done():
        return ""
    case res := <-ch:
        return res
    }
}
```

#### الحلول المعتمدة:
1. **استخدام قناة مبطنة بحجم الإرسال (`make(chan string, 1)`):** تتيح للعامل وضع النتيجة والخروج حتى لو لم يقرأها أحد.
2. **الاستماع للإلغاء أثناء الإرسال:**
```go
// ✅ صحيح: الاستماع لـ ctx.Done أثناء إرسال النتيجة
select {
case ch <- res:
case <-ctx.Done():
    // تم إلغاء المستمع، خروج آمن بدون تعليق
    return
}
```

### ب. تسريب الحلقات التكرارية والعمال (Workers)
```go
// ✅ صحيح: الاستماع الدائم في الحلقات عبر select
func ProcessQueue(ctx context.Context, jobs <-chan Job) error {
    for {
        select {
        case <-ctx.Done():
            // إلغاء السياق: إيقاف المعالجة فوراً
            return ctx.Err()

        case job, ok := <-jobs:
            if !ok {
                return nil // أغلقت القناة طبيعياً
            }
            if err := executeJob(ctx, job); err != nil {
                return err
            }
        }
    }
}
```

---

## 4. التزامن المنضبط مع `golang.org/x/sync/errgroup`

بدلاً من إدارة `sync.WaitGroup` وقنوات الأخطاء يدوياً، توفر حزمة `errgroup` النمط الأمثل لإلغاء المهام التابعة فور فشل إحداها:

```go
package aggregator

import (
    "context"
    "golang.org/x/sync/errgroup"
)

func FetchAggregatedData(ctx context.Context, id string) (*Result, error) {
    // ينشئ مجموعة وسياقاً مشتقاً يُلغى تلقائياً بمجرد إرجاع أي مهمة لخطأ
    g, ctx := errgroup.WithContext(ctx)
    
    // وضع سقف أعلى لعدد الـ Goroutines المتزامنة
    g.SetLimit(5)

    var resA *DataA
    var resB *DataB

    g.Go(func() error {
        var err error
        resA, err = fetchA(ctx, id)
        return err // إذا فشل هذا، تُلغى المهام الأخرى عبر نفس ctx
    })

    g.Go(func() error {
        var err error
        resB, err = fetchB(ctx, id)
        return err
    })

    // انتظار اكتمال الكل أو أول خطأ يطرأ
    if err := g.Wait(); err != nil {
        return nil, err
    }

    return &Result{A: resA, B: resB}, nil
}
```
