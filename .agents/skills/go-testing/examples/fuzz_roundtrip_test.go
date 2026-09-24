package examples_test

import (
	"bytes"
	"encoding/base64"
	"testing"
)

// FuzzBase64RoundTrip يوضح نموذج الاختبار العشوائي المدمج في Go
func FuzzBase64RoundTrip(f *testing.F) {
	// 1. تزويد البذور الأولية (Seed Corpus)
	f.Add([]byte("simple ASCII text"))
	f.Add([]byte(""))
	f.Add([]byte("مرحبا بالعالم - Arabic Unicode"))
	f.Add([]byte("\x00\x01\x02\xff\xfe\xfd"))
	f.Add(bytes.Repeat([]byte("A"), 512))

	// 2. تشغيل حلقة الـ Fuzzing
	f.Fuzz(func(t *testing.T, original []byte) {
		// التشفير
		encodedStr := base64.StdEncoding.EncodeToString(original)

		// فك التشفير
		decodedBytes, err := base64.StdEncoding.DecodeString(encodedStr)
		if err != nil {
			t.Fatalf("failed to decode valid base64 string: %v", err)
		}

		// التحقق من مطابقة النتيجة بعد دورة كاملة (Round-trip equality)
		if !bytes.Equal(original, decodedBytes) {
			t.Fatalf("roundtrip mismatch: got %v, want %v", decodedBytes, original)
		}
	})
}
