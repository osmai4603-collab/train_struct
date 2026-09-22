# 02. تصنيف الأخطاء واستراتيجيات الاستخدام (Error Taxonomy & Strategies)

> **الهدف:** توضيح الاستراتيجيات المعيارية الأربع للتعامل مع الأخطاء في Go، ومعرفة متى وكيف تُطبق كل استراتيجية بدقة.

---

## مصفوفة استراتيجيات الأخطاء

| الاستراتيجية | المفهوم | متى تُستخدم؟ | مثال في Go |
| :--- | :--- | :--- | :--- |
| **Sentinel Errors** | قيم عامة ثابتة يتم فحصها بـ `errors.Is` | لحالات متوقعة وثابتة بدون تفاصيل ديناميكية | `io.EOF`, `sql.ErrNoRows` |
| **Custom Type Errors** | هياكل مخصصة تحمل حقول بيانات إضافية | عند الحاجة لبيانات مهيكلة (مثل أخطاء التحقق) | `*platformerr.ValidationError` |
| **Unified Platform Error** | هيكل موحد يجمع الكود والرسالة والمسار | العمود الفقري لكافة طبقات التطبيق والخدمات | `*platformerr.Error` |
| **Opaque / Behavioral** | فحص سلوك الخطأ عبر واجهة بدلاً من نوعه | لفك الاقتران التام بين الحزم المستقلة | `IsRetryable(err)` |

---

## 1. أخطاء الحارس (Sentinel Errors)

أخطاء الحارس هي متغيرات مسبقة التعريف تعبر عن حالة معروفة:

```go
package domain

import "errors"

var (
    ErrAccountSuspended = errors.New("account is suspended")
    ErrInsufficientFunds = errors.New("insufficient funds for withdrawal")
)
```

### قواعد استخدام أخطاء الحارس:
1. **تسمية متسقة:** تبدأ دائماً بـ `Err` (مثل `ErrNotFound`).
2. **فحص آمن:** تُفحص دائماً عبر `errors.Is(err, ErrNotFound)` ويُمنع استخدام `==`.
3. **تجنب الإفراط:** الإفراط في أخطاء الحارس ينشئ اقتراناً قوياً (Tight Coupling) يجبر المستدعي على استيراد الحزمة.

---

## 2. الهياكل المهيكلة المخصصة (Custom Types)

تُستخدم عندما يحتاج المستدعي الوصول إلى تفاصيل إضافية لا يمكن التعبير عنها بنص بسيط، مثل `ValidationError`:

```go
valErr := platformerr.NewValidationError("UserRegistration")
valErr.AddViolation("email", "invalid email address format")
valErr.AddViolation("age", "must be 18 or older")

// استخراج النوع بأمان عبر errors.As
var targetValErr *platformerr.ValidationError
if errors.As(err, &targetValErr) {
    for _, violation := range targetValErr.Violations {
        fmt.Printf("field: %s -> %s\n", violation.Field, violation.Description)
    }
}
```

---

## 3. فحص السلوك (Behavioral / Opaque Errors)

يعتبر هذا النمط، الذي دعا إليه **Dave Cheney**، الأسلوب الأمثل لعزل التبعيات:

```go
// تعريف واجهة سلوكية داخلية
type retryable interface {
    Retryable() bool
}

// دالة فحص السلوك دون استيراد أنواع الحزمة المصدرة للخطأ
func IsRetryable(err error) bool {
    var r retryable
    if errors.As(err, &r) {
        return r.Retryable()
    }
    return false
}
```
