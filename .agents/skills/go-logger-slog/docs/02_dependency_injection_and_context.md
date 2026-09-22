# 02. حقن التبعيات الصريح ونقل السياق (Dependency Injection & Context)

> **المصادر المرجعية:** Peter Bourgon (*Go: The Unwritten Parts*), Dave Cheney (*Let's talk about logging*), Go Standard Library Conventions.

---

## 1. لماذا الحقن الصريح (Explicit Dependency Injection)؟

في أنظمة Go الإنتاجية، يعتبر استخدام السجل العام (Global Logger) عبر `slog.Default()` نمطاً مضاداً (Anti-Pattern) داخل طبقات الخدمات وقواعد البيانات ومكونات الأعمال.

### مقارنة بين السجل العام والحقن الصريح

| المعيار | السجل العام (`slog.Default()`) | الحقن الصريح (`*slog.Logger`) |
| :--- | :--- | :--- |
| **الشفافية** | يخفي التبعية داخل منطق العمل | يوضح احتياج المكون للسجل في واجهة بنائه |
| **قابلية الاختبار** | عسير؛ يتطلب تعديل الحالة العامة المتزامنة | سهل جداً؛ يمكن تمرير سجل موجه لـ `bytes.Buffer` أو `testing.T` |
| **تحديد النطاق** | سجل عام مسطح بدون سياق المكون | سجل مخصص للمكون عبر `.With(slog.String("component", "..."))` |
| **سلامة التزامن** | قد يسبب سباق بيانات (Race Condition) عند تغييره أثناء التشغيل | آمن وثابت (Immutable) بمجرد الإنشاء |

---

## 2. النمط المعياري لحقن السجل في البنى والخدمات

يتم تمرير المؤشر `*slog.Logger` كمعلمة صريحة في دالة بناء المكون (`Constructor`)، مع اشتقاق نسخة محددة النطاق تحمل سمة المكون:

```go
package order

import (
    "context"
    "database/sql"
    "log/slog"
)

type Service struct {
    logger *slog.Logger
    db     *sql.DB
}

func NewService(logger *slog.Logger, db *sql.DB) *Service {
    return &Service{
        // اشتقاق logger فرعي يحمل اسم المكون
        logger: logger.With(slog.String("component", "OrderService")),
        db:     db,
    }
}

func (s *Service) ProcessOrder(ctx context.Context, orderID string) error {
    s.logger.InfoContext(ctx, "processing order",
        slog.String("order_id", orderID),
    )
    // ...
    return nil
}
```

---

## 3. لماذا استخدام `*slog.Logger` المباشر بدلاً من `interface` مخصصة؟

في الماضي، كان المطورون يعرّفون واجهات مخصصة للوقرز (مثل `type Logger interface { Info(...) }`). مع ظهور `log/slog`، أصبح التوجه المعتمد في Go هو استخدام `*slog.Logger` مباشرة داخل الخدمات للأسباب التالية:

1. **دعم كامل لكافة الميزات المتقدمة:**
   تعريف `interface` مخصصة يجبرك على التخلي عن دوال جوهرية مثل `With()`, `WithGroup()`, `LogAttrs()`, و `Enabled()`, أو إعادة كتابتها بصعوبة.
2. **المرونة عبر المعالجات (Handlers):**
   التجريد المطلوب تم نقله بالفعل إلى طبقة الـ `slog.Handler`. إذا رغبت بتغيير السلوك أو المخرجات أو التوجيه، يمكنك تبديل المعالج دون المساس بـ `*slog.Logger`.
3. **أداء أعلى وتخصيصات أقل:**
   تجاوز طبقات التجريد الزائدة يوفر استدعاءات إضافية على الـ dynamic dispatch.

> **استثناء لمطوري المكتبات العامة (Public Libraries):** إذا كنت تطور مكتبة مفتوحة المصدر مستقلة، لا تفرض على مستهلكي المكتبة طباعة سجلات أصلاً؛ بل أرجع الأخطاء للمستدعي. وإذا كانت السجلات ضرورية جداً، اقبل واجهة بسيطة أو دالة callback اختيارية.

---

## 4. نقل السياق (Context Propagation)

يجب دائماً تفضيل دوال `Context` مثل `InfoContext`, `ErrorContext`, و `LogAttrs` على الدوال التقليدية `Info`, `Error`.

### لماذا يُعد `ctx` جوهرياً؟

1. **معرفات التتبع (Correlation & Trace IDs):**
   عند مرور الطلب عبر عدة طبقات، يحمل الـ `ctx` معرف الطلب (`trace_id`, `span_id`, `request_id`). يقوم المعالج أو الـ Middleware باستخراجه آلياً.
2. **الإلغاء والمهلات (Cancellation & Deadlines):**
   في حال انتهاء مهلة الطلب (`context.DeadlineExceeded`) أو إلغائه من قبل العميل، يتم ربط حالة السجل بذلك السياق بدقة.
