# 05. حجب الأسرار والبيانات الحساسة (Security & Sensitive Data)

> **المصادر المرجعية:** Go Documentation: `slog.LogValuer`, OWASP Top 10 Security Guidelines: Cryptographic Failures & Sensitive Data Exposure.

---

## 1. خطر تسرب الأسرار في السجلات

تُعد السجلات الهدف الأول للمهاجمين الداخليين والخارجيين عند اختراق بيئات المراقبة، حيث يؤدي التسجيل العشوائي لكائنات الطلبات والـ Structs إلى تسريب:
- كلمات المرور والرموز السرية (Passwords & Hashes).
- مفاتيح الـ API والـ Tokens (JWT, OAuth Access Tokens).
- بيانات البطاقات الائتمانية وأرقام الهوية الشخصية (PII).

في المشاريع الاحترافية، يجب ألا تعتمد الحماية على "انتباه المطور"، بل تكون **مضمنة في بنية الأنواع (Type-Level Enforcement)**.

---

## 2. واجهة `slog.LogValuer`

توفر حزمة `log/slog` واجهة قياسية تمكّن أي نوع بيانات في Go من التحكم في كيفية تحويله إلى قيمة سجل:

```go
type LogValuer interface {
    LogValue() Value
}
```

عند تمرير أي كائن يطبق `LogValuer` إلى `slog`، يقوم الـ Logger باستدعاء الدالة `LogValue()` بدلاً من طباعة الحقول الداخلية للكائن.

### مثال: نوع `SecretString`
```go
package security

import "log/slog"

type SecretString string

func (s SecretString) LogValue() slog.Value {
    return slog.StringValue("***")
}

func (s SecretString) String() string {
    return "***"
}

func (s SecretString) Expose() string {
    return string(s)
}
```

---

## 3. نمط الحاوية الآمنة العامة `Sensitive[T]`

لتفادي تكرار تعريف أنواع مخصصة لكل حقل سري، يمكن بناء نوع عام (Generic Type) يغلف أي قيمة ويحميها من التسرب عبر `slog` و `fmt.Println`:

```go
package security

import "log/slog"

// Sensitive يغلف أي قيمة لمنع تسربها في السجلات
type Sensitive[T any] struct {
    value T
}

// NewSensitive ينشئ حاوية آمنة جديدة
func NewSensitive[T any](v T) Sensitive[T] {
    return Sensitive[T]{value: v}
}

// LogValue يطبق واجهة slog.LogValuer
func (s Sensitive[T]) LogValue() slog.Value {
    return slog.StringValue("***")
}

// String يطبق واجهة fmt.Stringer لمنع التسرب عند استخدام fmt.Printf
func (s Sensitive[T]) String() string {
    return "***"
}

// Value يسترجع القيمة الحقيقية للاستخدام الداخلي
func (s Sensitive[T]) Value() T {
    return s.value
}
```

### استخدام `Sensitive[T]` في هياكل التكوين وبيانات الأعمال:
```go
type DatabaseConfig struct {
    Host     string
    Port     int
    Username string
    Password security.Sensitive[string]
}

cfg := DatabaseConfig{
    Host:     "localhost",
    Port:     5432,
    Username: "postgres",
    Password: security.NewSensitive("my-super-secret-password"),
}

// سيظهر في السجلات:
// host=localhost port=5432 username=postgres password=***
logger.Info("database configuration",
    slog.String("host", cfg.Host),
    slog.Any("password", cfg.Password),
)
```
