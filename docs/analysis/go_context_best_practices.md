# أفضل الممارسات العالمية للتعامل مع السياق (Context) في مشاريع Go الكبيرة

> **تاريخ البحث والتوثيق:** 2026-09-21  
> **المصادر الرسمية والمراجع المعتمدة:** Go Official Documentation (`pkg.go.dev/context`), Go Official Blog (Sameer Ajmani, Jean Barkhuysen & Matt T. Proud), Google Go Style Guide, Uber Go Style Guide, Go Proposals (#51365, #57928, #56345), Dave Cheney, Ian Lance Taylor.

---

## جدول المحتويات

1. [الفلسفة التأسيسية: ما هو `context.Context` ولماذا تم ابتكاره؟](#1-الفلسفة-التأسيسية-ما-هو-contextcontext-ولماذا-تم-ابتكاره)
2. [التطور المعياري لحزمة `context` عبر إصدارات Go](#2-التطور-المعياري-لحزمة-context-عبر-إصدارات-go)
3. [قواعد التمرير وتصميم واجهات البرمجة (API Design & Propagation)](#3-قواعد-التمرير-وتصميم-واجهات-البرمجة-api-design--propagation)
4. [إدارة الإلغاء ومكافحة تسريب الـ Goroutines والذاكرة](#4-إدارة-الإلغاء-ومكافحة-تسريب-الـ-goroutines-والذاكرة)
5. [ميزانية المهل الزمنية وانتقالها عبر الخدمات (Timeouts & Deadlines)](#5-ميزانية-المهل-الزمنية-وانتقالها-عبر-الخدمات-timeouts--deadlines)
6. [بيانات السياق (`context.WithValue`): الاستخدام المنضبط والحدود الصارمة](#6-بيانات-السياق-contextwithvalue-الاستخدام-المنضبط-والحدود-الصارمة)
7. [التكامل مع المنظومة والمكتبات القياسية (Ecosystem Integration)](#7-التكامل-مع-المنظومة-والمكتبات-القياسية-ecosystem-integration)
8. [التعامل مع الأخطاء والتشخيص (Error Handling & Diagnostics)](#8-التعامل-مع-الأخطاء-والتشخيص-error-handling--diagnostics)
9. [استراتيجيات الاختبار المتقدمة مع Context (Testing Strategies)](#9-استراتيجيات-الاختبار-المتقدمة-مع-context-testing-strategies)
10. [مصفوفة الأنماط المضادة الشائعة (Anti-Patterns Matrix)](#10-مصفوفة-الأنماط-المضادة-الشائعة-anti-patterns-matrix)
11. [قائمة مراجعة الإنتاج للمشاريع الضخمة (Production Checklist)](#11-قائمة-مراجعة-الإنتاج-للمشاريع-الضخمة-production-checklist)
12. [المصادر والمراجع الرسمية](#12-المصادر-والمراجع-الرسمية)

---

## 1. الفلسفة التأسيسية: ما هو `context.Context` ولماذا تم ابتكاره؟

في الأنظمة الموزعة وخوادم الويب المبنية بلغة Go، يمر معالجة الطلب الواحد (Incoming Request) عبر شبكة من الـ Goroutines المستقلة: استعلامات قاعدة بيانات، اتصالات بـ RPCs خارجية، كتابة في سجلات النظام، وعمليات تشفير.

بدون آلية موحدة لتنسيق هذه الـ Goroutines، تنشأ معضلتان كارثيتان في بيئات الإنتاج:

1. **العمل المهدر (Wasted Work):** إذا قطع العميل اتصاله (Client Disconnect)، يستمر الخادم في معالجة طلب مكلف واستهلاك وحدة المعالجة المركزية (CPU) والذاكرة دون فائدة.
2. **تسريب الـ Goroutines (Goroutine Leaks):** تعليق الـ Goroutines إلى الأبد عند انتظار عمليات دخل/خرج (I/O) لا تنتهي أو قنوات (Channels) لا يُرسل إليها أحد.

ابتكرت Google حزمة `context` لتوفير قناة قياسية موحدة عابرة للحدود (Cross-boundary coordination) تؤدي ثلاث وظائف جوهرية:

```text
┌──────────────────────────────────────────────────────────────────┐
│                         context.Context                          │
├─────────────────┬────────────────────────────┬───────────────────┤
│ 1. الإلغاء      │ 2. المهل الزمنية           │ 3. بيانات النطاق  │
│  (Cancellation) │  (Deadlines & Timeouts)    │  (Scoped Values)  │
│                 │                            │                   │
│ إيقاف العمليات  │ وضع حد أقصى لعمر العملية   │ نقل المعرّفات     │
│ فور عدم الحاجة  │ لمنع التوقف الدائم         │ (Trace/Auth IDs)  │
└─────────────────┴────────────────────────────┴───────────────────┘
```

> **المصدر الرسمي:** مقال مدونة Go: [Go Concurrency Patterns: Context](https://go.dev/blog/context) للمهندس Sameer Ajmani.

### تشريح واجهة `context.Context`

تتكون الواجهة من 4 توابع فقط، وهي مصممة لتكون آمنة تماماً للاستخدام المتزامن من عدة Goroutines في نفس الوقت (Thread-safe):

```go
type Context interface {
    // Deadline يعيد الوقت الذي سيتم فيه إلغاء السياق (إن وُجد)
    Deadline() (deadline time.Time, ok bool)

    // Done يعيد قناة مغلقة (Closed Channel) عند إلغاء السياق أو انتهاء مهلته
    Done() <-chan struct{}

    // Err يعيد سبب الإلغاء بعد غلق قناة Done
    Err() error

    // Value يسترجع البيانات المرتبطة بمفتاح معين عبر مسار الاستدعاء
    Value(key any) any
}
```

### بنية الشجرة الشجرية (Context Tree Hierarchy)

السياقات في Go غير قابلة للتعديل (Immutable). عند اشتقاق سياق جديد، يتم إنشاء عقدة ابن ترتبط بوالدها. تنتقل إشارة الإلغاء **من الأعلى إلى الأسفل دائماً** (Top-Down Cancellation)، ولا يمكن لابن إلغاء والده أو إخوته:

```text
                     ┌──────────────────┐
                     │ context.Background()│ (الجذر - Root)
                     └─────────┬────────┘
                               │
                     ┌─────────▼────────┐
                     │ WithTimeout(30s) │ (سياق الطلب الرئيسي)
                     └────┬────────┬────┘
                          │        │
          ┌───────────────┘        └────────────────┐
          ▼                                         ▼
┌──────────────────┐                      ┌──────────────────┐
│  WithCancel()    │ (استعلام الـ DB)     │ WithValue(Auth)  │ (استدعاء خدمة دفع)
└──────────────────┘                      └──────────────────┘
```

إذا أُلغي السياق الأب (Timeout)، تُلغى جميع السياقات المشتقة منه تلقائياً وفورياً.

---

## 2. التطور المعياري لحزمة `context` عبر إصدارات Go

لم تكن الحزمة دائماً جزءاً من المكتبة القياسية؛ فقد مرت برحلة نضج هندسي مستمرة:

| الإصدار | الميزة المضافة | الأثر المعماري في المشاريع الكبيرة |
| :--- | :--- | :--- |
| **Go 1.7** | نقل الحزمة من `golang.org/x/net/context` إلى المكتبة القياسية `context` | اعتماد السياق رسمياً كمعيار أساسي في `net/http` و `database/sql`. |
| **Go 1.16** | إضافة `signal.NotifyContext` في حزمة `os/signal` | تبسيط الإيقاف السلس للخدمات (Graceful Shutdown) دون كتابة قنوات معقدة يدوياً. |
| **Go 1.20** | إضافة `context.WithCancelCause` و `context.Cause` | حل مشكلة "لماذا أُلغي السياق؟" بإرفاق خطأ مخصص يوضح السبب الجذري للإلغاء. |
| **Go 1.21** | إضافة `context.WithoutCancel` | فصل مهام الخلفية (Background Tasks) عن دورة حياة الطلب دون فقدان بيانات التتبع (Telemetry/Logging). |
| **Go 1.21** | إضافة `context.AfterFunc` | تسجيل دوال تنظيف تُستدعى فور الإلغاء بدون حجز Goroutine مستمر لكل عملية حظر. |
| **Go 1.21** | إضافة `context.WithDeadlineCause` و `WithTimeoutCause` | إرفاق خطأ سببي مخصص عند تجاوز المهلة الزمنية. |
| **Go 1.24** | إضافة `testing.T.Context()` و `b.Context()` | إلغاء السياق تلقائياً فور انتهاء الاختبار ومنع تسريب الموارد بين الحالات التجريبية. |

---

## 3. قواعد التمرير وتصميم واجهات البرمجة (API Design & Propagation)

وضعت شركة Google ودليل Uber معايير صارمة لتمرير السياق داخل كتل الأكواد البرمجية للمشاريع الكبيرة:

### 3.1 القاعدة الذهبية: `ctx` هو المعامل الأول دائماً

يجب أن يكون `ctx context.Context` المعامل الأول لأي دالة تنفذ عمليات دخل وخرج (I/O)، أو تستدعي خدمات شبكية، أو تستهلك وقتاً طويلاً:

```go
// صحيح - المطابق لمعايير Go الرسمية
func FetchUserProfile(ctx context.Context, userID string) (*UserProfile, error)

// خاطئ - انتهاك لاتفاقيات التسمية والترتيب
func FetchUserProfile(userID string, ctx context.Context) (*UserProfile, error)
```

> **المصدر:** [Google Go Style Guide - Context Parameters](https://google.github.io/styleguide/go/decisions#context-parameters)

### 3.2 حظر تخزين `context.Context` داخل الـ Structs

تخزين السياق داخل بنية المعطيات (Struct) يُعد من أسوأ الأنماط المضادة انتشاراً، لأنه يخفي دورة حياة السياق ويشوش نطاق التنفيذ.

```go
// ❌ خطأ فادح: السياق ليس ملكية للكائن (Struct State)
type OrderRepository struct {
    ctx context.Context // تجنب هذا النمط نهائياً
    db  *sql.DB
}

func (r *OrderRepository) FindOrder(id string) (*Order, error) {
    return r.db.QueryRowContext(r.ctx, ...) // سياق مشترك قد يكون منتهي الصلاحية!
}
```

```go
// ✅ صحيح: السياق عابر ويمر صراحة مع كل طلب
type OrderRepository struct {
    db *sql.DB
}

func (r *OrderRepository) FindOrder(ctx context.Context, id string) (*Order, error) {
    return r.db.QueryRowContext(ctx, "SELECT ... WHERE id = $1", id)
}
```

> **الاستثناء الوحيد:** يُسمح بتخزين السياق فقط عندما تكون البنية نفسها تمثل رسالة طلب عابرة مصممة للحفاظ على التوافقية العكسية مثل `http.Request` في المكتبة القياسية، أو رسائل RPC مؤقتة. ولا يُسمح به أبداً في كائنات الخدمات (Services) أو المستودعات (Repositories) أو العمال (Workers).
> **المصدر:** مقال مدونة Go الرسمي [Contexts and structs](https://go.dev/blog/context-and-structs) بقلم Jean Barkhuysen & Matt T. Proud.

### 3.3 تجنب تمرير `nil` نهائياً: استعمل `Background` أو `TODO`

لا تمرر `nil` كقيمة لسياق على الإطلاق، لأن استدعاء `ctx.Done()` أو `ctx.Value()` على مؤشر فارغ سينهار عبر `panic`:

```go
// ❌ خطأ كارثي
result, err := client.DoSomething(nil)

// ✅ عند نقطة البداية (Entrypoint: main, server handler, background worker)
ctx := context.Background()

// ✅ عند العمل على كود قيد التطوير أو دالة لم يتحدد مصدر سياقها بعد
ctx := context.TODO()
```

### 3.4 الحذر من تظليل المتغيرات (Variable Shadowing)

عند إعادة اشتقاق سياق داخل شرط `if`، احذر من استخدام `:=` التي تُعرّف متغيراً محلياً جديداً ينتهي بنهاية الكتلة:

```go
// ❌ خطأ: ctx داخل if هو متغير جديد وتأثيره لا يخرج خارج الكتلة
func Process(ctx context.Context, hasTimeout bool) error {
    if hasTimeout {
        ctx, cancel := context.WithTimeout(ctx, 5*time.Second) // ظلال!
        defer cancel()
    }
    return doWork(ctx) // هنا ctx هو السياق الأصلي غير المقيد بمهلة!
}

// ✅ صحيح: إعادة التعيين مع إدارة دالة الإلغاء
func Process(ctx context.Context, hasTimeout bool) error {
    if hasTimeout {
        var cancel context.CancelFunc
        ctx, cancel = context.WithTimeout(ctx, 5*time.Second)
        defer cancel()
    }
    return doWork(ctx)
}
```

---

## 4. إدارة الإلغاء ومكافحة تسريب الـ Goroutines والذاكرة

### 4.1 حتمية استدعاء `defer cancel()` فوراً

كل استدعاء للدوال المشتقة `WithCancel` أو `WithTimeout` أو `WithDeadline` يعيد دالة إلغاء `cancel()`. يجب استدعاؤها عبر `defer` فور التحقق من السياق، حتى لو انتهت الدالة بنجاح أو عبر خطأ:

```go
func QueryDownstream(ctx context.Context) error {
    ctx, cancel := context.WithTimeout(ctx, 2*time.Second)
    defer cancel() // ⚠️ إلزامي فوراً لمنع تسريب الموارد

    return executeRPC(ctx)
}
```

**لماذا `defer cancel()` إلزامية حتى مع `WithTimeout`؟**  
لأن `WithTimeout` ينشئ مؤقتاً في الذاكرة (Internal Timer) ويضيف العقدة كابن في الشجرة. إذا انتهت الدالة بعد 10ms، يظل المؤقت وعقدة السياق محجوزين في الذاكرة حتى مرور الـ 2s الكاملة ما لم يتم استدعاء `cancel()` لتحريرهما فوراً.

### 4.2 نمط مراقبة الإلغاء داخل الحلقات التكرارية والـ Workers

الـ Goroutines لا تتوقف تلقائياً بمجرد إلغاء السياق؛ يجب على الكود فحص قناة `ctx.Done()` صراحة:

```go
func EventWorker(ctx context.Context, events <-chan Event) error {
    for {
        select {
        case <-ctx.Done():
            // تم إلغاء السياق أو انتهت المهلة: تنظيف الموارد والخروج فوراً
            return ctx.Err()

        case ev, ok := <-events:
            if !ok {
                return nil // أغلقت القناة
            }
            if err := processEvent(ctx, ev); err != nil {
                return err
            }
        }
    }
}
```

### 4.3 المهام الخلفية المنفصلة: حظر تمرير سياق الطلب واستخدام `WithoutCancel`

واحدة من أكبر الثغرات في خوادم الويب: إطلاق Goroutine لتنفيذ مهمة غير متزامنة (مثل إرسال بريد إلكتروني أو تسجيل عملية تدقيق Audit Log) وتمرير سياق طلب الـ HTTP إليها:

```go
// ❌ كارثة إنتاج: بمجرد انتهاء دالة الـ Handler، يُلغى r.Context()
// مما يتسبب في فشل المهمة الخلفية فوراً بإلغاء غير مقصود
func handleOrder(w http.ResponseWriter, r *http.Request) {
    order := createOrder(r)
    
    go func() {
        // خطأ: r.Context() سيلغى بعد إرسال الاستجابة للعميل
        auditService.Record(r.Context(), order) 
    }()
    
    w.WriteHeader(http.StatusCreated)
}
```

#### الحل النموذجي في Go 1.21+: `context.WithoutCancel`

دالة `WithoutCancel` تفصل إشارة الإلغاء وانتهاء المهلة عن السياق الأب، **مع الحفاظ التام على القيم المرفقة (Trace ID, Request ID, Slog logger, Auth Claims)**:

```go
// ✅ صحيح ومحدث (Go 1.21+): فصل الإلغاء مع بقاء سياق القياس والتتبع
func handleOrder(w http.ResponseWriter, r *http.Request) {
    order := createOrder(r)
    
    // إنشاء سياق مستقل غير قابل للإلغاء بإلغاء الطلب
    detachedCtx := context.WithoutCancel(r.Context())
    
    go func() {
        // تقييد المهمة بمهلة زمنية مستقلة معقولة
        bgCtx, cancel := context.WithTimeout(detachedCtx, 15*time.Second)
        defer cancel()
        
        auditService.Record(bgCtx, order)
    }()
    
    w.WriteHeader(http.StatusCreated)
}
```

### 4.4 التزامن المنضبط وتفادي التسريب: `errgroup.WithContext`

عند تشغيل عدة مهام متوازية تابعة لنفس المعاملة، يُعد `golang.org/x/sync/errgroup` المعيار العالمي:

```go
import "golang.org/x/sync/errgroup"

func FetchDashboardData(ctx context.Context, userID string) (*Dashboard, error) {
    g, ctx := errgroup.WithContext(ctx)
    // تحديد حد أقصى للـ goroutines المتزامنة لمنع إغراق الموارد
    g.SetLimit(10)

    var (
        user    *User
        orders  []Order
        metrics *Metrics
    )

    g.Go(func() error {
        var err error
        user, err = fetchUser(ctx, userID)
        return err // إذا فشل هذا، تُلغى المهام الأخرى فوراً عبر ctx
    })

    g.Go(func() error {
        var err error
        orders, err = fetchOrders(ctx, userID)
        return err
    })

    g.Go(func() error {
        var err error
        metrics, err = fetchMetrics(ctx, userID)
        return err
    })

    // انتظار جميع العمليات أو أول خطأ
    if err := g.Wait(); err != nil {
        return nil, err
    }

    return &Dashboard{User: user, Orders: orders, Metrics: metrics}, nil
}
```

---

## 5. ميزانية المهل الزمنية وانتقالها عبر الخدمات (Timeouts & Deadlines)

### 5.1 حتمية تقليص المهل (Deadline Monotonicity)

قاعدة رياضية ثابتة في Go: **السياق الابن لا يمكنه أبداً تمديد مهلة السياق الأب**. إذا كان سياق الأب سينتهي بعد 5 ثوانٍ، واستدعيت `WithTimeout(ctx, 10*time.Second)`، فإن السياق الناتج سينتهي حتماً بعد 5 ثوانٍ فقط:

$$\text{Effective Deadline} = \min(\text{Parent Deadline}, \text{Child Deadline})$$

```go
parentCtx, cancelParent := context.WithTimeout(context.Background(), 5*time.Second)
defer cancelParent()

// المهلة الفعلية لـ childCtx ستكون 5 ثوانٍ وليس 10 ثوانٍ!
childCtx, cancelChild := context.WithTimeout(parentCtx, 10*time.Second)
defer cancelChild()
```

### 5.2 توزيع ميزانية الوقت (Deadline Budgeting)

في البنى الموزعة، إذا كانت المهلة الإجمالية لمعالجة الطلب هي 3 ثوانٍ، لا يجوز منح كل استدعاء فرعي مهلة 3 ثوانٍ، بل يجب توزيع الميزانية وترك هامش أمان (Buffer) لمعالجة التراجع والأخطاء:

```text
إجمالي ميزانية الطلب (3000ms)
├───────────────────────────────┬───────────────────────────────┬────────────┐
│ قاعدة البيانات (1000ms max)   │ خدمة الدفع (1500ms max)        │ أمان (500ms)│
└───────────────────────────────┴───────────────────────────────┴────────────┘
```

```go
func ProcessCheckout(ctx context.Context) error {
    // 1. استعلام قاعدة البيانات بهامش زمني محدد
    dbCtx, dbCancel := context.WithTimeout(ctx, 1*time.Second)
    defer dbCancel()
    if err := saveDraft(dbCtx); err != nil {
        return fmt.Errorf("draft save failed: %w", err)
    }

    // 2. التحقق من بقاء وقت كافٍ قبل المتابعة للعملية المكلفة
    if deadline, ok := ctx.Deadline(); ok {
        if time.Until(deadline) < 500*time.Millisecond {
            return fmt.Errorf("insufficient deadline remaining for payment: %w", context.DeadlineExceeded)
        }
    }

    // 3. استدعاء بوابة الدفع
    paymentCtx, payCancel := context.WithTimeout(ctx, 1500*time.Millisecond)
    defer payCancel()
    return chargePayment(paymentCtx)
}
```

---

## 6. بيانات السياق (`context.WithValue`): الاستخدام المنضبط والحدود الصارمة

أكثر أجزاء حزمة `context` تعرضاً لسوء الاستخدام هي `WithValue`. أساء مبرمجون كثر فهمها واعتبروها حاوية عامة (Global Map) أو أداة لحقن التبعيات (Dependency Injection).

### 6.1 مصفوفة الفصل الصارم: ما يُسمح به وما يُمنع

| المحتوى | الحكم | التعليل الهندسي |
| :--- | :---: | :--- |
| **Trace ID / Span Context (OpenTelemetry)** | ✅ مسموح | بيانات تتبع ملازمة للطلب وتنتقل عبر كل الطبقات. |
| **Correlation ID / Request ID** | ✅ مسموح | لربط السجلات (Logs) الخاصة بنفس دورة الطلب. |
| **هوية المستخدم الموثق (Auth Claims / Subject ID)** | ✅ مسموح | يُحقن في Middleware الأمان ويُستهلك في طبقة الصلاحيات. |
| **اتصال قاعدة البيانات (`*sql.DB`)** | ❌ ممنوع قطعاً | يخفي التبعيات (Service Locator Anti-Pattern) ويُعطل التحقق النوعي (Type Safety). |
| **المعاملات الاختيارية للدوال (Optional Args)** | ❌ ممنوع قطعاً | يُعقد عقود الواجهات ويجعل توقيع الدالة غير صادق (Dishonest Signature). |
| **إعدادات التكوين (Configuration)** | ❌ ممنوع قطعاً | يجب تمرير الإعدادات عبر البواني (Constructors) صراحة. |

> **قاعدة ذهبية:** بيانات السياق هي للبيانات التي تخص **الرحلة والطلب (Request Scope)**، وليست للمكونات التي تخص **بنية النظام والخدمة (System Scope)**.

### 6.2 الحماية من تصادم المفاتيح: نمط الأنواع غير المصدرة (Unexported Keys)

استخدام النصوص البدائية (`string`) كمفاتيح في `WithValue` كارثة معمارية تؤدي إلى تصادم الحزم ومسح البيانات دون إنذار:

```go
// ❌ خطأ: حزمتان تستخدمان المفتاح "user_id" ستتصادمان حتماً
ctx = context.WithValue(ctx, "user_id", "12345")
```

#### النمط المعياري المعتمد عالمياً

```go
package requestid

import "context"

// 1. تعريف نوع خاص غير مصدّر (حجمه صفر بايت في الذاكرة)
type contextKey struct{}

// 2. تعريف متغير المفتاح غير مصدّر
var key = contextKey{}

// 3. دالة آمنة نوعياً لحقن القيمة
func WithRequestID(ctx context.Context, requestID string) context.Context {
    return context.WithValue(ctx, key, requestID)
}

// 4. دالة آمنة نوعياً لاستخراج القيمة دون كشف المفتاح للعملاء
func FromContext(ctx context.Context) (string, bool) {
    id, ok := ctx.Value(key).(string)
    return id, ok
}
```

### 6.3 كفاءة الأداء وتفادي تعقيد شجرة البحث $O(N)$

داخلياً، يتم تمثيل `valueCtx` في Go كقائمة مترابطة فردية (Single Linked List). في كل مرة تستدعي فيها `WithValue`، تُنشئ عقدة جديدة تغلف العقدة السابقة. إذا قمت بحقن 20 خاصية منفصلة في حلقة تكرارية، فإن استرجاع أي خاصية سيتطلب مسحاً خطياً بقيمة $O(N)$.

#### نمط التجميع في كائن هيكلي موحد (Bundling Pattern)

إذا كان الطلب يحتاج إلى عدة سمات تعريفية، قم بجمعها في بنية واحدة وحقنها مرة واحدة:

```go
//a ✅ صحيح: حقن عقدة واحدة تجمع البيانات السياقية
type RequestMetadata struct {
    RequestID string
    TenantID  string
    UserID    string
    ClientIP  string
}

type metadataKey struct{}

func WithMetadata(ctx context.Context, meta *RequestMetadata) context.Context {
    return context.WithValue(ctx, metadataKey{}, meta)
}

func GetMetadata(ctx context.Context) (*RequestMetadata, bool) {
    meta, ok := ctx.Value(metadataKey{}).(*RequestMetadata)
    return meta, ok
}
```

---

## 7. التكامل مع المنظومة والمكتبات القياسية (Ecosystem Integration)

### 7.1 مع خوادم وعملاء HTTP

#### في الخوادم (HTTP Server)

استخرج السياق دائماً من `r.Context()`، ولا تنشئ سياقاً جديداً إلا إذا أردت تقييده بمهلة أقصر:

```go
func OrderHandler(w http.ResponseWriter, r *http.Request) {
    // السياق مرتبط بـ socket العميل؛ إذا أغلق المتصفح، يُلغى ctx فوراً
    ctx := r.Context()

    // تقييد المعالجة بمهلة أقصاها 5 ثوانٍ
    ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
    defer cancel()

    order, err := service.CreateOrder(ctx, r.Body)
    if err != nil {
        handleHTTPError(w, err)
        return
    }
    json.NewEncoder(w).Encode(order)
}
```

#### في عملاء HTTP (HTTP Client)

استخدم دائماً `http.NewRequestWithContext` وتجنب `http.Get` أو `http.NewRequest` القديمة:

```go
func CallExternalAPI(ctx context.Context, url string) (*Response, error) {
    req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
    if err != nil {
        return nil, fmt.Errorf("failed to create request: %w", err)
    }

    resp, err := httpClient.Do(req)
    if err != nil {
        return nil, fmt.Errorf("external call failed: %w", err)
    }
    defer resp.Body.Close()
    
    return parseResponse(resp)
}
```

### 7.2 مع قواعد البيانات (`database/sql`)

تجاهل الدوال القديمة (`db.Query`, `db.Exec`) واستخدم توابع السياق حصراً:

```go
func GetAccountBalance(ctx context.Context, db *sql.DB, accID string) (int64, error) {
    var balance int64
    // إذا قطع العميل اتصاله، تلغي Go الاستعلام في محرك قاعدة البيانات فوراً
    err := db.QueryRowContext(ctx, "SELECT balance FROM accounts WHERE id = $1", accID).Scan(&balance)
    if err != nil {
        return 0, fmt.Errorf("failed to query balance: %w", err)
    }
    return balance, nil
}
```

#### إدارة المعاملات (Transactions)

```go
func TransferFunds(ctx context.Context, db *sql.DB, from, to string, amount int64) error {
    tx, err := db.BeginTx(ctx, &sql.TxOptions{Isolation: sql.LevelReadCommitted})
    if err != nil {
        return err
    }
    defer tx.Rollback() // يتراجع تلقائياً إذا فشل السياق أو لم يتم الـ Commit

    if _, err := tx.ExecContext(ctx, "UPDATE ...", from, amount); err != nil {
        return err
    }
    if _, err := tx.ExecContext(ctx, "UPDATE ...", to, amount); err != nil {
        return err
    }

    return tx.Commit()
}
```

### 7.3 الإيقاف السلس للخدمات: `signal.NotifyContext` (Go 1.16+)

النمط المعياري لإنهاء الخدمات والـ Microservices دون فقدان أي طلب قيد المعالجة:

```go
func RunServer() error {
    // 1. إنشاء سياق يستمع لإشارات إنهاء النظام (SIGINT, SIGTERM)
    ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
    defer stop()

    srv := &http.Server{
        Addr:    ":8080",
        Handler: setupRoutes(),
    }

    go func() {
        if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
            slog.Error("server failed to start", "error", err)
            os.Exit(1)
        }
    }()

    slog.Info("server is running on :8080")

    // 2. الانتظار حتى تصل إشارة الإنهاء
    <-ctx.Done()
    slog.Info("shutdown signal received, draining connections...")

    // 3. سياق بمهلة 10 ثوانٍ للسماح بإنهاء الطلبات الحالية
    shutdownCtx, cancelShutdown := context.WithTimeout(context.Background(), 10*time.Second)
    defer cancelShutdown()

    if err := srv.Shutdown(shutdownCtx); err != nil {
        return fmt.Errorf("server forced to shutdown: %w", err)
    }

    slog.Info("server exited gracefully")
    return nil
}
```

### 7.4 تنظيف الموارد الفوري دون Goroutines: `context.AfterFunc` (Go 1.21+)

قبل Go 1.21، لإغلاق اتصال أو مقبس عند إلغاء السياق، كان يجب تشغيل Goroutine مستمر ينتظر `<-ctx.Done()`. مع `context.AfterFunc`، يتم تسجيل دالة تنفذ فور الإلغاء بأعلى كفاءة:

```go
func ReadWithContext(ctx context.Context, conn net.Conn, buf []byte) (int, error) {
    // تسجيل إغلاق الاتصال فور إلغاء السياق لإيقاف الـ Read العالقة
    stop := context.AfterFunc(ctx, func() {
        conn.SetReadDeadline(time.Now()) // يفك حظر دالة Read فوراً
    })
    defer stop() // إلغاء التسجيل إذا اكتملت القراءة بنجاح دون الحاجة للتنفيذ

    return conn.Read(buf)
}
```

---

## 8. التعامل مع الأخطاء والتشخيص (Error Handling & Diagnostics)

### 8.1 التمييز بين الإلغاء وتجاوز المهلة

عند فحص الخطأ الناتج عن عملية تستخدم السياق، يجب استخدام `errors.Is`:

```go
if err := service.Execute(ctx); err != nil {
    switch {
    case errors.Is(err, context.Canceled):
        // قطع الاتصال من قبل العميل أو إلغاء مقصود من الخادم
        slog.Warn("operation was canceled by caller")
        http.Error(w, "Request Canceled", 499) // Client Closed Request

    case errors.Is(err, context.DeadlineExceeded):
        // انتهت المهلة المحددة
        slog.Error("operation timed out")
        http.Error(w, "Gateway Timeout", http.StatusGatewayTimeout) // 504

    default:
        // خطأ أعمال أو خادم داخلي
        slog.Error("internal processing error", "error", err)
        http.Error(w, "Internal Server Error", http.StatusInternalServerError) // 500
    }
}
```

### 8.2 تشخيص السبب الجذري: `WithCancelCause` و `Cause` (Go 1.20+)

قبل Go 1.20، كانت دالة `cancel()` تلغي السياق ويعيد `ctx.Err()` فقط `context.Canceled` العامة. لا يمكنك معرفة: هل أُلغي بسبب فشل التحقق؟ أم استهلاك حصة الاستخدام؟ أم انقطاع اتصال قاعدة البيانات؟

مع `WithCancelCause`، يمكنك تمرير خطأ وصفي يُسترجع عبر `context.Cause(ctx)`:

```go
var (
    ErrQuotaExceeded    = errors.New("rate limit: quota exceeded")
    ErrCircuitTripped   = errors.New("circuit breaker: open state")
)

func OrchestrateTasks(ctx context.Context) error {
    ctx, cancel := context.WithCancelCause(ctx)
    defer cancel(nil)

    go func() {
        if quotaExceeded() {
            // إلغاء السياق مع تحديد السبب الدقيق
            cancel(ErrQuotaExceeded)
        }
    }()

    <-ctx.Done()

    // استخراج السبب الجذري
    cause := context.Cause(ctx)
    if errors.Is(cause, ErrQuotaExceeded) {
        return fmt.Errorf("orchestration aborted due to rate limit: %w", cause)
    }

    return ctx.Err()
}
```

---

## 9. استراتيجيات الاختبار المتقدمة مع Context (Testing Strategies)

### 9.1 المعيار الحديث في Go 1.24+: `t.Context()`

في Go 1.24، أصبح لدى `*testing.T` و `*testing.B` تابع `t.Context()` مدمج، يضمن إلغاء السياق وتنظيف كافة العمليات التابعة له بمجرد انتهاء الاختبار، مما يغني عن الرموز المتكررة:

```go
func TestOrderService_Create(t *testing.T) {
    // Go 1.24+: سياق مربوط تلقائياً بدورة حياة الاختبار
    ctx := t.Context()

    svc := NewOrderService(testDB)
    order, err := svc.CreateOrder(ctx, validOrderReq)

    if err != nil {
        t.Fatalf("expected success, got: %v", err)
    }
    if order.ID == "" {
        t.Errorf("expected generated ID, got empty")
    }
}
```

### 9.2 نمط ما قبل Go 1.24: التنظيف الصريح عبر `t.Cleanup`

```go
func TestLegacyProcess(t *testing.T) {
    ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
    t.Cleanup(cancel) // ضمان إلغاء السياق حتى لو فشل الاختبار عبر t.FailNow()

    err := executeWorkflow(ctx)
    if err != nil {
        t.Fatalf("workflow failed: %v", err)
    }
}
```

### 9.3 اختبار سيناريوهات الإلغاء الفوري (Pre-Canceled Context)

للتحقق من أن دوالك ترفض العمل وتفشل سريعاً (Fail-Fast) عند استقبال سياق ملغى مسبقاً:

```go
func TestService_FailFastOnCanceledContext(t *testing.T) {
    ctx, cancel := context.WithCancel(context.Background())
    cancel() // إلغاء السياق قبل تمريره للدالة

    svc := NewOrderService(testDB)
    _, err := svc.CreateOrder(ctx, validOrderReq)

    if !errors.Is(err, context.Canceled) {
        t.Errorf("expected context.Canceled error, got: %v", err)
    }
}
```

---

## 10. مصفوفة الأنماط المضادة الشائعة (Anti-Patterns Matrix)

| النمط المضاد (Anti-Pattern) | المخاطر المترتبة | الكود الخاطئ | التصحيح المعياري المعتمد |
| :--- | :--- | :--- | :--- |
| **تخزين السياق في Struct** | سياق قديم، ارتباك في دورة الحياة، أخطاء متزامنة | `type Svc struct { ctx context.Context }` | تمرير `ctx` كأول معامل في كل تابع: `func (s *Svc) Do(ctx context.Context)` |
| **تمرير `nil` كقيمة سياق** | انهيار التطبيق (`panic: runtime error: invalid memory address`) | `svc.Call(nil, data)` | تمرير `context.Background()` أو `context.TODO()` |
| **إهمال استدعاء `cancel()`** | تسريب موارد الذاكرة ومؤقتات النظام (Timer Leaks) | `ctx, _ := context.WithTimeout(p, 5*time.Second)` | `ctx, cancel := context.WithTimeout(p, 5*time.Second)`<br>`defer cancel()` |
| **تمرير سياق الطلب لمهام خلفية** | إلغاء المهمة فور انتهاء استجابة الـ HTTP للعميل | `go worker.Process(r.Context(), data)` | استخدام `context.WithoutCancel(r.Context())` مع مهلة جديدة |
| **استخدام نصوص كمفاتيح `WithValue`** | تصادم المفاتيح بين الحزم واستبدال القيم خفية | `context.WithValue(ctx, "trace_id", id)` | تعريف نوع خاص غير مصدّر: `type key struct{}` |
| **حقن التبعيات عبر `WithValue`** | كسر التحقق النوعي، غياب الوضوح، Service Locator | `db := ctx.Value("db").(*sql.DB)` | تمرير التبعيات في باني الكائن: `NewService(db *sql.DB)` |
| **تظليل المتغيرات (Shadowing)** | استخدام سياق غير مقيد بالمهلة خارج شرط `if` | `if cond { ctx, cancel := context.WithTimeout(...) }` | `var cancel context.CancelFunc`<br>`ctx, cancel = context.WithTimeout(...)` |
| **الاستماع فقط لقناة البيانات في الـ Goroutines** | تسريب الـ Goroutine إلى الأبد عند عدم وصول بيانات | `for msg := range ch { ... }` | استخدام `select` والمراقبة المتزامنة مع `case <-ctx.Done():` |
| **تفريغ مصفوفات القيم بعمق شجرة $O(N)$** | بطء ملحوظ في استرجاع القيم وزيادة استهلاك الذاكرة | استدعاء `WithValue` 30 مرة في حلقة تكرارية | تجميع الخصائص في `struct` موحد وحقنه مرة واحدة |

---

## 11. قائمة مراجعة الإنتاج للمشاريع الضخمة (Production Checklist)

قبل إطلاق أي خدمة Go إلى بيئة الإنتاج، تأكد من استيفاء المعايير التالية أثناء مراجعة الكود (Code Review):

### أ. تصميم الدوال والواجهات (API Contracts)

- [ ] هل كل دالة تنفذ عمليات شبكة أو دخل/خرج أو تتطلب وقتاً تقبل `ctx context.Context` كأول معامل؟
- [ ] هل تخلو كافة الـ Structs في المشروع من أي حقل من نوع `context.Context` (باستثناء متطلبات التوافق العكسي النادرة)؟
- [ ] هل تم الامتناع تماماً عن تمرير `nil` كقيمة سياق واستبدالها بـ `context.Background()` أو `context.TODO()`؟

### ب. إدارة الإلغاء والمهل (Lifecycles & Timeouts)

- [ ] هل يُستدعى `defer cancel()` فور إنشاء أي سياق بواسطة `WithCancel` أو `WithTimeout` أو `WithDeadline`؟
- [ ] هل تم تحديد مهلة زمنية عليا (Timeout/Deadline) للخدمات الخارجية واستعلامات قواعد البيانات؟
- [ ] هل كل حلقة تكرارية غير متزامنة أو عامل خلفي (Worker) يتضمن `case <-ctx.Done():` للخروج النظيف؟
- [ ] هل تم عزل المهام غير المتزامنة المنفصلة عن سياق طلب الـ HTTP باستخدام `context.WithoutCancel(ctx)`؟
- [ ] هل تم استخدام `errgroup.WithContext` للعمليات المتوازية مع وضع حد أقصى للتزامن عبر `SetLimit`؟

### ج. سلامة بيانات السياق (Context Values)

- [ ] هل جميع مفاتيح `context.WithValue` مبنية على أنواع مخصصة غير مصدّرة (`unexported types`)؟
- [ ] هل تقتصر البيانات المحقونة على بيانات نطاق الطلب (Trace ID, Request ID, User Claims) وخلوها تماماً من كائنات قواعد البيانات والإعدادات؟
- [ ] هل توجد دوال وصول آمنة نوعياً (`With...` و `FromContext`) تمنع كشف المفاتيح والتحويلات النوعية اليدوية في كود الأعمال؟

### د. التكامل والجاهزية التشغيلية (Observability & Shutdown)

- [ ] هل تم ربط إشارات إغلاق النظام (`SIGINT`, `SIGTERM`) عبر `signal.NotifyContext` لتنفيذ الإيقاف السلس (Graceful Shutdown)؟
- [ ] هل يتم استخدام توابع `*Context` في مكتبة `database/sql` حصراً؟
- [ ] هل يتم التمييز في طبقة الـ HTTP و gRPC بين `context.Canceled` (499) و `context.DeadlineExceeded` (504)؟
- [ ] هل تستفيد المشروعات الحديثة من `context.WithCancelCause` لتسجيل الأسباب الجذرية الدقيقة للإلغاء؟
- [ ] هل تستخدم الاختبارات التابع المعياري الحديث `t.Context()` في Go 1.24+ لمنع تسريب الموارد التجريبية؟

---

## 12. المصادر والمراجع الرسمية

1. **Go Official Blog:**
   - [Go Concurrency Patterns: Context](https://go.dev/blog/context) — Sameer Ajmani (2014)
   - [Contexts and structs](https://go.dev/blog/context-and-structs) — Jean Barkhuysen & Matt T. Proud (2021)
2. **Go Standard Library Documentation:**
   - [`pkg.go.dev/context`](https://pkg.go.dev/context) — التوثيق الرسمي لحزمة Context وتحديثاتها.
3. **Go Proposals & Design Documents:**
   - [Proposal #51365: context: add WithoutCancel](https://github.com/golang/go/issues/51365)
   - [Proposal #57928: context: add AfterFunc](https://github.com/golang/go/issues/57928)
   - [Proposal #51365: context: add WithCancelCause and Cause](https://github.com/golang/go/issues/51365)
   - [Proposal: testing: add Context method to *T and *B](https://github.com/golang/go/issues/65304) (Go 1.24)
4. **Official Style Guides:**
   - [Google Go Style Guide: Context](https://google.github.io/styleguide/go/decisions#context-parameters)
   - [Google Go Best Practices: Contexts](https://google.github.io/styleguide/go/best-practices#contexts)
   - [Uber Go Style Guide](https://github.com/uber-go/guide/blob/master/style.md)
5. **Architectural References:**
   - Dave Cheney: *Never start a goroutine without knowing how and when it will stop.*
