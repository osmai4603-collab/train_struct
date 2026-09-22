# 04. التزامن والذعر وأمان البيانات (Concurrency, Panic & Security)

> **الهدف:** توضيح كيفية إدارة الأخطاء في العمليات المتزامنة، ووضع حدود الذعر (Panic Recovery)، وحماية البيانات الحساسة من التسرب في السجلات والردود.

---

## 1. الأخطاء المتزامنة وحزمة `errgroup`

لا تطلق أبداً Goroutines بدون وسيلة لالتقاط أخطائها. النمط المعتمد هو استخدام `golang.org/x/sync/errgroup`:

```go
func FetchAllUserData(ctx context.Context, userID string) (*UserProfile, *UserOrders, error) {
    g, ctx := errgroup.WithContext(ctx)

    var profile *UserProfile
    var orders *UserOrders

    g.Go(func() error {
        p, err := fetchProfile(ctx, userID)
        if err != nil {
            return fmt.Errorf("fetchProfile failed: %w", err)
        }
        profile = p
        return nil
    })

    g.Go(func() error {
        o, err := fetchOrders(ctx, userID)
        if err != nil {
            return fmt.Errorf("fetchOrders failed: %w", err)
        }
        orders = o
        return nil
    })

    if err := g.Wait(); err != nil {
        return nil, nil, err
    }

    return profile, orders, nil
}
```

---

## 2. نمط التقاط أخطاء `defer` مع دمجها (`errors.Join`)

خطأ شائع جداً: تجاهل خطأ دالة `Close()` في `defer`. الحل القياسي باستخدام القيم المعادة المسماة:

```go
func CopyFile(dst, src string) (err error) {
    in, err := os.Open(src)
    if err != nil {
        return err
    }
    defer func() {
        if closeErr := in.Close(); closeErr != nil {
            err = errors.Join(err, fmt.Errorf("close src file: %w", closeErr))
        }
    }()

    out, err := os.Create(dst)
    if err != nil {
        return err
    }
    defer func() {
        if closeErr := out.Close(); closeErr != nil {
            err = errors.Join(err, fmt.Errorf("close dst file: %w", closeErr))
        }
    }()

    _, err = io.Copy(out, in)
    return err
}
```

---

## 3. حدود الذعر والاسترداد (Panic Recovery Boundary)

القاعدة الذهبية في Go: **لا تستخدم `panic` للتحكم في تدفق البرنامج العادي.**

يجب وضع وسيط استرداد (Recovery Middleware) عند حدود خادم الويب:

```go
func RecoveryMiddleware(next http.Handler, logger *slog.Logger) http.Handler {
    return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
        defer func() {
            if rvr := recover(); rvr != nil {
                stack := debug.Stack()
                logger.ErrorContext(r.Context(), "panic recovered in handler",
                    slog.Any("panic", rvr),
                    slog.String("stack", string(stack)),
                    slog.String("path", r.URL.Path),
                )
                
                // إعادة رد خطأ آمن للمستخدم دون كشف أي تفاصيل
                w.Header().Set("Content-Type", "application/json")
                w.WriteHeader(http.StatusInternalServerError)
                json.NewEncoder(w).Encode(map[string]string{
                    "code":    "INTERNAL",
                    "message": "A critical server error occurred",
                })
            }
        }()

        next.ServeHTTP(w, r)
    })
}
```

---

## 4. أمان البيانات وتنقية الأسرار (PII Redaction)

1. **حجب تفاصيل البنية التحتية:** لا تكشف عناوين IP داخلية، أسماء مستخدمي قواعد البيانات، أو أخطاء المحركات.
2. **تنقية الأسرار:** لا تضع كلمات مرور أو رموز API في رسائل الخطأ:

```go
// ❌ كارثة أمنية:
return fmt.Errorf("failed to authenticate %s with password %s: %w", user, pass, err)

// ✅ أسلوب آمن:
return fmt.Errorf("failed to authenticate user %q: %w", user, err)
```
