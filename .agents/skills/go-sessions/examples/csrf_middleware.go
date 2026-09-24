package examples

import (
	"crypto/subtle"
	"net/http"
)

// CSRFConfig إعدادات وسيط الحماية من CSRF
type CSRFConfig struct {
	HeaderName string
	FormField  string
}

// CSRFMiddleware وسيط صارم للتحقق من رمز الحماية المزدوجة المقترن بالجلسة
func CSRFMiddleware(cfg CSRFConfig) func(http.Handler) http.Handler {
	if cfg.HeaderName == "" {
		cfg.HeaderName = "X-CSRF-Token"
	}
	if cfg.FormField == "" {
		cfg.FormField = "csrf_token"
	}

	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			// تجاوز الفحص للطلبات الآمنة (Idempotent Methods)
			switch r.Method {
			case http.MethodGet, http.MethodHead, http.MethodOptions, http.MethodTrace:
				next.ServeHTTP(w, r)
				return
			}

			// استخراج بيانات الجلسة من السياق
			sess, ok := FromContext(r.Context())
			if !ok || sess.CSRFToken == "" {
				http.Error(w, `{"error":"forbidden: session missing or invalid"}`, http.StatusForbidden)
				return
			}

			// استخراج رمز العميل من الترويسة أولاً ثم من النموذج
			clientToken := r.Header.Get(cfg.HeaderName)
			if clientToken == "" {
				clientToken = r.PostFormValue(cfg.FormField)
			}

			// فحص الطول والمقارنة بالوقت الثابت لمنع هجمات التوقيت
			if len(clientToken) == 0 || subtle.ConstantTimeCompare([]byte(clientToken), []byte(sess.CSRFToken)) != 1 {
				http.Error(w, `{"error":"forbidden: invalid csrf token"}`, http.StatusForbidden)
				return
			}

			next.ServeHTTP(w, r)
		})
	}
}
