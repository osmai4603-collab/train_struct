package examples

import (
	"context"
	"errors"
	"net/http"
	"strings"
)

// نوع مفتاح غير مصدّر لمنع تصادم المفاتيح بين الحزم البرمجية
type contextKey struct{}

var userClaimsContextKey = contextKey{}

// TokenVerifier واجهة عزل التحقق لتسهيل اختبارات الوحدة
type TokenVerifier interface {
	VerifyToken(tokenString string) (*AppClaims, error)
	IsTokenRevoked(ctx context.Context, jti string) (bool, error)
}

// AuthMiddleware وسيط net/http للتحقق من الرموز وتمرير الادعاءات في السياق
func AuthMiddleware(verifier TokenVerifier) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			authHeader := r.Header.Get("Authorization")
			if authHeader == "" {
				http.Error(w, `{"error":"authorization header required"}`, http.StatusUnauthorized)
				return
			}

			// استخراج الرمز بكفاءة وتفادي التخصيصات العشوائية
			tokenStr, ok := strings.CutPrefix(authHeader, "Bearer ")
			if !ok || tokenStr == "" {
				http.Error(w, `{"error":"invalid authorization format; expected Bearer <token>"}`, http.StatusUnauthorized)
				return
			}

			// فحص صحة الرمز تشفيرياً وزمنياً
			claims, err := verifier.VerifyToken(tokenStr)
			if err != nil {
				if errors.Is(err, ErrExpiredToken) {
					http.Error(w, `{"error":"token has expired"}`, http.StatusUnauthorized)
					return
				}
				http.Error(w, `{"error":"invalid or untrusted token"}`, http.StatusUnauthorized)
				return
			}

			// فحص القائمة السوداء الفورية إن وُجد jti
			if claims.ID != "" {
				revoked, err := verifier.IsTokenRevoked(r.Context(), claims.ID)
				if err != nil || revoked {
					http.Error(w, `{"error":"token has been revoked"}`, http.StatusUnauthorized)
					return
				}
			}

			// حقن الادعاءات في السياق بأمان تام
			ctx := context.WithValue(r.Context(), userClaimsContextKey, claims)
			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}

// UserClaimsFromContext يستخرج الادعاءات المحقونة من السياق
func UserClaimsFromContext(ctx context.Context) (*AppClaims, bool) {
	claims, ok := ctx.Value(userClaimsContextKey).(*AppClaims)
	return claims, ok
}
