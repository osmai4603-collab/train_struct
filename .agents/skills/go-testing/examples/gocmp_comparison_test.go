package examples_test

import (
	"testing"
	"time"

	"github.com/google/go-cmp/cmp"
	"github.com/google/go-cmp/cmp/cmpopts"
)

// SessionData تمثل بيانات جلسة مستخدم معقدة
type SessionData struct {
	SessionID string
	UserID    string
	Roles     []string
	CreatedAt time.Time
	ExpiresAt time.Time
	metadata  map[string]string // حقل غير مصدّر
}

func CreateUserSession(userID string, roles []string) *SessionData {
	now := time.Now()
	return &SessionData{
		SessionID: "sess_random_123456",
		UserID:    userID,
		Roles:     roles,
		CreatedAt: now,
		ExpiresAt: now.Add(24 * time.Hour),
		metadata:  map[string]string{"ip": "192.168.1.1", "device": "mobile"},
	}
}

func TestCreateUserSession_WithGoCmp(t *testing.T) {
	t.Parallel()

	got := CreateUserSession("usr_99", []string{"editor", "viewer"})

	want := &SessionData{
		SessionID: "", // نتجاهله في المقارنة لأن قيمته عشوائية
		UserID:    "usr_99",
		Roles:     []string{"viewer", "editor"}, // الترتيب مختلف عمداً
		CreatedAt: time.Time{},                  // نتجاهله
		ExpiresAt: time.Time{},                  // نتجاهله
		metadata:  map[string]string{"ip": "192.168.1.1", "device": "mobile"},
	}

	// تكوين خيارات المقارنة الذكية وفق معايير Google
	cmpOptions := cmp.Options{
		// 1. تجاهل المعرفات العشوائية وحقول الوقت المتغيرة
		cmpopts.IgnoreFields(SessionData{}, "SessionID", "CreatedAt", "ExpiresAt"),

		// 2. السماح بمقارنة الحقول الخاصة غير المصدرة
		cmp.AllowUnexported(SessionData{}),

		// 3. ترتيب عناصر الشريحة لتفادي الفشل بسبب اختلاف الترتيب العشوائي
		cmpopts.SortSlices(func(a, b string) bool {
			return a < b
		}),

		// 4. مساواة الـ nil والـ empty map/slice
		cmpopts.EquateEmpty(),
	}

	// حساب الفروقات بدقة: (-want +got)
	if diff := cmp.Diff(want, got, cmpOptions...); diff != "" {
		t.Errorf("CreateUserSession() mismatch (-want +got):\n%s", diff)
	}
}
