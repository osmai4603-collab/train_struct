package examples

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"net/http"
	"time"
)

var (
	ErrSessionNotFound = errors.New("session: not found or expired")
	ErrSessionExpired  = errors.New("session: absolute expiration reached")
)

type contextKey struct{}

var sessionContextKey = contextKey{}

// SessionData تمثل بيانات الجلسة المخزنة
type SessionData struct {
	UserID       string                 `json:"user_id"`
	Roles        []string               `json:"roles"`
	CSRFToken    string                 `json:"csrf_token"`
	CreatedAt    time.Time              `json:"created_at"`
	LastActiveAt time.Time              `json:"last_active_at"`
	AbsoluteExp  time.Time              `json:"absolute_exp"`
	Metadata     map[string]interface{} `json:"metadata,omitempty"`
}

// SessionStore واجهة تجريد التخزين
type SessionStore interface {
	Get(ctx context.Context, tokenHash string) (*SessionData, error)
	Set(ctx context.Context, tokenHash string, data *SessionData, ttl time.Duration) error
	Delete(ctx context.Context, tokenHash string) error
	DeleteByUserID(ctx context.Context, userID string) error
}

// Config إعدادات المدير
type Config struct {
	CookieName       string
	IdleTimeout      time.Duration
	AbsoluteTimeout  time.Duration
	CookieSecure     bool
	CookieSameSite   http.SameSite
	ThrottleInterval time.Duration
}

// Manager يُدير الجلسات وحقنها في السياق
type Manager struct {
	store  SessionStore
	config Config
}

func NewManager(store SessionStore, cfg Config) *Manager {
	if cfg.CookieName == "" {
		cfg.CookieName = "__Host-sess"
	}
	if cfg.IdleTimeout <= 0 {
		cfg.IdleTimeout = 30 * time.Minute
	}
	if cfg.AbsoluteTimeout <= 0 {
		cfg.AbsoluteTimeout = 12 * time.Hour
	}
	if cfg.CookieSameSite == 0 {
		cfg.CookieSameSite = http.SameSiteLaxMode
	}
	if cfg.ThrottleInterval <= 0 {
		cfg.ThrottleInterval = 1 * time.Minute
	}

	return &Manager{
		store:  store,
		config: cfg,
	}
}

// CreateSession ينشئ جلسة جديدة، يحفظها بالتجزئة، ويرسل الكوكي للعميل
func (m *Manager) CreateSession(ctx context.Context, w http.ResponseWriter, userID string, roles []string) (*SessionData, error) {
	rawToken, tokenHash, err := m.generateTokens()
	if err != nil {
		return nil, fmt.Errorf("failed to generate session tokens: %w", err)
	}

	csrfBytes := make([]byte, 32)
	if _, err := io.ReadFull(rand.Reader, csrfBytes); err != nil {
		return nil, fmt.Errorf("failed to generate csrf token: %w", err)
	}
	csrfToken := hex.EncodeToString(csrfBytes)

	now := time.Now().UTC()
	data := &SessionData{
		UserID:       userID,
		Roles:        roles,
		CSRFToken:    csrfToken,
		CreatedAt:    now,
		LastActiveAt: now,
		AbsoluteExp:  now.Add(m.config.AbsoluteTimeout),
		Metadata:     make(map[string]interface{}),
	}

	if err := m.store.Set(ctx, tokenHash, data, m.config.IdleTimeout); err != nil {
		return nil, fmt.Errorf("failed to store session: %w", err)
	}

	m.writeCookie(w, rawToken, m.config.IdleTimeout)
	return data, nil
}

// RegenerateToken يجدد معرف الجلسة فور المصادقة لمنع تثبيت الجلسة
func (m *Manager) RegenerateToken(ctx context.Context, w http.ResponseWriter, r *http.Request) error {
	rawToken := m.extractCookie(r)
	if rawToken == "" {
		return ErrSessionNotFound
	}
	oldHash := m.hashToken(rawToken)

	data, err := m.store.Get(ctx, oldHash)
	if err != nil {
		return err
	}

	newRaw, newHash, err := m.generateTokens()
	if err != nil {
		return err
	}

	now := time.Now().UTC()
	data.LastActiveAt = now
	ttl := time.Until(data.AbsoluteExp)
	if ttl > m.config.IdleTimeout {
		ttl = m.config.IdleTimeout
	}

	if err := m.store.Set(ctx, newHash, data, ttl); err != nil {
		return err
	}
	_ = m.store.Delete(ctx, oldHash)

	m.writeCookie(w, newRaw, ttl)
	return nil
}

// DestroySession يمحو الجلسة من المتجر ويحذف الكوكي
func (m *Manager) DestroySession(ctx context.Context, w http.ResponseWriter, r *http.Request) error {
	rawToken := m.extractCookie(r)
	if rawToken != "" {
		_ = m.store.Delete(ctx, m.hashToken(rawToken))
	}

	cookie := &http.Cookie{
		Name:     m.config.CookieName,
		Value:    "",
		Path:     "/",
		MaxAge:   -1,
		Expires:  time.Unix(0, 0),
		HttpOnly: true,
		Secure:   m.config.CookieSecure,
		SameSite: m.config.CookieSameSite,
	}
	http.SetCookie(w, cookie)
	return nil
}

// Middleware وسيط HTTP لتحميل وفحص الجلسة مع كبح كتابة وقت النشاط
func (m *Manager) Middleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		rawToken := m.extractCookie(r)
		if rawToken == "" {
			next.ServeHTTP(w, r)
			return
		}

		tokenHash := m.hashToken(rawToken)
		data, err := m.store.Get(r.Context(), tokenHash)
		if err != nil {
			next.ServeHTTP(w, r)
			return
		}

		now := time.Now().UTC()

		// التحقق من الانتهاء المطلق
		if now.After(data.AbsoluteExp) {
			_ = m.store.Delete(r.Context(), tokenHash)
			m.DestroySession(r.Context(), w, r)
			next.ServeHTTP(w, r)
			return
		}

		// تطبيق كبح الكتابة لتحديث وقت النشاط
		if now.Sub(data.LastActiveAt) >= m.config.ThrottleInterval {
			data.LastActiveAt = now
			ttl := time.Until(data.AbsoluteExp)
			if ttl > m.config.IdleTimeout {
				ttl = m.config.IdleTimeout
			}
			_ = m.store.Set(r.Context(), tokenHash, data, ttl)
		}

		ctx := context.WithValue(r.Context(), sessionContextKey, data)
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

// FromContext استخراج الجلسة من السياق
func FromContext(ctx context.Context) (*SessionData, bool) {
	data, ok := ctx.Value(sessionContextKey).(*SessionData)
	return data, ok && data != nil
}

func (m *Manager) generateTokens() (string, string, error) {
	b := make([]byte, 32)
	if _, err := io.ReadFull(rand.Reader, b); err != nil {
		return "", "", err
	}
	raw := hex.EncodeToString(b)
	return raw, m.hashToken(raw), nil
}

func (m *Manager) hashToken(raw string) string {
	h := sha256.Sum256([]byte(raw))
	return hex.EncodeToString(h[:])
}

func (m *Manager) extractCookie(r *http.Request) string {
	c, err := r.Cookie(m.config.CookieName)
	if err != nil {
		return ""
	}
	return c.Value
}

func (m *Manager) writeCookie(w http.ResponseWriter, rawToken string, ttl time.Duration) {
	cookie := &http.Cookie{
		Name:     m.config.CookieName,
		Value:    rawToken,
		Path:     "/",
		MaxAge:   int(ttl.Seconds()),
		HttpOnly: true,
		Secure:   m.config.CookieSecure,
		SameSite: m.config.CookieSameSite,
	}
	http.SetCookie(w, cookie)
}
