# 05. أفضل الممارسات الهندسية والأنماط المضادة لحزمة `context`

تعتبر حزمة `context` حساسة جداً في مشاريع Go الإنتاجية؛ فاستخدامها بطريقة غير مدروسة قد يؤدي إلى سلوكيات خفية غير متوقعة، أو تسريبات للذاكرة، أو مشاكل في التزامن. يجمع هذا الملف أهم القواعد المعتمدة في كبرى الشركات العالمية (Google, Uber) ومجتمع Go المتقدم.

---

## 🏆 القواعد الذهبية لمجتمع Go (The Golden Rules)

### 1. `ctx` هو المعامل الأول دائماً (The First Parameter Rule)

يجب أن يكون `ctx context.Context` هو أول معامل صريح في أي دالة تحتاج لإدارة دورة الحياة أو الإلغاء أو التتبع:

```go
//a ✅ صحيح ومطابق لمعايير Go
func FetchUserOrders(ctx context.Context, userID string) ([]Order, error)

// ❌ خطأ فادح: وضع السياق في المنتصف أو النهاية
func FetchUserOrders(userID string, ctx context.Context) ([]Order, error)
```

---

### 2. حظر تخزين `Context` في هياكل البيانات (Never Store in Structs)

> [!CAUTION]
> **قاعدة قطعية:** لا تقم أبداً بتخزين `context.Context` كحقل داخل هيكل (`struct`) في الـ Services أو Repositories أو Handlers!

#### لماذا؟

1. **تفاوت دورة الحياة (Lifecycle Mismatch):** الهيكل (مثل `UserService`) يعيش طوال فترة تشغيل التطبيق (Application Lifetime)، بينما السياق مخصص لطلب واحد فقط يعيش لأجزاء من الثانية (Request Lifetime). تخزين سياق طلب منتهي داخل الخدمة سيجعل كل الطلبات اللاحقة تفشل فوراً باعتبار السياق ملغياً!
2. **سباقات البيانات (Data Races):** إذا كان الهيكل مشتركاً بين عدة طلبات متزامنة، فإن استبدال أو قراءة حقل `ctx` سيسبب سباق بيانات يهدد استقرار التطبيق.

#### الاستثناءات النادرة والمبررة فقط

- كائن `http.Request` في المكتبة القياسية: لأنه يمثل طلباً عابراً واحداً لا يُعاد استخدامه أبداً بين عملاء مختلفين.

```go
//a ❌ خطأ جسيم في التصميم
type OrderService struct {
    db  *sql.DB
    ctx context.Context // كارثة معمارية!
}

// ✅ التصميم النظيف الصحيح
type OrderService struct {
    db *sql.DB
}

func (s *OrderService) CreateOrder(ctx context.Context, order *Order) error {
    return s.db.ExecContext(ctx, "INSERT ...")
}
```

---

### 3. حظر تمرير `nil` نهائياً (Never Pass a Nil Context)

إذا كانت دالة تطلب `context.Context`، فلا تمرر `nil` مطلقاً حتى لو كانت الدالة تقبل ذلك حالياً، لأن الدوال ستطلق `panic` عند استدعاء أي دالة فرعية:

```go
//a ❌ غير مقبول
res, err := client.DoSomething(nil)

// ✅ إذا كنت في البداية أو الاختبارات، استخدم Background
res, err := client.DoSomething(context.Background())

// ✅ إذا كان الكود قيد التطوير ولم تحدد السياق بعد، استخدم TODO
res, err := client.DoSomething(context.TODO())
```

---

### 4. الحفاظ على الموارد باستدعاء `defer cancel()` فوراً

كل استدعاء لـ `WithCancel` أو `WithTimeout` أو `WithDeadline` يجب أن يتبعه استدعاء مباشر لـ `defer cancel()`:

```go
func QueryWithTimeout(parent context.Context) error {
    ctx, cancel := context.WithTimeout(parent, 2*time.Second)
    defer cancel() // يضمن تحرير المؤقت الزمني وإزالة الابن من الأب

    return execute(ctx)
}
```

---

### 5. النمط الآمن لمفاتيح القيم (Zero-Allocation Unexported Keys)

لا تستخدم النصوص العادية (`string`) أو الأنواع الأساسية كمفاتيح في `WithValue` لمنع تصادم المفاتيح بين الحزم المختلفة. اتبع دائماً هذا النمط المعتمد:

```go
package correlation

import "context"

// 1. تعريف نوع غير مصدّر خالي الحجم (Zero size struct)
type correlationKey struct{}

// 2. دالة حقن آمنة ذات نوع محدد
func WithCorrelationID(ctx context.Context, id string) context.Context {
    return context.WithValue(ctx, correlationKey{}, id)
}

// 3. دالة استخراج آمنة ذات نوع محدد
func FromContext(ctx context.Context) (string, bool) {
    id, ok := ctx.Value(correlationKey{}).(string)
    return id, ok
}
```

---

## 🚫 مصفوفة الأنماط المضادة (Anti-Patterns Matrix)

| النمط المضاد (Anti-Pattern) | الخطر والأثر المترتب | الحل الهندسي المعتمد |
| :--- | :--- | :--- |
| **تمرير المعاملات الاختيارية عبر `WithValue`** | فقدان التحقق الثابت للأنواع (Type Safety)، وصعوبة قراءة واكتشاف معاملات الدالة، وغموض الكود. | استخدام معاملات صريحة بالدالة، أو نمط الخيارات الوظيفية (`Functional Options Pattern`). |
| **تجاهل `<-ctx.Done()` في الحلقات** | استمرار الـ Goroutines في العمل في الخلفية وهدر المعالج حتى بعد إلغاء الطلب من العميل. | فحص `ctx.Err() != nil` أو وضع `case <-ctx.Done(): return ctx.Err()` في جملة الـ `select`. |
| **تمرير `r.Context()` لمهام خلفية منفصلة** | فشل المهام الخلفية وموتها فور انتهاء دالة الـ HTTP Handler لأن الخادم يلغي السياق. | استخدام `context.WithoutCancel(r.Context())` متبوعاً بمهلة زمنية مستقلة. |
| **إعادة استخدام سياق ملغى** | فشل العمليات اللاحقة فوراً دون أن تبدأ حتى. | إنشاء سياق جديد مشتق من `context.Background()` للعمليات الجديدة كلياً. |

---

## 🧪 استراتيجيات الاختبارات في Go الحديثة (Go 1.24+)

في بيئات الاختبارات الأوتوماتيكية، وفرت لغة Go في إصداراتها الحديثة دالة مدمجة داخل كائن الاختبار `testing.T`:

```go
func TestOrderProcessing(t *testing.T) {
    // يوفر سياقاً يتم إلغاؤه تلقائياً فور انتهاء الاختبار أو فشله
    ctx := t.Context()

    err := processOrder(ctx, "order-123")
    if err != nil {
        t.Fatalf("expected success, got %v", err)
    }
}
```

- **المميزات:**
  - يلغي الحاجة لإنشاء `context.WithCancel` واستدعاء `defer cancel()` يدوياً في الاختبارات.
  - يعزل الاختبارات الفرعية (`Subtests`) وينهي العمليات المعلقة تلقائياً لمنع تداخل الاختبارات المتوازية (`t.Parallel()`).
