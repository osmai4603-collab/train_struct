package examples_test

import (
	"errors"
	"testing"
	"testing/synctest"
	"time"
)

// PollUntilSuccess تحاكي عملية استطلاع متكررة تعتمد على فترات انتظار
func PollUntilSuccess(fn func() (bool, error), interval time.Duration, maxAttempts int) error {
	for i := 0; i < maxAttempts; i++ {
		done, err := fn()
		if err != nil {
			return err
		}
		if done {
			return nil
		}
		time.Sleep(interval) // انتظار طويل
	}
	return errors.New("poll timeout exceeded")
}

// TestPollUntilSuccess_WithSynctest يوضح قوة ميزة Go 1.24+ في اختبار فترات الانتظار الطويلة فورياً
func TestPollUntilSuccess_WithSynctest(t *testing.T) {
	// يتم تشغيل الاختبار داخل فقاعة الوقت الافتراضي لـ synctest
	synctest.Run(func() {
		attempts := 0
		pollInterval := 15 * time.Minute // 15 دقيقة انتظار بين كل محاولة!

		err := PollUntilSuccess(func() (bool, error) {
			attempts++
			if attempts == 3 {
				return true, nil // تنجح المحاولة الثالثة
			}
			return false, nil
		}, pollInterval, 5)

		// هذا الاختبار سينتهي في العالم الواقعي خلال 1 ميلي ثانية بدلاً من 30 دقيقة!
		if err != nil {
			t.Fatalf("expected polling to succeed, got: %v", err)
		}

		if attempts != 3 {
			t.Errorf("attempts = %d, want 3", attempts)
		}
	})
}
