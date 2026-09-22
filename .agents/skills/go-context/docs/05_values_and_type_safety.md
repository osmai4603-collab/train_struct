# بيانات السياق (`context.WithValue`): الاستخدام المنضبط والحدود الصارمة

## 1. المصفوفة المعيارية: ما ينتمي وما لا ينتمي إلى `context.Value`

| المحتوى | الحكم | التعليل الهندسي والبديل المعياري |
|:---|:---:|:---|
| **Trace ID / Span Context (OpenTelemetry)** | ✅ مسموح | بيانات تتبع ملازمة للطلب وتنتقل عبر كافة الطبقات والشبكة. |
| **Correlation ID / Request ID** | ✅ مسموح | لربط السجلات (Logs) الخاصة بنفس دورة حياة الطلب. |
| **هوية المستخدم الموثق (Auth Claims / User ID)** | ✅ مسموح | يُحقن في Middleware الأمان عند الحدود ويُستهلك في طبقة الصلاحيات. |
| **معرّف المستأجر (Tenant ID)** | ✅ مسموح | لعزل البيانات في الأنظمة متعددة المستأجرين (Multi-Tenant). |
| **اتصال قاعدة البيانات (`*sql.DB`)** | ❌ ممنوع | Service Locator Anti-Pattern؛ يجب حقن التبعيات صراحة عبر البواني. |
| **كائن السجلات الرئيسي (`*slog.Logger`)** | ❌ ممنوع | يُحقن الـ Logger في بنية الخدمة؛ السياق يمرر فقط لتخصيص الخصائص. |
| **إعدادات التكوين (Configuration)** | ❌ ممنوع | يجب تمرير الإعدادات عند بناء الكائنات صراحة. |
| **المعاملات الاختيارية (Optional Arguments)** | ❌ ممنوع | كسر لأمان الأنواع؛ استخدم نمط الخيارات الوظيفية (Functional Options). |

---

## 2. الحماية من تصادم المفاتيح: نمط الأنواع غير المصدّرة (Unexported Types)

استخدام النصوص العادية مثل `"user_id"` كمفاتيح يؤدي إلى تصادم الحزم (Key Collision) ومسح البيانات دون أي تحذير من المترجم:

```go
// ❌ كارثة: مفتاح عام قابل للتصادم
ctx = context.WithValue(ctx, "user_id", "usr_123")
```

### النمط المعياري المعتمد عالمياً:

```go
package platformctx

import "context"

// 1. تعريف نوع خاص غير مصدّر بحجم صفر بايت
type userIDKey struct{}

// 2. دالة حقن آمنة نوعياً
func WithUserID(ctx context.Context, userID string) context.Context {
    return context.WithValue(ctx, userIDKey{}, userID)
}

// 3. دالة استخراج آمنة ومحمية من panic
func UserIDFromContext(ctx context.Context) (string, bool) {
    id, ok := ctx.Value(userIDKey{}).(string)
    return id, ok
}
```

---

## 3. نمط التجميع لتفادي تكلفة شجرة البحث $O(N)$ (Bundling Pattern)

يتم تمثيل `valueCtx` في محرك Go كقائمة مترابطة أحادية (Single Linked List). كل استدعاء لدالة `WithValue` ينشئ عقدة جديدة تغلف العقدة السابقة:

```text
ctx -> Value("A") -> Value("B") -> Value("C") -> Background()
```

إذا قمت بحقن 10 سمات مختلفة كل واحدة في استدعاء مستقل، فإن البحث عن السمة الأولى سيتطلب قطع مسار خطي $O(N)$، فضلاً عن حجز 10 كائنات إضافية على الـ Heap لكل طلب وارد!

### الحل المعياري في `internal/platform/context`:
تجميع كل بيانات الطلب في هيكل بيانات موحد وحقنه مرة واحدة بعقدة سياق واحدة:

```go
package platformctx

import "context"

type RequestMetadata struct {
    RequestID     string
    CorrelationID string
    TenantID      string
    UserID        string
    ClientIP      string
}

type metadataKey struct{}

// حقن كائن التجميع مرة واحدة
func WithRequestMetadata(ctx context.Context, meta *RequestMetadata) context.Context {
    return context.WithValue(ctx, metadataKey{}, meta)
}

// استخراج كائن التجميع
func MetadataFromContext(ctx context.Context) (*RequestMetadata, bool) {
    meta, ok := ctx.Value(metadataKey{}).(*RequestMetadata)
    return meta, ok
}
```
