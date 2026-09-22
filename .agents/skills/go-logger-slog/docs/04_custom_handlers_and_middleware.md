# 04. تصميم الـ Middleware والـ Custom Handlers

> **المصادر المرجعية:** Go slog Handler Writing Guide, pkg.go.dev/log/slog Handler Interface.

---

## 1. نمط HTTP Logging Middleware

يُعد وسيط HTTP Logging أحد أهم نقاط التكامل في خدمات الويب وخوادم REST/gRPC. وظيفته الأساسية هي:
1. استخراج أو توليد معرف فريد للطلب (`request_id` / `trace_id`).
2. ربط هذا المعرف بـ `*slog.Logger` مخصص للطلب وحقنه داخل `r.Context()`.
3. قياس وقت معالجة الطلب وتسجيل حالة الاستجابة ورمز الحالة (HTTP Status Code).

```go
type contextKey struct{}

var loggerContextKey = contextKey{}

// WithLoggerContext يضيف السجل إلى السياق
func WithLoggerContext(ctx context.Context, logger *slog.Logger) context.Context {
    return context.WithValue(ctx, loggerContextKey, logger)
}

// FromContext يستخرج السجل من السياق بأمان
func FromContext(ctx context.Context) *slog.Logger {
    if logger, ok := ctx.Value(loggerContextKey).(*slog.Logger); ok {
        return logger
    }
    return slog.Default()
}
```

---

## 2. معمارية الـ Custom Handler

في بعض الحالات، لا تكفي المعالجات القياسية (`JSONHandler` و `TextHandler`)، مثل:
- تنقيح وحجب البيانات الحساسة عبر قائمة مفاتيح ديناميكية (Dynamic Key Redaction).
- إرسال السجلات إلى وجهات متعددة بالتوازي (Multi-writer أو Fan-out).
- تصفية السجلات وفق معايير أعمال محددة.

### واجهة `slog.Handler` — الدوال الأربع الإلزامية

لإنشاء معالج سليم متوافق بنسبة 100% مع معايير Go، يجب تطبيق الدوال الأربع التالية دون استثناء:

```go
type Handler interface {
    // 1. فحص هل المستوى مفعّل في هذا السياق
    Enabled(context.Context, Level) bool

    // 2. معالجة وتنسيق وكتابة السجل
    Handle(context.Context, Record) error

    // 3. اشتقاق معالج جديد مع سمات ثابتة إضافية
    WithAttrs(attrs []Attr) Handler

    // 4. اشتقاق معالج جديد مع تجميع السمات تحت مسمى مجموعة (Group)
    WithGroup(name string) Handler
}
```

---

## 3. نمط تغليف المعالج (Handler Middleware Pattern)

أفضل وأسهل طريقة لبناء معالج مخصص هي تغليف معالج داخلي (`inner Handler`) وتعديل السجلات قبل تمريرها له:

```go
type RedactingHandler struct {
    inner      slog.Handler
    maskKeys   map[string]bool
}

func (h *RedactingHandler) Enabled(ctx context.Context, level slog.Level) bool {
    return h.inner.Enabled(ctx, level)
}

func (h *RedactingHandler) Handle(ctx context.Context, r slog.Record) error {
    newRecord := slog.NewRecord(r.Time, r.Level, r.Message, r.PC)
    r.Attrs(func(a slog.Attr) bool {
        if h.maskKeys[a.Key] {
            a.Value = slog.StringValue("***")
        }
        newRecord.AddAttrs(a)
        return true
    })
    return h.inner.Handle(ctx, newRecord)
}

func (h *RedactingHandler) WithAttrs(attrs []slog.Attr) slog.Handler {
    return &RedactingHandler{
        inner:    h.inner.WithAttrs(attrs),
        maskKeys: h.maskKeys,
    }
}

func (h *RedactingHandler) WithGroup(name string) slog.Handler {
    return &RedactingHandler{
        inner:    h.inner.WithGroup(name),
        maskKeys: h.maskKeys,
    }
}
```

> **تنبيه:** عند بناء أي Handler مخصص، يجب التحقق من صحة تنفيذه والتزامه بكافة قواعد تسطيح المجموعات والأوقات باستخدام اختبار `testing/slogtest` المدمج في Go.
