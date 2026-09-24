# 07. استراتيجيات الاختبار والمحاكاة والأداء (`Testing & Benchmarking`)

تخضع البنية التحتية للجلسات لاختبارات صارمة لضمان موثوقيتها التامة تحت أعباء العمل المرتفعة، وخلوها التام من سباقات البيانات، ومحافظتها على سرعة استجابة فائقة.

---

## 1. كشف سباق البيانات عبر فاحص السباق (`go test -race`)

في مشاريع Go، يجب تشغيل اختبارات الجلسات باستخدام علم فاحص السباق الإلزامي لضمان عدم وجود تداخلات بين طلبات الـ Goroutines المتزامنة:

```bash
go test -v -race -run TestSessionConcurrency ./internal/infrastructure/session/...
```

### اختبار التزامن النموذجي

```go
package session_test

import (
 "context"
 "net/http"
 "net/http/httptest"
 "sync"
 "testing"
 "time"

 "train/internal/infrastructure/session"
)

// MockStore مخزن ذاكرة آمن للاختبارات فقط
type MockStore struct {
 mu   sync.RWMutex
 data map[string]*session.SessionData
}

func NewMockStore() *MockStore {
 return &MockStore{data: make(map[string]*session.SessionData)}
}

func (s *MockStore) Get(ctx context.Context, hash string) (*session.SessionData, error) {
 s.mu.RLock()
 defer s.mu.RUnlock()
 d, ok := s.data[hash]
 if !ok {
  return nil, session.ErrSessionNotFound
 }
 return d, nil
}

func (s *MockStore) Set(ctx context.Context, hash string, d *session.SessionData, ttl time.Duration) error {
 s.mu.Lock()
 defer s.mu.Unlock()
 s.data[hash] = d
 return nil
}

func (s *MockStore) Delete(ctx context.Context, hash string) error {
 s.mu.Lock()
 defer s.mu.Unlock()
 delete(s.data, hash)
 return nil
}

func (s *MockStore) DeleteByUserID(ctx context.Context, userID string) error {
 s.mu.Lock()
 defer s.mu.Unlock()
 for k, v := range s.data {
  if v.UserID == userID {
   delete(s.data, k)
  }
 }
 return nil
}

func TestSessionConcurrency(t *testing.T) {
 store := NewMockStore()
 manager := session.NewManager(store, session.Config{
  CookieName: "__Host-sess",
 })

 // 1. إنشاء جلسة أولية
 wInit := httptest.NewRecorder()
 sess, err := manager.CreateSession(context.Background(), wInit, "user_42", []string{"user"})
 if err != nil {
  t.Fatalf("failed to create session: %v", err)
 }

 cookie := wInit.Result().Cookies()[0]

 // 2. إطلاق 100 Goroutine ترسل طلبات متزامنة بنفس الكوكي
 var wg sync.WaitGroup
 workers := 100

 handler := manager.Middleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
  s, ok := session.FromContext(r.Context())
  if !ok || s.UserID != "user_42" {
   t.Errorf("expected active session for user_42")
  }
  w.WriteHeader(http.StatusOK)
 }))

 for i := 0; i < workers; i++ {
  wg.Add(1)
  go func() {
   defer wg.Done()
   req := httptest.NewRequest("GET", "/dashboard", nil)
   req.AddCookie(cookie)
   rec := httptest.NewRecorder()
   handler.ServeHTTP(rec, req)
   if rec.Code != http.StatusOK {
    t.Errorf("unexpected status: %d", rec.Code)
   }
  }()
 }

 wg.Wait()
}
```

---

## 2. اختبارات قياس الأداء (Benchmarking)

يجب مراقبة معدل تخصيص الذاكرة (`B/op` و `allocs/op`) لضمان عدم تأثير الجلسات على أداء الـ Garbage Collector:

```go
func BenchmarkTokenHashing(b *testing.B) {
 rawToken := "0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef"
 b.ResetTimer()
 b.ReportAllocs()

 for i := 0; i < b.N; i++ {
  _ = session.HashToken(rawToken)
 }
}

func BenchmarkCSRFConstantTimeCompare(b *testing.B) {
 t1 := "e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855"
 t2 := "e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855"
 b.ResetTimer()
 b.ReportAllocs()

 for i := 0; i < b.N; i++ {
  _ = session.ValidateCSRFToken(t1, t2)
 }
}
```
