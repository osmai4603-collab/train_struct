package examples_test

import (
	"context"
	"errors"
	"testing"
	"time"
)

// User يمثل هيكل المستخدم الأساسي
type User struct {
	ID    string
	Email string
	Age   int
	Role  string
}

// Custom errors
var (
	ErrEmptyEmail  = errors.New("email cannot be empty")
	ErrUnderage    = errors.New("user must be at least 18 years old")
	ErrInvalidRole = errors.New("role is not permitted")
)

// ValidateUser يطبق منطق الأعمال المطلوب فحصه
func ValidateUser(u User) error {
	if u.Email == "" {
		return ErrEmptyEmail
	}
	if u.Age < 18 {
		return ErrUnderage
	}
	switch u.Role {
	case "admin", "member", "guest":
		return nil
	default:
		return ErrInvalidRole
	}
}

// TestValidateUser يوضح نمط Table-Driven Test المتكامل مع كافة الممارسات العالمية
func TestValidateUser(t *testing.T) {
	t.Parallel() // تفعيل الموازاة للمجموعة الرئيسية

	tests := []struct {
		name    string
		input   User
		wantErr error
	}{
		{
			name: "valid adult member",
			input: User{
				ID:    "usr_01",
				Email: "ahmed@example.com",
				Age:   28,
				Role:  "member",
			},
			wantErr: nil,
		},
		{
			name: "valid admin user",
			input: User{
				ID:    "usr_02",
				Email: "sarah@example.com",
				Age:   35,
				Role:  "admin",
			},
			wantErr: nil,
		},
		{
			name: "missing email returns specific error",
			input: User{
				ID:    "usr_03",
				Email: "",
				Age:   22,
				Role:  "member",
			},
			wantErr: ErrEmptyEmail,
		},
		{
			name: "underage age returns specific error",
			input: User{
				ID:    "usr_04",
				Email: "khalid@example.com",
				Age:   17,
				Role:  "guest",
			},
			wantErr: ErrUnderage,
		},
		{
			name: "unauthorized role returns error",
			input: User{
				ID:    "usr_05",
				Email: "attacker@example.com",
				Age:   30,
				Role:  "super_root",
			},
			wantErr: ErrInvalidRole,
		},
	}

	for _, tc := range tests {
		tc := tc // حماية مسبقة لنطاق المتغير في Go
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel() // تفعيل الموازاة لكل حالة فرعية

			// استخدام سياق الاختبار في Go 1.24+
			ctx, cancel := context.WithTimeout(t.Context(), 500*time.Millisecond)
			defer cancel()
			_ = ctx

			// محاكاة حجز مورد وتنظيفه عبر t.Cleanup
			cleanupDone := false
			t.Cleanup(func() {
				cleanupDone = true
				_ = cleanupDone
			})

			err := ValidateUser(tc.input)

			// التحقق بصيغة Got before Want
			if tc.wantErr != nil {
				if err == nil {
					t.Fatalf("ValidateUser(%+v) = nil, want error %v", tc.input, tc.wantErr)
				}
				if !errors.Is(err, tc.wantErr) {
					t.Errorf("ValidateUser(%+v) error = %v, want error %v", tc.input, err, tc.wantErr)
				}
				return
			}

			if err != nil {
				t.Fatalf("ValidateUser(%+v) unexpected error: %v", tc.input, err)
			}
		})
	}
}

// دالة مساعدة مع t.Helper
func assertUserRole(t *testing.T, u User, expectedRole string) {
	t.Helper()
	if u.Role != expectedRole {
		t.Fatalf("User role = %q, want %q", u.Role, expectedRole)
	}
}
