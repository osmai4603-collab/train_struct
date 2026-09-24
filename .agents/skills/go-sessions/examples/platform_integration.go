package examples

import (
	"context"
	"errors"
	"net/http"
	"time"

	platformctx "train/internal/platform/context"
	platformerrors "train/internal/platform/errors"
	platformlogger "train/internal/platform/logger"
)

// PlatformSessionMiddleware وسيط متكامل يربط الجلسة بجميع حزم المنصة
func PlatformSessionMiddleware(mgr *Manager) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			const op = "session.PlatformMiddleware"
			log := platformlogger.FromContext(r.Context())

			rawToken := mgr.extractCookie(r)
			if rawToken == "" {
				// لا توجد جلسة مرفقة، نتابع الطلب كزائر غير مسجل
				next.ServeHTTP(w, r)
				return
			}

			tokenHash := mgr.hashToken(rawToken)
			data, err := mgr.store.Get(r.Context(), tokenHash)
			if err != nil {
				if errors.Is(err, ErrSessionNotFound) {
					// إتلاف الكوكي الفاسد أو المنتهي فوراً
					mgr.DestroySession(r.Context(), w, r)
					errResp := platformerrors.Unauthorized(op, "session has expired or does not exist", err)
					http.Error(w, errResp.Error(), http.StatusUnauthorized)
					return
				}
				errResp := platformerrors.Internal(op, "failed to query session store", err)
				http.Error(w, errResp.Error(), http.StatusInternalServerError)
				return
			}

			now := time.Now().UTC()

			// التحقق من الانتهاء المطلق
			if now.After(data.AbsoluteExp) {
				_ = mgr.store.Delete(r.Context(), tokenHash)
				mgr.DestroySession(r.Context(), w, r)
				errResp := platformerrors.Unauthorized(op, "absolute session timeout reached; please re-login", nil)
				http.Error(w, errResp.Error(), http.StatusUnauthorized)
				return
			}

			// كبح تحديث وقت النشاط
			if now.Sub(data.LastActiveAt) >= mgr.config.ThrottleInterval {
				data.LastActiveAt = now
				ttl := time.Until(data.AbsoluteExp)
				if ttl > mgr.config.IdleTimeout {
					ttl = mgr.config.IdleTimeout
				}
				if setErr := mgr.store.Set(r.Context(), tokenHash, data, ttl); setErr != nil {
					log.Warn("failed to extend session ttl", "error", setErr)
				}
			}

			// 1. إثراء RequestMetadata الخاص بالمنصة بهوية المستخدم
			ctx := platformctx.WithUserID(r.Context(), data.UserID)

			// 2. حقن بيانات الجلسة الكاملة
			ctx = context.WithValue(ctx, sessionContextKey, data)

			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}

// LogSessionAudit يسجل أحداث الجلسات بصورة آمنة مع تعتيم المعرف الحساس
func LogSessionAudit(ctx context.Context, op string, event string, userID string, rawToken string) {
	log := platformlogger.FromContext(ctx)

	// تعتيم المعرف تلقائياً لكي يظهر كـ *** في السجلات
	maskedToken := platformlogger.NewSensitive(rawToken)

	log.Info("session security audit",
		"op", op,
		"event", event,
		"user_id", userID,
		"token", maskedToken,
	)
}
