# قواعد التمرير وتصميم واجهات البرمجة (API Design & Propagation)

## 1. القاعدة الذهبية: `ctx` هو المعامل الأول دائماً

وفقاً لـ [Google Go Style Guide](https://google.github.io/styleguide/go/decisions#context-parameters) وتوثيق Go الرسمي، يجب أن يكون السياق هو المعامل الأول في أي دالة أو تابع، مع الالتزام الصارم بتسمية المعامل `ctx`:

```go
// ✅ صحيح ومطابق لمعايير Go العالمية
func (s *PaymentService) ProcessTransaction(ctx context.Context, txID string, amount int64) error

// ❌ خطأ: تغيير ترتيب المعاملات يربك القارئ والتحليل الثابت
func (s *PaymentService) ProcessTransaction(txID string, amount int64, ctx context.Context) error

// ❌ خطأ: تسمية السياق بأسماء غير قياسية مثل c أو context
func (s *PaymentService) ProcessTransaction(c context.Context, txID string) error
```

---

## 2. حظر تخزين `context.Context` داخل الـ Structs

تخزين السياق داخل بنية المعطيات (Struct) هو أحد أسوأ الأنماط المضادة التي تواجهها المشاريع الكبيرة:

```go
// ❌ كارثة تصميمية: تخزين السياق في بنية الخدمة أو المستودع
type UserRepository struct {
    ctx context.Context // ممنوع نهائياً!
    db  *sql.DB
}

func (r *UserRepository) FindByID(id string) (*User, error) {
    return r.db.QueryRowContext(r.ctx, "...", id) // خطأ!
}
```

### الأسباب الهندسية للحظر الصارم

1. **اختلاف النطاق والعمر (Scope Mismatch):**
   - كائن الـ Struct (`Service`, `Repository`, `Client`) هو كائن طويل الأمد (Long-Lived Object) يعيش طوال فترة تشغيل التطبيق.
   - كائن الـ `Context` هو كائن قصير الأمد عابر (Short-Lived Request-Scoped) يولد مع وصول الطلب ويموت بمجرد إرسال الاستجابة.
   - تخزين السياق داخل الـ Struct يجعل كل الاستدعاءات اللاحقة تعتمد على سياق قديم، قد يكون ملغياً أو منتهي الصلاحية!
2. **غياب الأمان المتزامن (Race Hazards & State Pollution):**
   - إذا تم استدعاء توبع الـ Struct بالتوازي من عدة طلبات متزامنة، فإن استخدام نفس السياق المخزن يؤدي إلى تشويش بيانات التتبع وتلويث حالة الطلبات.
3. **إخفاء عقود الواجهات (Hidden Dependencies):**
   - المستدعي لا يدرك أن التابع مقيد بمهلة أو قابل للإلغاء دون فحص الكود الداخلي للبنية.

### التصحيح المعياري المعتمد

```go
// ✅ صحيح: السياق يمر صراحة مع كل استدعاء
type UserRepository struct {
    db *sql.DB
}

func (r *UserRepository) FindByID(ctx context.Context, id string) (*User, error) {
    return r.db.QueryRowContext(ctx, "SELECT ... WHERE id = $1", id)
}
```

> **الاستثناء الوحيد المعترف به:** يُسمح بتخزين السياق فقط في البنى التي تمثل رسالة أو طلباً في حد ذاتها بغرض التوافقية العكسية مثل `http.Request` في المكتبة القياسية.

---

## 3. حظر تمرير `nil` نهائياً: متى تستخدم `Background` ومتى `TODO`؟

لا تمرر `nil` كقيمة لسياق على الإطلاق؛ فمحاولة الوصول إلى أي تابع للسياق على مؤشر `nil` ستفجر التطبيق بـ `panic`:

```go
// ❌ كارثة: يسبب panic عند استدعاء ctx.Done() في الطبقات الأدنى
service.Execute(nil, data)
```

### المقارنة بين `context.Background()` و `context.TODO()`

| المعيار | `context.Background()` | `context.TODO()` |
| :--- | :--- | :--- |
| **الاستخدام الأساسي** | جذر الشجرة الأساسي للعمليات الإنتاجية الحية. | إشارة مؤقتة أثناء كتابة وتطوير الكود أو إعادة الهيكلة (Refactoring). |
| **الموقع المناسب** | نقطة انطلاق الخادم (`main.go`)، أو عمال الخلفية المستقلين، أو الاختبارات. | عندما تكون الدالة بحاجة لسياق ولكن المستدعي الحالي لم يُحدث بعد لتمريره. |
| **الرسالة الدلالية** | "هذا هو السياق الجذري الدائم للعملية". | "أنا لست متأكداً بعد من أي سياق يجب استخدامه، سأعود لتصحيحه لاحقاً". |
| **أدوات الفحص الثابت** | تقبله الأدوات دون إنذار. | تكتشفه أدوات مثل `golangci-lint` لتنبيه المطور بضرورة استبداله قبل الدمج. |

---

## 4. قاعدة التوقيع الصادق: أي دالة تقبل السياق يجب أن تُرجع `error`

وفقاً لـ [Google Go Style Guide](https://google.github.io/styleguide/go/decisions#context-parameters):
> "إذا كانت الدالة تقبل `context.Context`، فيجب عليها عادةً أن تُرجع `error`."

### التعليل الهندسي

قبول الدالة لـ `ctx` يعني ضمناً أنها تراعي الإلغاء أو المهل الزمنية. وإذا تم إلغاء السياق أثناء تنفيذ الدالة، يجب أن تمتلك الدالة وسيلة لإبلاغ المستدعي بذلك عبر إرجاع `ctx.Err()`:

```go
// ❌ غير منطقي: كيف ستخبر المستدعي إذا تم إلغاء السياق أثناء المعالجة؟
func CalculateSum(ctx context.Context, numbers []int) int

// ✅ سليم: تتيح للمستدعي معرفة ما إذا كانت العملية توقفت بسبب انتهاء المهلة
func CalculateSum(ctx context.Context, numbers []int) (int, error)
```

---

## 5. فخ تظليل المتغيرات (Variable Shadowing Trap)

عند إضافة مهلة زمنية مشروطة لسياق، يقع الكثير في خطأ تعريف متغير جديد داخل كتلة الـ `if` باستخدام `:=`:

```go
// ❌ خطأ تظليل المتغير (Shadowing):
func ExecuteWorkflow(ctx context.Context, timeout time.Duration) error {
    if timeout > 0 {
        // خطأ: ctx هنا متغير جديد تماماً يقتصر نطاقه على داخل الـ if!
        ctx, cancel := context.WithTimeout(ctx, timeout)
        defer cancel()
    }

    // هنا: ctx هو السياق الأصلي غير المقيد بأي مهلة!
    return downstreamCall(ctx)
}

// ✅ التصحيح المعتمد:
func ExecuteWorkflow(ctx context.Context, timeout time.Duration) error {
    if timeout > 0 {
        var cancel context.CancelFunc
        // استخدام = لإعادة تعيين المتغير الأصلي نفسه
        ctx, cancel = context.WithTimeout(ctx, timeout)
        defer cancel()
    }

    return downstreamCall(ctx)
}
```
