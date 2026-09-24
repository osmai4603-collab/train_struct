package examples

import (
	"crypto/ed25519"
	"errors"
	"fmt"
	"time"

	"github.com/golang-jwt/jwt/v5"
)

var (
	ErrInvalidToken = errors.New("invalid or tampered token")
	ErrExpiredToken = errors.New("token has expired")
)

// AppClaims الهيكل المعياري لادعاءات التطبيق
type AppClaims struct {
	UserID   string   `json:"sub"`
	TenantID string   `json:"tid,omitempty"`
	Roles    []string `json:"roles"`
	jwt.RegisteredClaims
}

// TokenService يدير توليد وفحص الرموز باستخدام التوقيع الرقمي غير المتناظر Ed25519
type TokenService struct {
	privateKey ed25519.PrivateKey
	publicKey  ed25519.PublicKey
	issuer     string
	audience   string
}

// NewTokenService ينشئ خدمة الرموز بالمفاتيح المعتمدة
func NewTokenService(priv ed25519.PrivateKey, pub ed25519.PublicKey, issuer, audience string) *TokenService {
	return &TokenService{
		privateKey: priv,
		publicKey:  pub,
		issuer:     issuer,
		audience:   audience,
	}
}

// IssueAccessToken ينشئ رمز وصول قصير الأجل (15 دقيقة)
func (s *TokenService) IssueAccessToken(userID string, roles []string, jti string) (string, error) {
	now := time.Now().UTC()
	claims := AppClaims{
		UserID: userID,
		Roles:  roles,
		RegisteredClaims: jwt.RegisteredClaims{
			Issuer:    s.issuer,
			Subject:   userID,
			Audience:  jwt.ClaimStrings{s.audience},
			ExpiresAt: jwt.NewNumericDate(now.Add(15 * time.Minute)), // 15 دقيقة كحد أقصى
			IssuedAt:  jwt.NewNumericDate(now),
			NotBefore: jwt.NewNumericDate(now),
			ID:        jti,
		},
	}

	token := jwt.NewWithClaims(jwt.SigningMethodEdDSA, claims)
	token.Header["typ"] = "at+jwt" // RFC 9068 Explicit Typing

	return token.SignedString(s.privateKey)
}

// VerifyToken يتحقق من صحة وسلامة الرمز التشفيرية والزمنية
func (s *TokenService) VerifyToken(tokenString string) (*AppClaims, error) {
	token, err := jwt.ParseWithClaims(tokenString, &AppClaims{}, func(t *jwt.Token) (any, error) {
		// التأكد الصارم من أن الخوارزمية هي EdDSA المعتمدة
		if _, ok := t.Method.(*jwt.SigningMethodEd25519); !ok {
			return nil, fmt.Errorf("unexpected signing algorithm: %v", t.Header["alg"])
		}
		return s.publicKey, nil
	},
		// خيارات الأمان الإلزامية وفق RFC 8725
		jwt.WithValidMethods([]string{jwt.SigningMethodEdDSA.Alg()}),
		jwt.WithIssuer(s.issuer),
		jwt.WithAudience(s.audience),
		jwt.WithExpirationRequired(),
		jwt.WithLeeway(1*time.Minute), // السماح بهامش دقيقة واحدة لفروق ساعات الخوادم
	)

	if err != nil {
		if errors.Is(err, jwt.ErrTokenExpired) {
			return nil, ErrExpiredToken
		}
		return nil, fmt.Errorf("%w: %v", ErrInvalidToken, err)
	}

	claims, ok := token.Claims.(*AppClaims)
	if !ok || !token.Valid {
		return nil, ErrInvalidToken
	}

	return claims, nil
}
