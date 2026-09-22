# ميزانية المهل وانتقالها التنازلي عبر الخدمات (Timeouts & Deadlines)

## 1. رتابة المهلة (Deadline Monotonicity)

القاعدة الحاكمة لشجرة السياق في لغة Go هي: **السياق الابن لا يمكنه أبداً تمديد مهلة السياق الأب**.

$$\text{Effective Deadline} = \min(\text{Parent Deadline}, \text{Child Deadline})$$

```go
// الأب ينتهي بعد 2 ثانية
parentCtx, cancelParent := context.WithTimeout(context.Background(), 2*time.Second)
defer cancelParent()

// الابن يطلب 10 ثوانٍ!
childCtx, cancelChild := context.WithTimeout(parentCtx, 10*time.Second)
defer cancelChild()

// النتيجة: childCtx سيتوقف حتماً بعد ثانيتين فقط!
```

---

## 2. إدارة وتوزيع ميزانية الوقت (Deadline Budgeting)

في بنية الخدمات المصغرة (Microservices) والتطبيقات الكبيرة، يجب ألا تُترك المهل للصدفة. إذا حُددت مهلة الطلب الإجمالية بـ 3 ثوانٍ، يجب تقسيم هذه الميزانية بذكاء بين الطبقات والخدمات الفرعية، مع إبقاء هامش أمان (Safety Buffer) لمعالجة الأخطاء والتنظيف:

```text
إجمالي ميزانية الطلب (3000ms)
├──────────────────────┬──────────────────────┬──────────────────────┬─────────────┐
│ طبقة الـ Cache       │ استعلام الـ DB       │ خدمة خارجية (RPC)    │ هامش أمان   │
│ (200ms max)          │ (800ms max)          │ (1500ms max)         │ (500ms)     │
└──────────────────────┴──────────────────────┴──────────────────────┴─────────────┘
```

---

## 3. نمط التحقق الاستباقي (Fail-Fast on Insufficient Budget)

قبل الشروع في عملية شبكية باهظة التكلفة، افحص ما إذا كان الوقت المتبقي في السياق كافياً لتنفيذها أصلاً؛ فإذا كان المتبقي 10 ملي ثانية وعملية الدفع تحتاج 300 ملي ثانية كحد أدنى، فالبدء بها هدر للموارد:

```go
func ExecutePaymentWithBudget(ctx context.Context, amount int64) error {
    const minRequiredTime = 300 * time.Millisecond

    if deadline, ok := ctx.Deadline(); ok {
        remaining := time.Until(deadline)
        if remaining < minRequiredTime {
            // فشل سريع دون إرهاق الشبكة
            return fmt.Errorf("insufficient time remaining (%v < %v): %w", 
                remaining, minRequiredTime, context.DeadlineExceeded)
        }
    }

    // تقييد استدعاء الدفع بالمهلة المخصصة له
    payCtx, cancel := context.WithTimeout(ctx, 1500*time.Millisecond)
    defer cancel()

    return gatewayClient.Charge(payCtx, amount)
}
```

---

## 4. نقل المهل عبر الحدود الشبكية (Distributed Context Propagation)

عند استدعاء خدمة مصغرة خارجية عبر HTTP، يجب نقل الوقت المتبقي في الترويسات (Headers) حتى تحترم الخدمة المستهدفة نفس المهلة ولا تعمل في فراغ:

```go
func CallDownstreamHTTP(ctx context.Context, targetURL string) (*http.Response, error) {
    req, err := http.NewRequestWithContext(ctx, http.MethodPost, targetURL, nil)
    if err != nil {
        return nil, err
    }

    // حساب المهلة المتبقية وتمريرها في ترويسة الطلب
    if deadline, ok := ctx.Deadline(); ok {
        remaining := time.Until(deadline)
        if remaining > 0 {
            req.Header.Set("X-Request-Timeout-Ms", fmt.Sprintf("%d", remaining.Milliseconds()))
        }
    }

    return httpClient.Do(req)
}
```

في بروتوكول **gRPC**، تتولى Go هذا النقل تلقائياً عبر ترويسة `grpc-timeout` المعيارية بين خوادم وعملاء gRPC.
