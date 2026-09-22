# التكامل مع المنظومة والمهام الخلفية (Ecosystem & Background Tasks)

## 1. التعامل مع خوادم وعملاء HTTP

### أ. في خوادم HTTP:
السياق مستمد مباشرة من كائن الطلب `r.Context()`. هذا السياق مربوط مباشرة بمقبس الاتصال (Socket) مع العميل؛ فإذا أغلق المتصفح أو قطع الاتصال، يتم إلغاء السياق فوراً:

```go
func OrderHandler(w http.ResponseWriter, r *http.Request) {
    ctx := r.Context()

    // تقييد دورة المعالجة بمهلة مناسبة للخدمة
    ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
    defer cancel()

    result, err := orderService.Create(ctx, r.Body)
    if err != nil {
        handleHTTPError(w, err)
        return
    }
    renderJSON(w, result)
}
```

### ب. في عملاء HTTP:
تجنب الدوال غير المقيدة بسياق مثل `http.Get` أو `http.Post`، واستخدم دائماً `http.NewRequestWithContext`:

```go
func CallDownstream(ctx context.Context, url string) (*http.Response, error) {
    req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
    if err != nil {
        return nil, err
    }
    return defaultClient.Do(req)
}
```

---

## 2. التكامل مع قواعد البيانات (`database/sql`)

استخدم دوال `*Context` حصراً لضمان أن استعلامات قواعد البيانات تُقطع فوراً في محرك الـ DB إذا تم إلغاء طلب العميل، مما يوفر موارد قاعدة البيانات:

```go
func GetUser(ctx context.Context, db *sql.DB, id string) (*User, error) {
    var u User
    err := db.QueryRowContext(ctx, "SELECT id, name FROM users WHERE id = $1", id).
        Scan(&u.ID, &u.Name)
    return &u, err
}
```

---

## 3. المهام الخلفية المنفصلة: دقة استخدام `context.WithoutCancel` (Go 1.21+)

كثيراً ما يحتاج المعالج (Handler) لإطلاق مهمة في الخلفية (كإرسال بريد، أو تسجيل عملية في سجل التدقيق Audit Log) بعد إرسال الرد للعميل.

```go
// ❌ خطأ شائع قاتل: استخدام r.Context() في مهمة خلفية
go func() {
    // بمجرد خروج OrderHandler، يُلغى r.Context() ويفشل إرسال البريد!
    mailer.SendReceipt(r.Context(), order) 
}()
```

### الحل المعياري الحديث (Go 1.21+):
استخدام `context.WithoutCancel` لفصل إشارة الإلغاء، **مع الاحتفاظ التام بكافة قيم التتبع والـ Trace IDs والـ Logger**:

```go
// ✅ صحيح: فصل الإلغاء مع بقاء بيانات التتبع وتحديد مهلة جديدة مستقلة
func CompleteOrder(w http.ResponseWriter, r *http.Request) {
    order := processOrder(r)

    // 1. فصل إشارة الإلغاء عن سياق الطلب
    detachedCtx := context.WithoutCancel(r.Context())

    // 2. إطلاق المهمة مع مهلة مستقلة تمنع التعليق
    go func() {
        bgCtx, cancel := context.WithTimeout(detachedCtx, 15*time.Second)
        defer cancel()

        auditService.Record(bgCtx, order)
    }()

    w.WriteHeader(http.StatusOK)
}
```

---

## 4. الإيقاف السلس للخدمات: `signal.NotifyContext` (Go 1.16+)

النمط المعياري العالمي لضمان عدم قطع أي طلبات حية أثناء إعادة تشغيل الخادم في Kubernetes أو Docker:

```go
func Run() error {
    // الاستماع لإشارات المقاطعة والإنهاء من نظام التشغيل
    ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
    defer stop()

    srv := &http.Server{Addr: ":8080", Handler: newRouter()}

    go func() {
        if err := srv.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
            log.Fatalf("listen error: %v", err)
        }
    }()

    // انتظار وصول إشارة الإيقاف
    <-ctx.Done()
    slog.Info("shutting down gracefully...")

    // مهلة 10 ثوانٍ لإنهاء الطلبات المتبقية
    shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
    defer cancel()

    return srv.Shutdown(shutdownCtx)
}
```

---

## 5. تنظيف الموارد فائق الكفاءة: `context.AfterFunc` (Go 1.21+)

بدلاً من تشغيل Goroutine مخصص لكل مورد للانتظار على `<-ctx.Done()`، تتيح `AfterFunc` تسجيل دالة استدعاء خلفي (Callback) تنفذ فوراً وبكفاءة عالية:

```go
func CancelableRead(ctx context.Context, conn net.Conn, b []byte) (int, error) {
    stop := context.AfterFunc(ctx, func() {
        // فك حظر دالة Read فور الإلغاء بضبط مهلة القراءة إلى الماضي
        conn.SetReadDeadline(time.Now())
    })
    defer stop() // إلغاء التسجيل إذا اكتملت القراءة طبيعياً

    return conn.Read(b)
}
```
