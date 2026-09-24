# 08. الاختبار العشوائي واختبار الخصائص (Fuzzing & Property-Based Testing)

## 1. الاختبار العشوائي الأصيل المدمج في Go (`testing.F`)

بدءاً من Go 1.18، أصبح الاختبار العشوائي (Fuzzing) مدمجاً مباشرة في اللغة ومحرك البناء. ميزة الـ Fuzzing في Go أنها موجهة بالتغطية البرمجية (**Coverage-Guided**)، حيث يقوم المحرك بتعديل المدخلات لاكتشاف مسارات كود برمجية جديدة لم تُختبر بعد.

### 1.1 متى يجب استخدام الـ Fuzzing إلزامياً؟

- مفككات التشفير والترميز (Decoders/Encoders مثل JSON, Protobuf, Hex, Base64).
- المحللات اللغوية والـ Parsers لمعالجة التعبيرات أو الترويسات (HTTP Headers, URLs, SQL).
- أنظمة التحقق من المدخلات الصارمة ومعالجة نصوص Unicode واللغات المختلفة.

### 1.2 نموذج Fuzzing قياسي

```go
package parser_test

import (
 "bytes"
 "testing"
)

func FuzzParseHeader(f *testing.F) {
 // 1. إضافة بذور أولية معروفة (Seed Corpus)
 f.Add([]byte("Authorization: Bearer token123"))
 f.Add([]byte("Content-Type: application/json; charset=utf-8"))
 f.Add([]byte(""))
 f.Add([]byte("InvalidHeaderWithoutColon"))
 f.Add([]byte("\x00\xff\xfe"))

 // 2. تشغيل حلقة الاختبار العشوائي
 f.Fuzz(func(t *testing.T, data []byte) {
  // يجب ألا يحدث Panic تحت أي ظرف مع أي مدخل مهما كان مشوهاً
  header, err := ParseHeader(data)
  if err != nil {
   // الأخطاء متوقعة ومقبولة للمدخلات المشوهة
   return
  }

  // إذا نجح التحليل، يجب أن ينتج كائناً سليماً يمكن إعادة تشفيره
  serialized := header.Bytes()
  if len(serialized) == 0 {
   t.Errorf("successful parse resulted in empty serialization")
  }
 })
}
```

تشغيل الاختبار في بيئة التطوير أو الـ CI:

```bash
go test -fuzz=FuzzParseHeader -fuzztime=30s ./...
```

---

## 2. الاختبار القائم على الخصائص (Property-Based Testing عبر `rapid`)

بينما يركز الـ Fuzzing على مصفوفات الـ bytes العشوائية، فإن **Property-Based Testing** يفحص القوانين الرياضية الثابتة للكود (Invariants) عبر توليد هياكل بيانات مخصصة ومعقدة.

المكتبة الأحدث والأقوى في Go هي `pgregory.net/rapid`:

```go
package mathutil_test

import (
 "testing"
 "pgregory.net/rapid"
)

// دالة الضغط وفك الضغط
func Compress(in []byte) []byte { /* ... */ }
func Decompress(in []byte) ([]byte, error) { /* ... */ }

func TestCompressionRoundtrip_Property(t *testing.T) {
 rapid.Check(t, func(t *rapid.T) {
  // توليد مصفوفة عشوائية بذكاء
  original := rapid.SliceOf(rapid.Byte()).Draw(t, "original")

  compressed := Compress(original)
  decompressed, err := Decompress(compressed)

  if err != nil {
   t.Fatalf("Decompress returned unexpected error: %v", err)
  }

  // خاصية حتمية: فك الضغط يجب أن يعيد نفس المدخل الأصلي تماماً
  if string(decompressed) != string(original) {
   t.Fatalf("roundtrip property violated!")
  }
 })
}
```

**الميزة الكبرى:** عند حدوث خطأ، تقوم `rapid` بعملية تصغير تلقائي (**Shrinking**) للعثور على أصغر مدخل ممكن يعيد إنتاج المشكلة، مما يسهل إصلاحه فوراً.
