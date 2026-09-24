# 06. الحماية من هجمات CSRF والاقتران بالجلسات (`CSRF Mitigation & Session Pairing`)

عند استخدام ملفات تعريف الارتباط (Cookies) لحمل معرفات الجلسات، يرسل المتصفح الكوكي تلقائياً مع كل طلب يتطابق مع نطاقه. يفتح هذا السلوك الباب أمام هجمات **تزوير الطلبات عبر المواقع (Cross-Site Request Forgery - CSRF)**. يشرح هذا الدليل كيفية تطبيق دفاع متكامل يربط رمز الحماية بالجلسة.

---

## 1. لماذا لا تكفي خاصية `SameSite=Lax` وحدها؟

على الرغم من أن `SameSite=Lax` توفر حماية مبدئية ممتازة، إلا أنها لا تغني عن منظومة حماية صريحة للأسباب التالية:

1. **طلبات التنقل من المستوى الأعلى (Top-Level GET Navigations):** يُرسل المتصفح كوكيز `Lax` عند النقر على روابط تقود إلى موقعك. إذا كان هناك أي إجراء على الخادم يغير الحالة عن طريق الخطأ عبر طلب `GET`، فسيتم تنفيذه بنجاح.
2. **هجمات النطاقات الفرعية (Subdomain Vulnerabilities):** إذا تعرض تطبيق فرعي آخر على نفس النطاق للاختراق (`vulnerable.example.com`)، يستطيع إرسال طلبات عابرة للمواقع تعامل كأنها من نفس الموقع (Same-Site).
3. **تطبيقات الهواتف والمتصفحات القديمة:** قد لا تدعم بعض البيئات القديمة أو غير القياسية خاصية `SameSite` بشكل صارم.

---

## 2. نمط الرمز المتزامن (Synchronizer Token Pattern)

يُعتبر نمط الرمز المتزامن المرتبط بالجلسة المعيار الأكثر أماناً وفقاً لدليل **[OWASP CSRF Prevention Cheat Sheet](https://cheatsheetseries.owasp.org/cheatsheets/Cross-Site_Request_Forgery_Prevention_Cheat_Sheet.html)**:

```text
[ Browser / Single-Page-App ]                             [ Go API Server / Session ]
               │                                                        │
               ├─────── 1. Login / Init Session ───────────────────────>│
               │<────── 2. Set-Cookie + CSRF Token in Body/Header ──────┤ (حفظ الرمز في الجلسة)
               │                                                        │
               │   3. State-Changing Request (POST /account/email)      │
               ├───────────────────────────────────────────────────────>│
               │   Headers:                                             │
               │     Cookie: __Host-sess=<raw_token>                    │
               │     X-CSRF-Token: <csrf_token>                         │
               │                                                        │ 4. المقارنة بالوقت الثابت
               │                                                        │    ConstantTimeCompare(Header, Session)
               │<────── 5. 200 OK or 403 Forbidden ─────────────────────┤
```

---

## 3. تطبيق وسيط الحماية في Go

```go
package session

import (
 "crypto/subtle"
 "net/http"
)

// CSRFMiddleware يتحقق من وجود وصحة رمز الـ CSRF في كافة الطلبات المعدلة للحالة
func CSRFMiddleware(next http.Handler) http.Handler {
 return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
  // السماح بالطلبات الآمنة (Safe Methods) دون فحص الرمز
  switch r.Method {
  case http.MethodGet, http.MethodHead, http.MethodOptions, http.MethodTrace:
   next.ServeHTTP(w, r)
   return
  }

  // استخراج الجلسة من السياق
  sess, ok := FromContext(r.Context())
  if !ok || sess.CSRFToken == "" {
   http.Error(w, `{"error":"forbidden: no active session found"}`, http.StatusForbidden)
   return
  }

  // استخراج الرمز من الترويسة المخصصة أو حقول النموذج
  clientToken := r.Header.Get("X-CSRF-Token")
  if clientToken == "" {
   clientToken = r.FormValue("csrf_token")
  }

  // المقارنة بالوقت الثابت لمنع هجمات التوقيت
  if subtle.ConstantTimeCompare([]byte(clientToken), []byte(sess.CSRFToken)) != 1 {
   http.Error(w, `{"error":"forbidden: invalid or missing csrf token"}`, http.StatusForbidden)
   return
  }

  next.ServeHTTP(w, r)
 })
}
```
