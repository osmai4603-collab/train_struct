# 05. استراتيجيات اختبار أنظمة الأخطاء (Testing Error Handling)

> **الهدف:** اختبار أنظمة التعامل مع الأخطاء بدقة، والتأكد من صحة سلاسل التغليف، وتعيين رموز HTTP، وعدم تسريب أي بيانات سرية.

---

## 1. اختبار سلاسل التغليف عبر `errors.Is` و `errors.As`

عند اختبار دالة ترجع خطأ، اختبر **سلوك الخطأ وهويته** بدلاً من مطابقة النصوص الحرفية:

```go
func TestService_OrderNotFound(t *testing.T) {
    svc := NewOrderService(&mockRepo{})
    _, err := svc.GetOrder(context.Background(), "non-existent")

    // فحص الهوية عبر errors.Is
    if !platformerr.Is(err, platformerr.CodeNotFound) {
        t.Fatalf("expected NOT_FOUND code, got: %v", err)
    }

    // فحص استخراج الهيكل عبر errors.As
    var appErr *platformerr.Error
    if !errors.As(err, &appErr) {
        t.Fatalf("expected *platformerr.Error, got: %T", err)
    }

    if appErr.Op != "orderService.GetOrder" {
        t.Errorf("expected op orderService.GetOrder, got %s", appErr.Op)
    }
}
```

---

## 2. اختبار تعيين رموز حالة الـ HTTP وتنسيق الردود

```go
func TestWriteHTTPError_Integration(t *testing.T) {
    w := httptest.NewRecorder()
    r := httptest.NewRequest(http.MethodGet, "/users/1", nil)

    err := platformerr.NotFound("users.Get", "user missing", nil)
    platformerr.WriteHTTPError(w, r, err, nil)

    resp := w.Result()
    if resp.StatusCode != http.StatusNotFound {
        t.Errorf("expected status 404, got %d", resp.StatusCode)
    }

    var body map[string]any
    _ = json.NewDecoder(resp.Body).Decode(&body)
    if body["code"] != "NOT_FOUND" {
        t.Errorf("expected code NOT_FOUND, got %v", body["code"])
    }
}
```

---

## 3. اختبارات الجداول (Table-Driven Tests) للأكواد والحالات

```go
func TestHTTPStatusMapping(t *testing.T) {
    tests := []struct {
        code       string
        wantStatus int
    }{
        {platformerr.CodeNotFound, http.StatusNotFound},
        {platformerr.CodeInvalid, http.StatusBadRequest},
        {platformerr.CodeConflict, http.StatusConflict},
        {platformerr.CodeUnauthorized, http.StatusUnauthorized},
        {platformerr.CodeForbidden, http.StatusForbidden},
        {platformerr.CodeRateLimited, http.StatusTooManyRequests},
        {platformerr.CodeTimeout, http.StatusGatewayTimeout},
        {platformerr.CodeBadGateway, http.StatusBadGateway},
        {platformerr.CodeUnavailable, http.StatusServiceUnavailable},
        {platformerr.CodeInternal, http.StatusInternalServerError},
    }

    for _, tt := range tests {
        t.Run(tt.code, func(t *testing.T) {
            err := &platformerr.Error{Code: tt.code}
            if got := platformerr.HTTPStatusCode(err); got != tt.wantStatus {
                t.Errorf("HTTPStatusCode(%s) = %d; want %d", tt.code, got, tt.wantStatus)
            }
        })
    }
}
```
