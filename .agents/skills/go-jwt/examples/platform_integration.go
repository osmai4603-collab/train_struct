package examples

import (
	"context"
	"errors"
	"net/http"
	"strings"
	"time"

	"github.com/golang-jwt/jwt/v5"

	postgresinfra "train/internal/infrastructure/postgres"
	platformctx "train/internal/platform/context"
	platformerrors "train/internal/platform/errors"
	platformlogger "train/internal/platform/logger"
)

// PlatformIntegratedVerifier يدمج التحقق من الرموز مع منصة المشروع
type PlatformIntegratedVerifier struct {
	issuer    string
	audience  string
	publicKey any
	methods   []string
}

func NewPlatformIntegratedVerifier(issuer, audience string, pubKey any, methods []string) *PlatformIntegratedVerifier {
	return &PlatformIntegratedVerifier{
		issuer:    issuer,
		audience:  audience,
		publicKey: pubKey,
		methods:   methods,
	}
}

// VerifyTokenAndTranslateErrors يفحص الرمز ويترجم أي فشل إلى platformerrors
func (v *PlatformIntegratedVerifier) VerifyTokenAndTranslateErrors(tokenStr string) (*AppClaims, error) {
	const op = "jwt.VerifyToken"

	token, err := jwt.ParseWithClaims(tokenStr, &AppClaims{}, func(t *jwt.Token) (any, error) {
		return v.publicKey, nil
	},
		jwt.WithValidMethods(v.methods),
		jwt.WithIssuer(v.issuer),
		jwt.WithAudience(v.audience),
		jwt.WithExpirationRequired(),
		jwt.WithLeeway(1*time.Minute),
	)

	if err != nil {
		if errors.Is(err, jwt.ErrTokenExpired) {
			return nil, platformerrors.Unauthorized(op, "token has expired", err)
		}
		if errors.Is(err, jwt.ErrTokenSignatureInvalid) {
			return nil, platformerrors.Unauthorized(op, "invalid token signature", err)
		}
		if errors.Is(err, jwt.ErrTokenMalformed) {
			return nil, platformerrors.Invalid(op, "malformed token format", err)
		}
		return nil, platformerrors.Unauthorized(op, "token verification failed", err)
	}

	claims, ok := token.Claims.(*AppClaims)
	if !ok || !token.Valid {
		return nil, platformerrors.Unauthorized(op, "invalid token claims", nil)
	}

	return claims, nil
}

// IntegratedAuthMiddleware وسيط net/http يربط بين platform/context و platform/errors و platform/logger
func IntegratedAuthMiddleware(verifier *PlatformIntegratedVerifier) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			const op = "middleware.Authenticate"
			ctx := r.Context()
			log := platformlogger.FromContext(ctx)

			authHeader := r.Header.Get("Authorization")
			if authHeader == "" {
				err := platformerrors.Unauthorized(op, "missing authorization header", nil)
				writePlatformError(w, err)
				return
			}

			tokenStr, ok := strings.CutPrefix(authHeader, "Bearer ")
			if !ok || tokenStr == "" {
				err := platformerrors.Unauthorized(op, "invalid authorization format; expected Bearer <token>", nil)
				writePlatformError(w, err)
				return
			}

			// 1. التحقق من الرمز وترجمة الأخطاء لهيكل platformerrors
			claims, err := verifier.VerifyTokenAndTranslateErrors(tokenStr)
			if err != nil {
				log.Warn("authentication failed",
					"op", op,
					"error", err,
					"token", platformlogger.NewSensitive(tokenStr), // تعتيم تلقائي بـ ***
				)
				writePlatformError(w, err)
				return
			}

			// 2. التكامل مع platform/context: إثراء RequestMetadata بـ UserID و TenantID
			ctx = platformctx.WithUserID(ctx, claims.UserID)
			if claims.TenantID != "" {
				ctx = platformctx.WithTenantID(ctx, claims.TenantID)
			}

			// 3. حقن كائن الادعاءات الكامل في السياق
			ctx = WithUserClaims(ctx, claims)

			log.Debug("user authenticated successfully",
				"user_id", claims.UserID,
				"tenant_id", claims.TenantID,
			)

			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}

// PostgresSessionRepository تنفيذ مستودع جلسات تدوير الرموز عبر postgres.DBTX
type PostgresSessionRepository struct {
	db postgresinfra.DBTX
}

func NewPostgresSessionRepository(db postgresinfra.DBTX) *PostgresSessionRepository {
	return &PostgresSessionRepository{db: db}
}

// Get يسترجع سجل الجلسة ورمز التحديث
func (r *PostgresSessionRepository) Get(ctx context.Context, tokenID string) (*SessionRecord, error) {
	const op = "postgres.GetSession"
	query := `
		SELECT id, family_id, user_id, is_used, expires_at
		FROM auth_refresh_tokens
		WHERE id = $1
	`
	row := r.db.QueryRow(ctx, query, tokenID)

	var record SessionRecord
	err := row.Scan(&record.ID, &record.FamilyID, &record.UserID, &record.IsUsed, &record.ExpiresAt)
	if err != nil {
		return nil, postgresinfra.TranslateError(op, err)
	}

	return &record, nil
}

// MarkAsUsed يعلّم الرمز كمستهلك ضمن معاملة ذرية
func (r *PostgresSessionRepository) MarkAsUsed(ctx context.Context, tokenID string) error {
	const op = "postgres.MarkSessionAsUsed"
	query := `UPDATE auth_refresh_tokens SET is_used = TRUE WHERE id = $1`
	_, err := r.db.Exec(ctx, query, tokenID)
	if err != nil {
		return postgresinfra.TranslateError(op, err)
	}
	return nil
}

// RevokeFamily يبطل عائلة الرموز بالكامل فور كشف إعادة الاستخدام
func (r *PostgresSessionRepository) RevokeFamily(ctx context.Context, familyID string) error {
	const op = "postgres.RevokeFamily"
	query := `UPDATE auth_refresh_tokens SET is_used = TRUE, expires_at = NOW() WHERE family_id = $1`
	_, err := r.db.Exec(ctx, query, familyID)
	if err != nil {
		return postgresinfra.TranslateError(op, err)
	}
	return nil
}

// Create ينشئ جلسة جديدة برمز تحديث غير مستهلك
func (r *PostgresSessionRepository) Create(ctx context.Context, record *SessionRecord) error {
	const op = "postgres.CreateSession"
	query := `
		INSERT INTO auth_refresh_tokens (id, family_id, user_id, is_used, expires_at)
		VALUES ($1, $2, $3, $4, $5)
	`
	_, err := r.db.Exec(ctx, query, record.ID, record.FamilyID, record.UserID, record.IsUsed, record.ExpiresAt)
	if err != nil {
		return postgresinfra.TranslateError(op, err)
	}
	return nil
}

// writePlatformError يكتب الخطأ المهيكل في الرد بـ HTTP Status المناسب
func writePlatformError(w http.ResponseWriter, err error) {
	status := platformerrors.HTTPStatus(err)
	msg := platformerrors.ErrorMessage(err)
	code := platformerrors.ErrorCode(err)

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_, _ = w.Write([]byte(`{"error":{"code":"` + code + `","message":"` + msg + `"}}`))
}
