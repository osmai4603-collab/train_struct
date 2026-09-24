# 05. وسيط HTTP وتمرير الادعاءات في السياق والأداء العالي

## 1. تصميم وسيط HTTP النموذجي في Go

في مشاريع Go الكبيرة، يتولى وسيط المصادقة (`net/http` Middleware) تنفيذ سلسلة العمليات التالية بانضباط صارم:

```text
Incoming Request
      │
      ▼
1. قراءة الترويسة `Authorization: Bearer <token>` (رفض الطلب بـ 401 إن غابت).
      │
      ▼
2. استخراج الرمز وفحصه تشفيرياً عبر محرك التدقيق (Signer/Validator).
      │
      ▼
3. فحص الأخطاء القياسية (errors.Is مع ErrTokenExpired أو ErrSignatureInvalid).
      │
      ▼
4. فحص الإلغاء الفوري (Revocation Check) عبر القائمة السوداء للـ jti.
      │
      ▼
5. حزم الادعاءات في بنية موحدة ونقلها إلى سياق الطلب r.Context() بنوع غير مصدّر.
      │
      ▼
Next Handler
```

---

## 2. الأمان النوعي للسياق (Type-Safe Context Propagation)

### الكارثة الشائعة:
استخدام نص عادي كمفتاح للسياق:
```go
// خطأ فادح - يعرض النظام لتصادم المفاتيح بين الحزم
ctx = context.WithValue(r.Context(), "user", claims)
```

### النمط الهندسي المعتمد:
إنشاء نوع هيكل فارغ غير مصدّر مع توابع وصول صريحة خالية من التخصيصات العشوائية:
```go
package jwt

import "context"

type contextKey struct{}

var userClaimsKey = contextKey{}

// WithUserClaims يحقن الادعاءات في السياق بأمان
func WithUserClaims(ctx context.Context, claims *UserClaims) context.Context {
	return context.WithValue(ctx, userClaimsKey, claims)
}

// UserClaimsFromContext يستخرج الادعاءات بأمان نوعي كامل
func UserClaimsFromContext(ctx context.Context) (*UserClaims, bool) {
	claims, ok := ctx.Value(userClaimsKey).(*UserClaims)
	return claims, ok
}
```

---

## 3. تحسين الأداء ومكافحة استهلاك الذاكرة (Low-Allocation Engineering)

يمر ملايين الطلبات عبر وسيط المصادقة، ولضمان أقل ضغط على الـ Garbage Collector:
1. **تجنب التكرار:** استخدام مجمعات البنى (`sync.Pool`) عند الحاجة لمعالجة كائنات الـ Claims المتكررة.
2. **فصل الترويسة بدون تقطيع إضافي:** استخدام `strings.CutPrefix(authHeader, "Bearer ")` المتاحة في Go بدلاً من `strings.Split` لتقليل التخصيصات (0 allocations).
3. **تعتيم الرموز في السجلات:** تطبيق واجهة `slog.LogValuer` لتعتيم الرمز واستبداله بـ `[REDACTED]` لمنع تسريب مفاتيح الدخول إلى شاشات ومخازن السجلات (Datadog/Splunk).
