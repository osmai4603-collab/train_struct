# أفضل الممارسات العالمية للتعامل مع Logger في مشاريع Go الكبيرة

> **تاريخ البحث:** 2026-09-21
> **المصادر:** Go Official Blog, pkg.go.dev, Go Proposal #56345, Peter Bourgon, Dave Cheney

---

## جدول المحتويات

1. [المعيار الرسمي: `log/slog`](#1-المعيار-الرسمي-logslog)
2. [البنية المعمارية: Frontend/Backend](#2-البنية-المعمارية-frontendbackend)
3. [أنماط Dependency Injection](#3-أنماط-dependency-injection)
4. [تحسين الأداء](#4-تحسين-الأداء)
5. [Middleware و Handler المخصص](#5-middleware-و-handler-المخصص)
6. [حماية البيانات الحساسة: `LogValuer`](#6-حماية-البيانات-الحساسة-logvaluer)
7. [الاختبار](#7-الاختبار)
8. [الأنماط المضادة (Anti-Patterns)](#8-الأنماط-المضادة-anti-patterns)
9. [قائمة مراجعة الإنتاج](#9-قائمة-مراجعة-الإنتاج)
10. [المصادر والمراجع](#10-المصادر-والمراجع)

---

## 1. المعيار الرسمي: `log/slog`

حزمة `log/slog` أُضيفت في **Go 1.21** (أغسطس 2023) وهي الآن **المعيار الرسمي** للـ Structured Logging في Go.

> **المصدر الرسمي:** مدونة Go الرسمية — [Structured Logging with slog](https://go.dev/blog/slog)
> الكاتب: Jonathan Amsterdam (فريق Go في Google)

### لماذا `slog`؟

- **مشكلة "فوضى اللوقات" (Log Anarchy):** المشاريع الكبيرة تسحب عشرات التبعيات، كل واحدة تستخدم مكتبة logging مختلفة (`logrus`, `zap`, `zerolog`, `go-kit/log`). النتيجة: صعوبة توحيد التنسيق والوجهة.
- **الحل:** `slog` يوفر **واجهة موحدة** (Frontend) مع **Handler قابل للتبديل** (Backend)، مما يسمح لكل المكتبات بالتحدث بنفس اللغة.

### أبسط استخدام

```go
package main

import "log/slog"

func main() {
    slog.Info("hello, world")
}
// Output: 2023/08/04 16:09:19 INFO hello, world
```

### إضافة Attributes (بيانات مهيكلة)

```go
slog.Info("user login", "user_id", 123, "ip", "1.2.3.4")
// Output: time=... level=INFO msg="user login" user_id=123 ip=1.2.3.4
```

---

## 2. البنية المعمارية: Frontend/Backend

> **المصدر:** Go Proposal Design Doc #56345 + Official Blog

أحد أهم قرارات التصميم في `slog` هو **فصل الواجهة (Frontend) عن التنفيذ (Backend)**:

```text
┌─────────────────────────────────────────────────┐
│                  Application Code               │
│         slog.Info("msg", "key", value)           │
└──────────────────────┬──────────────────────────┘
                       │
              ┌────────▼────────┐
              │   slog.Logger   │  ◄── Frontend (API)
              │   (الواجهة)      │
              └────────┬────────┘
                       │
              ┌────────▼────────┐
              │  slog.Handler   │  ◄── Backend (Interface)
              │   (التنفيذ)      │
              └────────┬────────┘
                       │
          ┌────────────┼────────────┐
          ▼            ▼            ▼
    TextHandler   JSONHandler   Custom/Zap/Zerolog
```

### المكونات الأساسية

| المكون     | الوصف                                             |
|:-----------|:--------------------------------------------------|
| `Logger`   | نقطة الدخول لاستدعاءات اللوق                       |
| `Record`   | حدث لوق واحد (الرسالة + المستوى + الوقت + الـ Attrs) |
| `Handler`  | واجهة التنسيق والإخراج                             |
| `Attr`     | زوج مفتاح-قيمة (البنية الأساسية للـ Structured Log)  |

### تهيئة Handler حسب البيئة

```go
func NewLogger(env string) *slog.Logger {
    var handler slog.Handler

    switch env {
    case "production":
        // JSON للإنتاج — سهل الفلترة في أنظمة المراقبة
        handler = slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{
            Level: slog.LevelInfo,
        })
    default:
        // نص مقروء للتطوير
        handler = slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{
            Level:     slog.LevelDebug,
            AddSource: true, // يضيف اسم الملف ورقم السطر
        })
    }

    return slog.New(handler)
}
```

---

## 3. أنماط Dependency Injection

> **المصدر:** إجماع مجتمع Go + Peter Bourgon (bourgon.org)

### ✅ النمط الموصى به: حقن `*slog.Logger` بشكل صريح

```go
type UserService struct {
    logger *slog.Logger
    db     *sql.DB
}

func NewUserService(logger *slog.Logger, db *sql.DB) *UserService {
    return &UserService{
        // إنشاء logger مُحدَّد النطاق لهذا المكوّن
        logger: logger.With(slog.String("component", "UserService")),
        db:     db,
    }
}

func (s *UserService) GetUser(ctx context.Context, id string) (*User, error) {
    s.logger.InfoContext(ctx, "fetching user", "user_id", id)
    // ...
}
```

### لماذا `*slog.Logger` المباشر بدلاً من واجهة مخصصة؟

| الاعتبار              | واجهة مخصصة (`Logger interface`)  | `*slog.Logger` المباشر          |
|:----------------------|:----------------------------------|:-------------------------------|
| **سهولة الاستخدام**    | تحتاج تعريف + تنفيذ               | جاهز من المكتبة القياسية        |
| **الميزات**            | تفقد `With()`, `WithGroup()`, `LogAttrs()` | الوصول الكامل لكل الميزات      |
| **الأداء**             | overhead إضافي من طبقة التجريد    | أداء أمثل مباشر                |
| **التوحيد**            | كل مشروع يعرّف واجهته              | معيار موحد عبر النظام البيئي    |
| **مؤلفو المكتبات**     | ✅ مقبول (لمرونة المستهلكين)        | ✅ الأفضل للخدمات الداخلية      |

### ⚠️ استثناء: مؤلفو المكتبات العامة

إذا كنت تكتب **مكتبة خارجية** للاستخدام العام، يمكنك تعريف واجهة بسيطة للسماح للمستهلكين باستخدام أي logger يريدونه.

### ❌ تجنب: Logger العالمي

```go
// ❌ سيء — يخفي التبعية ويصعّب الاختبار
func GetUser(id string) {
    slog.Default().Info("fetching user", "user_id", id)
}

// ✅ جيد — التبعية واضحة وقابلة للاختبار
func (s *UserService) GetUser(ctx context.Context, id string) {
    s.logger.InfoContext(ctx, "fetching user", "user_id", id)
}
```

---

## 4. تحسين الأداء

> **المصدر:** Official Go Blog (Performance section) + pkg.go.dev/log/slog

### 4.1 استخدم `LogAttrs` في المسارات الساخنة (Hot Paths)

دالة `slog.Info(msg, key, value...)` تقبل `any` مما يسبب **heap allocations**. في الكود الذي ينفذ آلاف المرات في الثانية، استخدم `LogAttrs`:

```go
// ❌ عادي — allocations بسبب interface{}
slog.Info("request handled", "method", r.Method, "status", status, "duration", d)

// ✅ أمثل — zero-allocation مع Attr types محددة
slog.LogAttrs(ctx, slog.LevelInfo, "request handled",
    slog.String("method", r.Method),
    slog.Int("status", status),
    slog.Duration("duration", d),
)
```

### 4.2 تحقق من `Enabled()` قبل العمليات المكلفة

```go
// ❌ يحسب القيمة حتى لو لن تُسجَّل
logger.Debug("state dump", "state", expensiveStateSnapshot())

// ✅ يتحقق أولاً
if logger.Enabled(ctx, slog.LevelDebug) {
    logger.Debug("state dump", "state", expensiveStateSnapshot())
}
```

### 4.3 استخدم `With()` للبيانات المتكررة

```go
// ❌ يكرر البيانات في كل سطر
logger.Info("step 1", "request_id", reqID, "tenant", tenant)
logger.Info("step 2", "request_id", reqID, "tenant", tenant)
logger.Info("step 3", "request_id", reqID, "tenant", tenant)

// ✅ Pre-formatting مرة واحدة — أسرع وأنظف
reqLogger := logger.With("request_id", reqID, "tenant", tenant)
reqLogger.Info("step 1")
reqLogger.Info("step 2")
reqLogger.Info("step 3")
```

> **ملاحظة من الـ Official Blog:**
> فريق Go وجد أن **95%+ من استدعاءات اللوق تمرر 5 attributes أو أقل**،
> والمكاسب الأكبر في الأداء جاءت من **الانتباه الدقيق لتخصيصات الذاكرة (memory allocations)**.

### 4.4 لا تُفرط في اللوق

- **تجنب** اللوق داخل الحلقات الضيقة (tight loops) إلا للضرورة.
- اللوق المفرط = عنق زجاجة في الـ I/O + تكلفة serialization + ضغط على GC.

---

## 5. Middleware و Handler المخصص

> **المصدر:** pkg.go.dev/log/slog + Go slog Handler Writing Guide

### 5.1 نمط HTTP Middleware (لوق مرتبط بالطلب)

```go
type contextKey string
const loggerKey contextKey = "logger"

func LoggingMiddleware(base *slog.Logger) func(http.Handler) http.Handler {
    return func(next http.Handler) http.Handler {
        return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
            start := time.Now()

            // إنشاء logger مُحدَّد النطاق لهذا الطلب
            reqLogger := base.With(
                slog.String("request_id", uuid.New().String()),
                slog.String("method", r.Method),
                slog.String("path", r.URL.Path),
            )

            // تخزينه في السياق
            ctx := context.WithValue(r.Context(), loggerKey, reqLogger)

            // تنفيذ الطلب
            next.ServeHTTP(w, r.WithContext(ctx))

            // لوق انتهاء الطلب
            reqLogger.Info("request completed",
                slog.Duration("duration", time.Since(start)),
            )
        })
    }
}

// دالة مساعدة لاستخراج الـ Logger من السياق
func LoggerFromContext(ctx context.Context) *slog.Logger {
    if logger, ok := ctx.Value(loggerKey).(*slog.Logger); ok {
        return logger
    }
    return slog.Default()
}
```

### 5.2 كتابة Handler مخصص (Handler Middleware Pattern)

لتنفيذ سلوك مخصص (مثل إخفاء PII، أو إرسال اللوقات لوجهات متعددة):

```go
// Handler يلتف حول handler آخر ويضيف سلوكاً
type SensitiveDataHandler struct {
    inner     slog.Handler
    redactKeys map[string]bool
}

func NewSensitiveDataHandler(inner slog.Handler, keys []string) *SensitiveDataHandler {
    m := make(map[string]bool)
    for _, k := range keys {
        m[k] = true
    }
    return &SensitiveDataHandler{inner: inner, redactKeys: m}
}

func (h *SensitiveDataHandler) Enabled(ctx context.Context, level slog.Level) bool {
    return h.inner.Enabled(ctx, level)
}

func (h *SensitiveDataHandler) Handle(ctx context.Context, r slog.Record) error {
    // نسخ الـ Record مع تعديل الـ Attributes الحساسة
    newRecord := slog.NewRecord(r.Time, r.Level, r.Message, r.PC)
    r.Attrs(func(a slog.Attr) bool {
        if h.redactKeys[a.Key] {
            a.Value = slog.StringValue("***")
        }
        newRecord.AddAttrs(a)
        return true
    })
    return h.inner.Handle(ctx, newRecord)
}

func (h *SensitiveDataHandler) WithAttrs(attrs []slog.Attr) slog.Handler {
    return &SensitiveDataHandler{
        inner:      h.inner.WithAttrs(attrs),
        redactKeys: h.redactKeys,
    }
}

func (h *SensitiveDataHandler) WithGroup(name string) slog.Handler {
    return &SensitiveDataHandler{
        inner:      h.inner.WithGroup(name),
        redactKeys: h.redactKeys,
    }
}
```

### واجهة `slog.Handler` — الأربع دوال المطلوبة

| الدالة                     | الغرض                                           |
|:--------------------------|:------------------------------------------------|
| `Enabled(ctx, Level)`     | فحص سريع قبل معالجة أي attributes                |
| `Handle(ctx, Record)`     | معالجة وتسجيل حدث اللوق                          |
| `WithAttrs([]Attr)`       | إنشاء handler جديد مع attributes إضافية مُسبقة     |
| `WithGroup(string)`       | إنشاء handler جديد مع تجميع الـ attributes          |

---

## 6. حماية البيانات الحساسة: `LogValuer`

> **المصدر:** pkg.go.dev/log/slog — LogValuer interface + Official Blog examples

واجهة `slog.LogValuer` تتحكم بكيفية ظهور النوع في اللوقات:

```go
// نوع يمثل كلمة سر
type SecretString string

// تنفيذ LogValuer — يُظهر "***" بدل القيمة الفعلية
func (s SecretString) LogValue() slog.Value {
    return slog.StringValue("***")
}

// الاستخدام:
type Config struct {
    Host     string
    Password SecretString
}

cfg := Config{Host: "db.example.com", Password: "super-secret-123"}
slog.Info("database config", "host", cfg.Host, "password", cfg.Password)
// Output: level=INFO msg="database config" host=db.example.com password=***
```

### نمط متقدم: `Sensitive[T]` عام (Generic)

```go
type Sensitive[T any] struct {
    value T
}

func NewSensitive[T any](v T) Sensitive[T] {
    return Sensitive[T]{value: v}
}

func (s Sensitive[T]) LogValue() slog.Value {
    return slog.StringValue("***")
}

func (s Sensitive[T]) Get() T {
    return s.value
}

// String() يمنع التسريب عبر fmt.Println أيضاً
func (s Sensitive[T]) String() string {
    return "***"
}
```

---

## 7. الاختبار

> **المصدر:** pkg.go.dev/testing/slogtest + Community Best Practices

### 7.1 اختبار بسيط مع `bytes.Buffer`

```go
func TestUserServiceLogging(t *testing.T) {
    var buf bytes.Buffer
    handler := slog.NewJSONHandler(&buf, nil)
    logger := slog.New(handler)

    svc := NewUserService(logger, mockDB)
    svc.GetUser(context.Background(), "user-42")

    output := buf.String()
    if !strings.Contains(output, "user-42") {
        t.Errorf("expected log to contain user_id, got: %s", output)
    }
    if !strings.Contains(output, "UserService") {
        t.Errorf("expected log to contain component name, got: %s", output)
    }
}
```

### 7.2 اختبار Handler مخصص مع `testing/slogtest`

الحزمة `testing/slogtest` من المكتبة القياسية تتحقق من أن الـ Handler يلتزم بالمواصفات:

```go
func TestCustomHandler(t *testing.T) {
    var buf bytes.Buffer
    handler := NewSensitiveDataHandler(
        slog.NewJSONHandler(&buf, nil),
        []string{"password", "token"},
    )

    // slogtest يتحقق من الامتثال لمواصفات slog.Handler
    err := slogtest.TestHandler(handler, func() []map[string]any {
        var ms []map[string]any
        for _, line := range bytes.Split(buf.Bytes(), []byte{'\n'}) {
            if len(line) == 0 {
                continue
            }
            var m map[string]any
            if err := json.Unmarshal(line, &m); err != nil {
                t.Fatal(err)
            }
            ms = append(ms, m)
        }
        return ms
    })
    if err != nil {
        t.Fatal(err)
    }
}
```

### 7.3 توجيه اللوقات لمخرجات الاختبار

```go
func TestWithTestOutput(t *testing.T) {
    // في Go الحديثة، يمكن توجيه اللوقات مباشرة لمخرجات الاختبار
    handler := slog.NewTextHandler(testWriter{t}, &slog.HandlerOptions{
        Level: slog.LevelDebug,
    })
    logger := slog.New(handler)
    // اللوقات تظهر فقط عند فشل الاختبار أو مع -v
}

type testWriter struct{ t *testing.T }

func (w testWriter) Write(p []byte) (n int, err error) {
    w.t.Helper()
    w.t.Log(strings.TrimRight(string(p), "\n"))
    return len(p), nil
}
```

---

## 8. الأنماط المضادة (Anti-Patterns)

> **المصادر:** Peter Bourgon (bourgon.org) — "Go, the Unwritten Parts"
> Dave Cheney (cheney.net) — "Let's talk about logging"
> 100go.co — "100 Go Mistakes"

### ❌ 8.1 اللوق داخل المكتبات/الحزم غير الرئيسية

```go
// ❌ سيء — المكتبة تقرر التسجيل بنفسها
func ParseConfig(path string) (*Config, error) {
    log.Printf("parsing config from %s", path)  // من يتحكم بهذا؟
    // ...
}

// ✅ جيد — المكتبة تُرجع الخطأ للمستدعي
func ParseConfig(path string) (*Config, error) {
    data, err := os.ReadFile(path)
    if err != nil {
        return nil, fmt.Errorf("reading config %s: %w", path, err)
    }
    // ...
}
```

### ❌ 8.2 تسجيل الخطأ ثم إرجاعه (Log and Return)

```go
// ❌ سيء — الخطأ يُسجَّل في كل طبقة = تكرار مزعج
func (s *Service) Process(ctx context.Context) error {
    result, err := s.repo.Fetch(ctx)
    if err != nil {
        s.logger.Error("failed to fetch", "error", err)  // تسجيل
        return err                                         // + إرجاع = تكرار!
    }
    // ...
}

// ✅ جيد — إما تسجيل أو إرجاع، لا كلاهما
func (s *Service) Process(ctx context.Context) error {
    result, err := s.repo.Fetch(ctx)
    if err != nil {
        return fmt.Errorf("processing: %w", err)  // wrap وأرجع فقط
    }
    // ...
}
// التسجيل يحدث في الطبقة العليا (main أو HTTP handler)
```

### ❌ 8.3 خلط اللوقات بالمقاييس (Metrics)

```go
// ❌ سيء — استخدام اللوقات لتتبع حالة النظام
logger.Info("request count", "total", requestCount)  // هذا metric وليس log!

// ✅ جيد — استخدم أدوات Metrics مخصصة
prometheus.NewCounter(prometheus.CounterOpts{Name: "request_total"}).Inc()
// واللوقات لأحداث محددة تحتاج تحقيقاً
logger.Error("payment failed", "order_id", orderID, "error", err)
```

### ❌ 8.4 اللوق المفرط (Chatty Logging)

> "إذا كان البرنامج يعمل بشكل طبيعي، فلا يجب أن يحتاج للتسجيل." — Dave Cheney

```go
// ❌ سيء — لوق في كل تكرار
for _, item := range items {
    logger.Debug("processing item", "id", item.ID)
    process(item)
    logger.Debug("item processed", "id", item.ID)
}

// ✅ جيد — لوق موجز وذو قيمة
logger.Info("batch processing started", "count", len(items))
for _, item := range items {
    if err := process(item); err != nil {
        logger.Error("item processing failed", "id", item.ID, "error", err)
    }
}
logger.Info("batch processing completed", "count", len(items))
```

### ❌ 8.5 تسجيل بيانات حساسة

```go
// ❌ خطير جداً
logger.Info("user authenticated", "password", req.Password, "token", token)

// ✅ آمن — استخدم LogValuer أو لا تسجّل أصلاً
logger.Info("user authenticated", "user_id", userID)
```

---

## 9. قائمة مراجعة الإنتاج

### الإعداد والتهيئة

| البند                        | الممارسة المطلوبة                                    |
|:----------------------------|:----------------------------------------------------|
| **التنسيق**                  | JSON للإنتاج، Text للتطوير المحلي                    |
| **المستوى**                  | `INFO` أو `WARN` في الإنتاج، `DEBUG` في التطوير      |
| **الوجهة**                   | `Stdout/Stderr` — البنية التحتية تتولى التجميع والدوران |
| **التهيئة**                  | مرة واحدة في `main.go` ثم حقن عبر DI                 |
| **المصدر**                   | `AddSource: true` اختياري (يضيف file:line)           |

### الأداء

| البند                        | الممارسة المطلوبة                                    |
|:----------------------------|:----------------------------------------------------|
| **Hot Paths**                | استخدم `LogAttrs` مع أنواع `slog.Attr` المحددة       |
| **بيانات متكررة**             | استخدم `logger.With()` للـ pre-formatting             |
| **عمليات مكلفة**              | تحقق بـ `Enabled()` قبل الحساب                       |
| **تجنب**                     | اللوق داخل الحلقات الضيقة                             |

### البنية

| البند                        | الممارسة المطلوبة                                    |
|:----------------------------|:----------------------------------------------------|
| **Structured Logging**       | دائماً key-value، لا strings مسطحة                   |
| **أسماء موحدة**              | وحّد أسماء الحقول: `user_id` وليس `userId`/`user-id`  |
| **Correlation IDs**          | دائماً `request_id` و `trace_id` لتتبع الطلبات        |
| **Context**                  | استخدم `InfoContext(ctx, ...)` لنقل بيانات الطلب       |
| **Scoped Loggers**           | `With("component", "PaymentService")` لكل مكون       |

### الأمان

| البند                        | الممارسة المطلوبة                                    |
|:----------------------------|:----------------------------------------------------|
| **PII**                      | لا تسجّل بيانات شخصية أبداً                           |
| **كلمات السر**                | استخدم `LogValuer` لإخفائها تلقائياً                   |
| **Tokens/API Keys**          | لا تظهر في اللوقات مطلقاً                             |
| **الأنواع المخصصة**           | أنشئ `Sensitive[T]` عام لكل البيانات الحساسة           |

---

## 10. المصادر والمراجع

### مصادر رسمية من فريق Go

| المصدر                                                    | الوصف                                    |
|:---------------------------------------------------------|:-----------------------------------------|
| [go.dev/blog/slog](https://go.dev/blog/slog)             | المدونة الرسمية — تقديم `slog`             |
| [pkg.go.dev/log/slog](https://pkg.go.dev/log/slog)       | توثيق الحزمة الرسمي مع أمثلة              |
| [Go Proposal #56345](https://go.dev/issue/56345)         | الاقتراح الرسمي (~800 تعليق)              |
| [Design Doc](https://go.googlesource.com/proposal/+/03441cb358c7b27a8443bca839e5d7a314677ea6/design/56345-structured-logging.md) | وثيقة التصميم التفصيلية        |
| [Handler Writing Guide](https://github.com/golang/example/blob/master/slog-handler-guide/README.md) | دليل كتابة Handler مخصص |
| [testing/slogtest](https://pkg.go.dev/testing/slogtest)  | حزمة اختبار امتثال الـ Handlers            |
| [go.dev/wiki/Resources-for-slog](https://go.dev/wiki/Resources-for-slog) | موارد المجتمع والـ Handlers |

### مصادر مجتمعية معترف بها

| المصدر                                                    | الوصف                                    |
|:---------------------------------------------------------|:-----------------------------------------|
| [Peter Bourgon — Logging v. Instrumentation](https://peter.bourgon.org/blog/2017/02/21/metrics-tracing-and-logging.html) | التفريق بين اللوقات والمقاييس والتتبع |
| [Dave Cheney — Let's talk about logging](https://dave.cheney.net/2015/11/05/lets-talk-about-logging) | فلسفة اللوق المبسط |
| [100 Go Mistakes — Error Handling](https://100go.co)      | أنماط خاطئة في التعامل مع الأخطاء واللوق  |
| [Uber Zap slog Handler](https://github.com/uber-go/zap/tree/master/exp/zapslog) | ربط Zap كـ backend لـ slog |

### تكامل مع مكتبات خارجية

| المكتبة     | Handler لـ slog                                          |
|:------------|:---------------------------------------------------------|
| **Zap**     | `go.uber.org/zap/exp/zapslog`                             |
| **logr**    | `github.com/go-logr/logr` (Pull #196)                    |
| **hclog**   | `github.com/evanphx/go-hclog-slog`                       |
| **zerolog** | متوافقة عبر adapters مجتمعية                               |

---

> **خلاصة:** ابدأ بـ `log/slog` — إنها الطريقة الرسمية والاصطلاحية (idiomatic) للتعامل مع اللوقات
> في Go. تجنب حزمة `log` القديمة لأي شيء أكبر من سكربت بسيط، واستفد من قابلية `slog`
> للتوسع والتبديل إذا تغيرت متطلبات الأداء.
