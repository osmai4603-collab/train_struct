package examples

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"time"

	"github.com/lestrrat-go/jwx/v2/jwk"
	"github.com/lestrrat-go/jwx/v2/jwt"
)

var (
	ErrKeyNotFound     = errors.New("signing key not found in jwks")
	ErrTokenValidation = errors.New("token validation failed")
)

// JWKSValidator يدير التحقق من الرموز عبر كاش JWKS تلقائي التحديث في الذاكرة
type JWKSValidator struct {
	cache     *jwk.Cache
	jwksURL   string
	issuer    string
	audience  string
	cachedSet jwk.Set
}

// NewJWKSValidator ينشئ مدققاً مع كاش دوري يعمل كخلفية مستمرة
func NewJWKSValidator(ctx context.Context, jwksURL, expectedIssuer, expectedAudience string) (*JWKSValidator, error) {
	// إنشاء كاش مرتبط بسياق التطبيق
	c := jwk.NewCache(ctx)

	// تسجيل الرابط مع تحديد أدنى فترة للتحديث (15 دقيقة) وتحديث دوري كل ساعة
	err := c.Register(jwksURL,
		jwk.WithMinRefreshInterval(15*time.Minute),
		jwk.WithRefreshInterval(1*time.Hour),
		jwk.WithHTTPClient(&http.Client{
			Timeout: 10 * time.Second,
		}),
	)
	if err != nil {
		return nil, fmt.Errorf("failed to register jwks url: %w", err)
	}

	// جلب أولي فوري للتأكد من الاتصال وصحة المفاتيح عند الإقلاع (Fail-Fast)
	_, err = c.Refresh(ctx, jwksURL)
	if err != nil {
		return nil, fmt.Errorf("initial jwks fetch failed: %w", err)
	}

	cachedSet := jwk.NewCachedSet(c, jwksURL)

	return &JWKSValidator{
		cache:     c,
		jwksURL:   jwksURL,
		issuer:    expectedIssuer,
		audience:  expectedAudience,
		cachedSet: cachedSet,
	}, nil
}

// ValidateToken يفحص سلامة التوقيع وصلاحية الادعاءات بالكامل
func (v *JWKSValidator) ValidateToken(ctx context.Context, tokenBytes []byte) (jwt.Token, error) {
	// التحقق الصارم وفق RFC 8725
	parsedToken, err := jwt.Parse(
		tokenBytes,
		jwt.WithKeySet(v.cachedSet),
		jwt.WithValidate(true),
		jwt.WithIssuer(v.issuer),
		jwt.WithAudience(v.audience),
		jwt.WithAcceptableSkew(1*time.Minute),
	)
	if err != nil {
		// في حال عدم وجود المفتاح (تدوير حديث للمفتاح)، يتم محاولة تحديث الكاش قسرياً لمرة واحدة
		if errors.Is(err, jwk.ErrKeyNotFound) {
			if _, refreshErr := v.cache.Refresh(ctx, v.jwksURL); refreshErr == nil {
				return jwt.Parse(
					tokenBytes,
					jwt.WithKeySet(v.cachedSet),
					jwt.WithValidate(true),
					jwt.WithIssuer(v.issuer),
					jwt.WithAudience(v.audience),
					jwt.WithAcceptableSkew(1*time.Minute),
				)
			}
		}
		return nil, fmt.Errorf("%w: %v", ErrTokenValidation, err)
	}

	return parsedToken, nil
}
