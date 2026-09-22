# الدليل المعماري 02: ترجمة أخطاء SQLSTATE وعزل نطاق الفشل

## 1. مبدأ عزل نطاق الفشل (Failure Domain Isolation)

في الأنظمة الموزعة والتطبيقات النظيفة، يُعد تسريب تفاصيل المحركات التخزينية (مثل أسماء الجداول، هياكل الأعمدة، واستثناءات السائق مثل `*pgconn.PgError`) إلى الطبقات العليا أو واجهات برمجة التطبيقات (APIs) خطراً أمنياً ومعمارياً جسيماً للأسباب التالية:

1. **كشف تفاصيل الأمان (Security Leakage)**: قد تحتوي رسائل أخطاء SQL على أسماء جداول، قيود، أو قيم مُدخلة حساسة.
2. **الارتباط الوثيق (Tight Coupling)**: إذا قامت طبقة الـ Service أو الـ Handler بفحص كود `23505` مباشرة، فإن استبدال قاعدة البيانات أو تعديل السائق مستقبلاً سيكسر منطق الأعمال بأكمله.
3. **تعدد مستهلكي الخطأ**: يجب أن يرى المطور في السجلات تفاصيل الخطأ الأصلية الكاملة مع التتبع المنطقي، بينما يرى المستخدم رسالة آمنة وموجزة، ويرى التطبيق كوداً برمجياً يمكن اتخاذ قرار بناءً عليه (`CodeConflict`, `CodeNotFound`).

```text
┌─────────────────────────────────────────────────────────────┐
│                    User / Client View                       │
│      "A resource with this identifier already exists"       │
└──────────────────────────────▲──────────────────────────────┘
                               │
┌──────────────────────────────┴──────────────────────────────┐
│                    Application / Logic                      │
│                platformerr.ErrorCode(err)                   │
│                     == CodeConflict                         │
└──────────────────────────────▲──────────────────────────────┘
                               │
┌──────────────────────────────┴──────────────────────────────┐
│                Operator / Structured Logs (slog)            │
│  "duplicate key value violates unique constraint            │
│   (constraint: users_email_key)" Op: repository.CreateUser  │
└──────────────────────────────▲──────────────────────────────┘
                               │
┌──────────────────────────────┴──────────────────────────────┐
│              PostgreSQL Protocol (SQLSTATE 23505)           │
│                   pgconn.PgError / pgx                      │
└─────────────────────────────────────────────────────────────┘
```

---

## 2. آلية عمل `postgres.TranslateError`

يتم استدعاء الدالة المركزية في نهاية كل عملية استعلام داخل طبقة الـ Repository:

```go
func (r *UserRepository) GetByID(ctx context.Context, id string) (*User, error) {
    const op = "UserRepository.GetByID"
    
    var user User
    err := r.db.QueryRow(ctx, "SELECT id, email, name FROM users WHERE id = $1", id).
        Scan(&user.ID, &user.Email, &user.Name)
    if err != nil {
        return nil, postgres.TranslateError(op, err)
    }
    return &user, nil
}
```

### خطوات الترجمة الداخلية

1. **التحقق من القيم الفارغة**: إذا كان الخطأ `nil`، تعيد الدالة `nil` فوراً دون أي تخصيص للذاكرة.
2. **منع التغليف المزدوج (No Double-Wrapping)**: إذا كان الخطأ قد تُرجم مسبقاً إلى `*platformerr.Error`، تعيده الدالة كما هو لضمان الحفاظ على نقطة العملية الأصلية (`Op`).
3. **فحص سياق التنفيذ (Context Inspection)**: تحويل `context.DeadlineExceeded` و `context.Canceled` إلى `CodeTimeout` (504).
4. **فحص سجلات غير موجودة**: تحويل `pgx.ErrNoRows` إلى `CodeNotFound` (404).
5. **فحص شفرات SQLSTATE عبر بروتوكول pgconn**:
   - `23505` (Unique Violation) $\rightarrow$ `CodeConflict` (409).
   - `23503` (Foreign Key Violation) $\rightarrow$ `CodeInvalid` (400).
   - `23502` (Not Null Violation) $\rightarrow$ `CodeInvalid` (400).
   - `23514` (Check Violation) $\rightarrow$ `CodeInvalid` (400).
   - `40001` (Serialization Failure) $\rightarrow$ `CodeUnavailable` (503 - قابل لإعادة المحاولة).
   - `40P01` (Deadlock Detected) $\rightarrow$ `CodeUnavailable` (503 - قابل لإعادة المحاولة).
   - `57P01/57P02/57P03` (Server Shutdown / Connection Refused) $\rightarrow$ `CodeUnavailable` (503).
   - `42501` (Insufficient Privilege) $\rightarrow$ `CodeForbidden` (403).
6. **فحص انقطاع الشبكة**: تحويل `net.Error` إلى `CodeUnavailable` (503).
7. **الحالة الافتراضية (Default Fallback)**: تحويل أي استثناء غير مصنف إلى `CodeInternal` (500) مع حجب النص الخام عن المستخدم الخارجي وحفظه في الحقل الباطني للخطأ `Err`.

---

## 3. الدوال المساعدة لفحص القيود والأخطاء العابرة

توفر الحزمة مجموعة من الدوال الاستقصائية السريعة:

```go
// 1. فحص ما إذا كان الخطأ عابراً ويمكن إعادة محاولته تلقائياً
if postgres.IsTransient(err) {
    // يمكن تكرار المعاملة
}

// 2. فحص قيد فرادة معين لتخصيص رسالة المستخدم
if postgres.IsConstraintViolation(err, "users_phone_idx") {
    return platformerr.Conflict(op, "رقم الهاتف مسجل مسبقاً بحساب آخر", err)
}

// 3. فحص عام لأخطاء القيود
if postgres.IsUniqueViolation(err) {
    // تعامل مع التعارض
}
```
