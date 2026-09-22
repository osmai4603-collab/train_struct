# 06. استراتيجيات اختبار السجلات (Testing Strategies)

> **المصادر المرجعية:** Go Standard Library: `testing/slogtest`, Go Testing Conventions.

---

## 1. اختبار استدعاءات السجلات عبر `bytes.Buffer`

عند الحاجة لاختبار أن خدمة معينة تسجل رسائل أو سمات محددة (مثل تسجيل حدث تسجيل الدخول أو معرف الطلب):

```go
package user_test

import (
    "bytes"
    "context"
    "encoding/json"
    "log/slog"
    "testing"
)

func TestUserService_LogsUserAction(t *testing.T) {
    var buf bytes.Buffer
    handler := slog.NewJSONHandler(&buf, nil)
    logger := slog.New(handler)

    // حقن الـ logger التجريبي في الخدمة
    svc := NewUserService(logger)
    svc.DoAction(context.Background(), "user-123")

    // فحص مخرجات الـ JSON
    var entry map[string]any
    if err := json.Unmarshal(buf.Bytes(), &entry); err != nil {
        t.Fatalf("failed to parse log json: %v", err)
    }

    if entry["msg"] != "user action completed" {
        t.Errorf("expected msg 'user action completed', got %v", entry["msg"])
    }
    if entry["user_id"] != "user-123" {
        t.Errorf("expected user_id 'user-123', got %v", entry["user_id"])
    }
}
```

---

## 2. اختبار امتثال الـ Custom Handler عبر `testing/slogtest`

توفر Go الحزمة القياسية `testing/slogtest` للتحقق من أن أي معالج مخصص يلتزم بجميع مواصفات بروتوكول `slog.Handler` (مثل معالجة المجموعات، والسمات المتكررة، ومعالجة القيم الخاصة).

### دالة `slogtest.TestHandler`:
```go
package handler_test

import (
    "bytes"
    "encoding/json"
    "testing"
    "testing/slogtest"
)

func TestMyCustomHandler_Compliance(t *testing.T) {
    var buf bytes.Buffer
    handler := NewMyCustomHandler(&buf)

    // slogtest ينفذ سلسلة من الحالات الاختبارية الصارمة للتأكد من المعايير
    err := slogtest.TestHandler(handler, func() []map[string]any {
        var entries []map[string]any
        for _, line := range bytes.Split(buf.Bytes(), []byte{'\n'}) {
            if len(line) == 0 {
                continue
            }
            var m map[string]any
            if err := json.Unmarshal(line, &m); err != nil {
                t.Fatal(err)
            }
            entries = append(entries, m)
        }
        return entries
    })

    if err != nil {
        t.Fatalf("custom handler failed slogtest compliance: %v", err)
    }
}
```

---

## 3. توجيه السجلات إلى مخرجات الاختبار (`testing.TB.Log`)

في اختبارات التكامل أو اختبارات الوحدة، لا يُحبذ كتابة السجلات في `os.Stdout` لأنها تشوش مخرجات `go test`. الأفضل هو توجيه السجلات إلى `t.Log` حتى تظهر فقط عند فشل الاختبار أو عند استخدام الراية `-v`:

```go
type testLogWriter struct {
    tb testing.TB
}

func (w *testLogWriter) Write(p []byte) (n int, err error) {
    w.tb.Helper()
    w.tb.Log(string(bytes.TrimSpace(p)))
    return len(p), nil
}

// NewTestLogger ينشئ سجلاً مخصصاً للاختبارات
func NewTestLogger(tb testing.TB) *slog.Logger {
    writer := &testLogWriter{tb: tb}
    handler := slog.NewTextHandler(writer, &slog.HandlerOptions{
        Level: slog.LevelDebug,
    })
    return slog.New(handler)
}
```
