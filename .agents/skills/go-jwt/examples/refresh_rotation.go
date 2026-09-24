package examples

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"time"
)

var (
	ErrTokenReuseDetected = errors.New("breach alert: refresh token reuse detected; entire token family revoked")
	ErrSessionExpired     = errors.New("refresh token is expired or revoked")
)

// DefaultGracePeriod مهلة السماح لحل مشكلة السباق الشبكي وتكرار الطلبات المتزامنة
const DefaultGracePeriod = 20 * time.Second

// SessionRecord سجل الجلسة ورمز التحديث في قاعدة البيانات
type SessionRecord struct {
	ID         string     // المعرف العشوائي لرمز التحديث
	FamilyID   string     // معرف عائلة الجلسة (يربط الرموز المتبادلة)
	UserID     string     // معرف المستخدم
	IsUsed     bool       // هل تم استهلاك هذا الرمز مسبقاً؟
	UsedAt     *time.Time // وقت الاستهلاك الأولي (لحساب مهلة السماح)
	ReplacedBy string     // معرف الرمز الجديد الذي حل محله أثناء التدوير
	ExpiresAt  time.Time  // موعد الانتهاء النهائي للجلسة
}

// SessionRepository واجهة التفاعل مع قاعدة البيانات لتتبع الجلسات
type SessionRepository interface {
	Get(ctx context.Context, tokenID string) (*SessionRecord, error)
	MarkAsUsed(ctx context.Context, tokenID string, usedAt time.Time, replacedByID string) error
	RevokeFamily(ctx context.Context, familyID string) error
	Create(ctx context.Context, record *SessionRecord) error
}

// TokenRotationEngine محرك تدوير رموز التحديث مع كاشف الاختراق ومهلة السماح الشبكي
type TokenRotationEngine struct {
	repo        SessionRepository
	gracePeriod time.Duration
}

func NewTokenRotationEngine(repo SessionRepository, gracePeriod time.Duration) *TokenRotationEngine {
	if gracePeriod <= 0 {
		gracePeriod = DefaultGracePeriod
	}
	return &TokenRotationEngine{
		repo:        repo,
		gracePeriod: gracePeriod,
	}
}

// RotateToken يستبدل رمز التحديث برمز جديد ويتحقق من عدم حدوث سرقة مع دعم مهلة السماح
func (e *TokenRotationEngine) RotateToken(ctx context.Context, presentedTokenID string) (*SessionRecord, error) {
	record, err := e.repo.Get(ctx, presentedTokenID)
	if err != nil {
		return nil, fmt.Errorf("session lookup error: %w", err)
	}

	now := time.Now().UTC()

	// 1. التحقق من انتهاء الصلاحية
	if now.After(record.ExpiresAt) {
		return nil, ErrSessionExpired
	}

	// 2. فحص كاشف الاختراق ومراعاة مهلة السماح الشبكي (Race Condition Grace Period)
	if record.IsUsed {
		// هل أُعيد إرسال الرمز خلال مهلة السماح المحددة (مثلاً 20 ثانية) بسبب بطء الشبكة؟
		if record.UsedAt != nil && now.Sub(*record.UsedAt) <= e.gracePeriod && record.ReplacedBy != "" {
			// استرجاع الرمز البديل الذي تم إنشاؤه بالفعل وإعادته بأمان دون إلغاء الجلسة
			replacementRecord, err := e.repo.Get(ctx, record.ReplacedBy)
			if err == nil && !now.After(replacementRecord.ExpiresAt) {
				return replacementRecord, nil
			}
		}

		// إذا تجاوز مهلة السماح: هذا اختراق مؤكد لسرقة الرمز القديم من مهاجم!
		_ = e.repo.RevokeFamily(ctx, record.FamilyID)
		return nil, ErrTokenReuseDetected
	}

	// 3. توليد رمز جديد ينتمي لنفس عائلة الجلسة
	newRecord := &SessionRecord{
		ID:        generateCryptoID(32),
		FamilyID:  record.FamilyID,
		UserID:    record.UserID,
		IsUsed:    false,
		ExpiresAt: now.Add(7 * 24 * time.Hour), // تمديد منزلق بحد أقصى 7 أيام
	}

	if err := e.repo.Create(ctx, newRecord); err != nil {
		return nil, fmt.Errorf("failed to persist new session: %w", err)
	}

	// 4. تعليم الرمز الحالي كمستهلك وتخزين وقت الاستهلاك والرمز البديل
	if err := e.repo.MarkAsUsed(ctx, record.ID, now, newRecord.ID); err != nil {
		return nil, fmt.Errorf("failed to mark token as used: %w", err)
	}

	return newRecord, nil
}

func generateCryptoID(n int) string {
	b := make([]byte, n)
	_, _ = rand.Read(b)
	return hex.EncodeToString(b)
}
