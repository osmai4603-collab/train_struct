# 08. مصفوفة التكامل والاعتماديات مع مهارات المنظومة (Platform Skills Integration)

تحدد هذه الوثيقة عقود التكامل المعماري المباشر بين مهارة **`go-jwt`** والمهارات الست الأساسية المعتمدة في مشروعنا:

```text
┌──────────────────────────────────────────────────────────────────────────────────┐
│                   شبكة تكامل مهارة go-jwt مع مهارات المشروع                     │
├─────────────────────────┬────────────────────────────────────────────────────────┤
│ المهارة المرتبطة        │ دور التكامل المعماري والعقد الهندسي                     │
├─────────────────────────┼────────────────────────────────────────────────────────┤
│ 1. go-context           │ إثراء RequestMetadata بـ UserID و TenantID في السياق   │
│ 2. go-errors            │ ترجمة أخطاء الرمز إلى platformerrors (401/403/500)      │
│ 3. go-logger-slog       │ تعتيم الرموز عبر Sensitive[T] والتدوين بالسياق          │
│ 4. go-postgres          │ تخزين جلسات رموز التحديث RTR عبر pgxpool و DBTX        │
│ 5. go-server-lifecycle  │ الجلب الاستباقي لـ JWKS عند الإقلاع وحماية المسارات     │
│ 6. go-configuration-json│ تحميل JWTConfig مع تعتيم الأسرار (***) والتحقق الفوري   │
└─────────────────────────┴────────────────────────────────────────────────────────┘
```

---

## 1. التكامل مع `go-context` (`internal/platform/context`)

### الهدف

عند نجاح فحص الـ Access Token، يجب ألا تُترك ادعاءات المستخدم معزولة، بل يتم دمجها مباشرة في كائن `platformctx.RequestMetadata` الموحد للمشروع.

### كود التكامل

```go
import (
 platformctx "train/internal/platform/context"
)

// حقن بيانات المستخدم والـ Tenant المستخرجة من الـ JWT في سياق المنصة
func EnrichContextWithClaims(ctx context.Context, claims *AppClaims) context.Context {
 // 1. تحديث RequestMetadata المركزي
 ctx = platformctx.WithUserID(ctx, claims.UserID)
 if claims.TenantID != "" {
  ctx = platformctx.WithTenantID(ctx, claims.TenantID)
 }

 // 2. حقن كائن الـ Claims الكامل للوصول إلى الأدوار (Roles)
 return WithUserClaims(ctx, claims)
}
```

---

## 2. التكامل مع `go-errors` (`internal/platform/errors`)

### 1. الهدف

عزل أخطاء مكتبات التشفير الخارجية (`golang-jwt` أو `jwx`) وترجمتها إلى أخطاء المنصة المهيكلة برمز تشغيلي (Op) ورمز خطأ قياسي (Machine-readable Code) وخريطة لحالة الـ HTTP.

### 2. مصفوفة الترجمة

| خطأ مكتبة JWT | رمز خطأ المنصة (`Code`) | حالة HTTP | دالة المنشئ في `platformerrors` |
| :--- | :--- | :--- | :--- |
| `jwt.ErrTokenExpired` | `CodeUnauthorized` | 401 | `platformerrors.Unauthorized("jwt.Validate", "token has expired", err)` |
| `jwt.ErrTokenSignatureInvalid` | `CodeUnauthorized` | 401 | `platformerrors.Unauthorized("jwt.Validate", "invalid signature", err)` |
| `jwt.ErrTokenMalformed` | `CodeInvalid` | 400 | `platformerrors.Invalid("jwt.Parse", "malformed token format", err)` |
| `ErrTokenReuseDetected` | `CodeUnauthorized` | 401 | `platformerrors.Unauthorized("jwt.Rotate", "security breach: token reuse detected", err)` |
| `ErrMissingRole` / `Forbidden` | `CodeForbidden` | 403 | `platformerrors.Forbidden("jwt.Authorize", "insufficient role permissions", nil)` |
| `ErrJWKSUnavailable` | `CodeBadGateway` | 502 | `platformerrors.E("jwt.FetchJWKS", platformerrors.CodeBadGateway, "failed to reach auth provider", err)` |

---

## 3. التكامل مع `go-logger-slog` (`internal/platform/logger`)

### 1 الهدف

1. استخدام نوع `platformlogger.SecretString` أو `platformlogger.Sensitive[T]` لتغليف المفاتيح الخاصة ورموز التحديث لمنع تسريبها في السجلات.
2. تدوين أحداث المصادقة عبر `platformlogger.FromContext(ctx)` لربط السجل تلقائياً بـ `req_id` و `corr_id` و `user_id`.

### 2 كود التكامل

```go
import (
 platformlogger "train/internal/platform/logger"
)

func LogAuthEvent(ctx context.Context, op string, tokenStr string, claims *AppClaims) {
 log := platformlogger.FromContext(ctx)

 // تغليف الرمز لمنع ظهوره صراحة في السجلات
 maskedToken := platformlogger.NewSensitive(tokenStr)

 log.Info("authentication successful",
  "op", op,
  "user_id", claims.UserID,
  "roles", claims.Roles,
  "token", maskedToken, // سيظهر تلقائياً كـ ***
 )
}
```

---

## 4. التكامل مع `go-postgres` (`internal/infrastructure/postgres`)

### - الهدف

تنفيذ مستودع جلسات رموز التحديث (`SessionRepository`) باستخدام `postgres.DBTX` مع ترجمة أخطاء قاعدة البيانات عبر `postgres.TranslateError` وتنفيذ المعاملات الذرية لإلغاء وتدوير الرموز عبر `postgres.ExecTxWithRetry`.

### مخطط قاعدة البيانات لجلسات الـ Refresh Tokens

```sql
CREATE TABLE auth_refresh_tokens (
    id VARCHAR(64) PRIMARY KEY,
    family_id VARCHAR(64) NOT NULL,
    user_id VARCHAR(64) NOT NULL,
    is_used BOOLEAN NOT NULL DEFAULT FALSE,
    expires_at TIMESTAMPTZ NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_refresh_tokens_family ON auth_refresh_tokens(family_id);
CREATE INDEX idx_refresh_tokens_user ON auth_refresh_tokens(user_id);
```

---

## 5. التكامل مع `go-server-lifecycle` (`internal/infrastructure/server`)

- الهدف

1. **الجلب الاستباقي (Fail-Fast Pre-fetching):** تهيئة كاش الـ JWKS أثناء مرحلة الـ Bootstrap وقبل إعلان الجاهزية `/readyz` لضمان أن الخادم لا يستقبل طلبات وهو عاجز عن فحص التواقيع.
2. **عزل المسارات (Security Isolation):** تطبيق وسيط المصادقة `jwt.Middleware` حصرياً على خادم الـ Public API (`:8070`)، مع استثناء مسارات الفحص الصحي `/livez` و `/readyz` ومسارات الإدارة المعزولة في Management Server (`:8066`).

---

## 6. التكامل مع `go-configuration-json` (`internal/platform/config`)

### بنية تكوين الـ JWT في إعدادات المشروع

```go
type JWTConfig struct {
 Issuer          string                      `json:"issuer" env:"JWT_ISSUER"`
 Audience        string                      `json:"audience" env:"JWT_AUDIENCE"`
 JWKSURL         string                      `json:"jwks_url" env:"JWT_JWKS_URL"`
 AccessTokenTTL  time.Duration               `json:"access_token_ttl" env:"JWT_ACCESS_TOKEN_TTL"`
 RefreshTokenTTL time.Duration               `json:"refresh_token_ttl" env:"JWT_REFRESH_TOKEN_TTL"`
 AllowedMethods  []string                    `json:"allowed_methods" env:"JWT_ALLOWED_METHODS"`
 PrivateKeyPEM   platformlogger.SecretString `json:"private_key_pem" env:"JWT_PRIVATE_KEY_PEM"`
}
```

يتم تعتيم `PrivateKeyPEM` تلقائياً عبر `SecretString` حتى لا تظهر في سجلات إقلاع الخادم (Configuration Dump).
