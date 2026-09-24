# 08. التكامل والاعتماديات مع مهارات المنظومة (`Platform Skills Integration`)

تتكامل البنية التحتية للجلسات (`internal/infrastructure/session`) بصورة عضوية ومباشرة مع المهارات الست الأساسية للمشروع لضمان اتساق المعمارية وتفادي تشتت الشيفرات البرمجية.

---

## 1. التكامل مع `go-context` (`internal/platform/context`)

### 1.1 الهدف

عند نجاح قراءة وفحص الجلسة، يجب ألا تُترك هوية المستخدم معزولة، بل يتم إثراء كائن `platformctx.RequestMetadata` الموحد للمشروع تلقائياً.

### 1.2 كود التكامل

```go
package session

import (
 "context"

 platformctx "train/internal/platform/context"
)

// InjectSessionIntoContext يحقن هوية المستخدم في RequestMetadata والسياق العام
func InjectSessionIntoContext(ctx context.Context, data *SessionData) context.Context {
 // 1. تحديث RequestMetadata المركزي بهوية المستخدم
 ctx = platformctx.WithUserID(ctx, data.UserID)

 // 2. حقن كائن الجلسة الكامل للوصول للأدوار ورمز الـ CSRF
 return context.WithValue(ctx, sessionContextKey, data)
}
```

---

## 2. التكامل مع `go-errors` (`internal/platform/errors`)

### 2.1 الهدف

ترجمة حالات فشل الجلسات وانتهاء الصلاحيات إلى أخطاء المنصة المهيكلة برمز تشغيلي (Op) ورمز خطأ قياسي (Machine-readable Code) وخريطة لحالة HTTP المناسبة.

### 2.2 مصفوفة الترجمة

| حالة الجلسة | رمز خطأ المنصة (`Code`) | حالة HTTP | دالة المنشئ في `platformerrors` |
| :--- | :--- | :--- | :--- |
| الجلسة غير موجودة أو منتهية | `UNAUTHORIZED` | 401 | `platformerrors.Unauthorized(op, msg, err)` |
| رمز CSRF مفقود أو غير مطابق | `FORBIDDEN` | 403 | `platformerrors.Forbidden(op, msg, err)` |
| تنسيق الكوكي أو المعرف تالف | `INVALID_ARGUMENT` | 400 | `platformerrors.Invalid(op, msg, err)` |
| فشل الاتصال بـ Redis أو Postgres | `INTERNAL` | 500 | `platformerrors.Internal(op, msg, err)` |

---

## 3. التكامل مع `go-logger-slog` (`internal/platform/logger`)

### 3.1 الهدف

1. حظر طباعة معرفات الجلسات الخام (Raw Tokens) في السجلات واستخدام `platformlogger.NewSensitive(token)` لتعتيمها تلقائياً.
2. تدوين أحداث المصادقة وتسجيل الخروج وتجديد الجلسات مع ربطها التلقائي بـ `req_id` و `corr_id` و `user_id`.

### 3.2 كود التكامل

```go
package session

import (
 "context"

 platformlogger "train/internal/platform/logger"
)

func LogSessionEvent(ctx context.Context, op string, event string, data *SessionData, rawToken string) {
 log := platformlogger.FromContext(ctx)

 // تعتيم الرمز الخام
 masked := platformlogger.NewSensitive(rawToken)

 log.Info("session event occurred",
  "op", op,
  "event", event,
  "user_id", data.UserID,
  "token", masked, // يظهر تلقائياً كـ *** في السجلات
 )
}
```

---

## 4. التكامل مع `go-postgres` (`internal/infrastructure/postgres`)

### 4.1 الهدف

تنفيذ مستودع الجلسات المتين باستخدام `postgres.DBTX` وعزل أخطاء قاعدة البيانات عبر `postgres.TranslateError`.

### 4.2 جدول جلسات PostgreSQL

```sql
CREATE TABLE IF NOT EXISTS auth_sessions (
    token_hash VARCHAR(64) PRIMARY KEY,
    user_id VARCHAR(64) NOT NULL,
    roles TEXT[] NOT NULL DEFAULT '{}',
    csrf_token VARCHAR(64) NOT NULL,
    data JSONB NOT NULL DEFAULT '{}',
    created_at TIMESTAMPTZ NOT NULL,
    last_active_at TIMESTAMPTZ NOT NULL,
    absolute_exp TIMESTAMPTZ NOT NULL,
    expires_at TIMESTAMPTZ NOT NULL
);

CREATE INDEX IF NOT EXISTS idx_auth_sessions_user_id ON auth_sessions(user_id);
CREATE INDEX IF NOT EXISTS idx_auth_sessions_expires_at ON auth_sessions(expires_at);
```

---

## 5. التكامل مع `go-server-lifecycle` (`internal/infrastructure/server`)

### 5.1 الهدف

1. **الفحص الصحي والجاهزية:** التحقق من جاهزية اتصال عنقود Redis أو قاعدة بيانات PostgreSQL قبل إعلان نجاح الفحص الصحي `/readyz`.
2. **الإيقاف المتناسق (Graceful Shutdown):** إنهاء عمال كنس الجلسات المنتهية (Background Sweeper Workers) بنعومة عند استقبال إشارات النظام (SIGTERM/SIGINT).

---

## 6. التكامل مع `go-configuration-json` (`internal/platform/config`)

### 6.1 بنية تكوين الجلسات في إعدادات المشروع

```go
type SessionConfig struct {
 CookieName       string        `json:"cookie_name" env:"SESSION_COOKIE_NAME"`
 IdleTimeout      time.Duration `json:"idle_timeout" env:"SESSION_IDLE_TIMEOUT"`
 AbsoluteTimeout  time.Duration `json:"absolute_timeout" env:"SESSION_ABSOLUTE_TIMEOUT"`
 CookieSecure     bool          `json:"cookie_secure" env:"SESSION_COOKIE_SECURE"`
 CookieDomain     string        `json:"cookie_domain" env:"SESSION_COOKIE_DOMAIN"`
 StoreBackend     string        `json:"store_backend" env:"SESSION_STORE_BACKEND"` // redis | postgres
 ThrottleDuration time.Duration `json:"throttle_duration" env:"SESSION_THROTTLE_DURATION"`
}
```

تُفحص هذه الإعدادات وتُعتمد أثناء مرحلة الـ Bootstrap للمشروع لضمان الإخفاق المبكر (Fail-Fast) عند وجود أي خطأ في الإعدادات.
