package examples

import "log/slog"

// SensitiveToken نوع يغلف الرمز ويمنع تسريبه عند كتابته في السجلات الموزعة
type SensitiveToken string

// LogValue يطبق واجهة slog.LogValuer لإخفاء الرمز تلقائياً في السجلات المهيكلة
func (t SensitiveToken) LogValue() slog.Value {
	if len(t) == 0 {
		return slog.StringValue("<empty>")
	}

	// إظهار بادئة قصيرة لغايات التتبع وإخفاء باقي الرمز الحساس
	raw := string(t)
	prefixLen := 6
	if len(raw) < prefixLen {
		prefixLen = len(raw)
	}

	return slog.StringValue(raw[:prefixLen] + "...[REDACTED]")
}

// String يحمي من الطباعة الصريحة بـ fmt.Println أو fmt.Sprintf
func (t SensitiveToken) String() string {
	return "[REDACTED_TOKEN]"
}
