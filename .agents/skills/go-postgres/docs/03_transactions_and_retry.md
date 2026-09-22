# الدليل المعماري 03: إدارة المعاملات وإعادة المحاولة مع التراجع الأسي

## 1. واجهة `postgres.DBTX` الموحدة

أحد أهم الأنماط المعمارية في تطبيقات Go النظيفة هو تمكين دوال المستودعات (`Repositories`) من تنفيذ عملياتها سواء بشكل مستقل على مستوى مجمع الاتصالات الكامل (`*pgxpool.Pool`)، أو كجزء من معاملة ذرية موحدة (`pgx.Tx`) دون تكرار الشفرة.

تحقق واجهة `DBTX` هذا الغرض:

```go
type DBTX interface {
    Exec(ctx context.Context, sql string, arguments ...any) (pgconn.CommandTag, error)
    Query(ctx context.Context, sql string, args ...any) (pgx.Rows, error)
    QueryRow(ctx context.Context, sql string, args ...any) pgx.Row
}
```

عند بناء أي مستودع، يتم حقن `DBTX`:

```go
type OrderRepository struct {
    db postgres.DBTX
}

func NewOrderRepository(db postgres.DBTX) *OrderRepository {
    return &OrderRepository{db: db}
}
```

وبهذا يمكن تمرير إما `pool` أو `tx` لنفس الـ Repository بسلاسة مطلقة.

---

## 2. النمط الآمن لإدارة المعاملات (`postgres.ExecTx`)

تتطلب إدارة المعاملات يدوياً كتابة شفرات `Begin`, `Commit`, `Rollback` متكررة ومعرضة للخطأ (مثل نسيان التراجع عند حدوث `panic` أو عند حدوث خطأ غير متوقع مما يسبب تسرب الاتصالات وإبقاء أقفال الجداول محجوزة).

توفر الحزمة دالة `ExecTx` التي توفر الضمانات التالية:

1. **تراجع تلقائي آمن (Safe Defer Rollback)**: يُستدعى `tx.Rollback(ctx)` دائماً في `defer`. وفي حال نجاح `tx.Commit(ctx)`، يتجاهل السائق التراجع التلقائي دون أي خطأ.
2. **التعافي من الانهيارات (Panic Recovery)**: تلتقط الدالة أي `panic` يحدث داخل الدالة الممررة، وتنفذ `Rollback` فوراً لمنع تعليق قاعدة البيانات، ثم تعيد إطلاق الـ `panic` للحفاظ على مسار التتبع الأصلي (Stack Trace).
3. **ترجمة مركزية للخطأ**: تمرير جميع أخطاء المعاملة والـ Commit عبر `postgres.TranslateError`.

```go
err := postgres.ExecTx(ctx, pool, func(txCtx context.Context, tx pgx.Tx) error {
    repo := storage.NewOrderRepository(tx)
    
    if err := repo.DeductStock(txCtx, itemID, qty); err != nil {
        return err // سيتم تنفيذ Rollback تلقائياً
    }
    
    if err := repo.CreateOrder(txCtx, order); err != nil {
        return err // سيتم تنفيذ Rollback تلقائياً
    }
    
    return nil // سيتم تنفيذ Commit تلقائياً
})
```

---

## 3. خوارزمية التراجع الأسي مع التشويش الكامل (`Full Jitter Backoff`)

### لماذا نحتاج التشويش (Jitter)؟

عند حدوث تصادم بين معاملتين متنافستين في مستوى العزل `Serializable` أو `Repeatable Read`، ترجع قاعدة البيانات خطأ `40001` (Serialization Failure) أو `40P01` (Deadlock Detected). إذا قامت كافة خوادم التطبيق بإعادة المحاولة بعد فترة زمنية متطابقة (مثلاً بعد 100ms)، فستصطدم جميعها مجدداً في نفس اللحظة، مما يسبب ظاهرة **تجمهر الاتصالات (Thundering Herd Problem)** وانهيار الإنتاجية.

الحل المعتمد من مهندسي AWS و Google هو **Full Jitter**:

$$\text{sleep} = \text{random}(0, \, \min(\text{MaxInterval}, \, \text{InitialInterval} \times \text{Multiplier}^{\text{attempt}}))$$

### دالة `postgres.ExecTxWithRetry`

```go
cfg := postgres.DefaultRetryConfig() // 3 محاولات، 50ms بداية، 2s كحد أقصى، مضاعف 2.0

err := postgres.ExecTxWithRetry(ctx, pool, cfg, func(txCtx context.Context, tx pgx.Tx) error {
    // يجب أن تكون العملية هنا Idempotent
    return transferFunds(txCtx, tx, fromAccount, toAccount, amount)
})
```

### قواعد ذهبية لإعادة المحاولة

1. **العمليات المتكررة آمنة (Idempotency)**: يجب أن تكون الدالة التي يتم إعادة محاولتها آمنة التكرار، وألا تُحدث آثاراً جانبية غير قابلة للتراجع (مثل إرسال بريد إلكتروني خارجي داخل نفس الدالة).
2. **الأخطاء العابرة فقط**: لا تتم إعادة المحاولة لأخطاء التحقق (`CodeInvalid`) أو انتهاك الفرادة العادية (`CodeConflict`)، بل فقط للأخطاء المصنفة كـ `Transient` (`40001`, `40P01`, انقطاع الشبكة المؤقت).
3. **احترام سياق الإلغاء (`context.Context`)**: إذا قام المستخدم بإلغاء الطلب أو انتهت مهلة الـ HTTP Timeout، تتوقف حلقة الإعادة فوراً ولا تنتظر انقضاء فترات الـ Sleep.
