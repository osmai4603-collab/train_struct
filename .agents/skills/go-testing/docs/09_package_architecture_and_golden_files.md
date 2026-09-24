# 09. معمارية الحزم والملفات الذهبية (Package Architecture & Golden Files)

## 1. الفصل المعماري: الصندوق الأبيض مقابل الصندوق الأسود

في المشاريع الكبيرة، يحدد نمط تسمية حزمة الاختبار كيفية تفاعل الاختبار مع الكود المصدري:

```text
┌────────────────────────────────────────────────────────────────────────┐
│                   White-Box Tests مقابل Black-Box Tests                │
├──────────────────────────┬─────────────────────────────────────────────┤
│ White-Box (داخل الحزمة)  │ Black-Box (خارج الحزمة - المعيار المفضل)    │
├──────────────────────────┼─────────────────────────────────────────────┤
│ • `package auth`         │ • `package auth_test`                       │
│ • يمكنه الوصول للمتغيرات │ • يتعامل مع الحزمة كمستهلك خارجي عبر الـ    │
│   والحقول الخاصة مباشرة. │   Exported API فقط.                         │
│ • قد يسبب دورات استيراد  │ • يستحيل أن يقع في فخ دورات الاستيراد       │
│   (Circular Imports).    │   (Circular Imports).                       │
│ • يُستخدم فقط للخوارزميات│ • يوثق طريقة الاستخدام الحقيقية للحزمة      │
│   الرياضية والحسابية     │   (Executable Documentation).               │
│   المعقدة داخل الحزمة.   │                                             │
└──────────────────────────┴─────────────────────────────────────────────┘
```

> [!TIP]
> **قاعدة Google:** اكتب اختباراتك دائماً في حزمة مستقلة مسبوقة بـ `_test` (مثل `package user_test`) للتأكد من أن الـ API العام كافٍ وواضح ولا يتطلب أسراراً داخلية لاستخدامه.

---

## 2. نمط `export_test.go` السحري (The Export Pattern)

إذا كنت تستخدم `package foo_test` ولكنك بحاجة ماسة لضبط متغير داخلي أو مراقبة حالة غير مصدرة أثناء الاختبار فقط:

1. أنشئ ملفاً باسم `export_test.go` داخل الحزمة الأصلية (`package foo`).
2. هذا الملف يتم تجاهله بالكامل أثناء بناء الإنتاج `go build`.
3. يُستخدم حصرياً في بيئة الاختبار لتصدير ما يلزم:

```go
// internal/ratelimit/export_test.go
package ratelimit

// تصدير متغيرات أو دوال خاصة لأغراض الاختبار فقط
var (
 ExportedResetGlobalBuckets = resetGlobalBuckets
 ExportedCurrentTokens      = (*Limiter).currentTokens
)
```

---

## 3. مجلد البيانات المرجعية `testdata/` والملفات الذهبية (Golden Files)

مجلد `testdata` له معاملة استثنائية في مترجم Go:

- تتجاهله أدوات البناء تماماً ولا يتم تضمينه في ملف الإنتاج الثنائي.
- هو المكان المثالي لحفظ ملفات الإعدادات، واستجابات الـ API المتوقعة، وشهادات الأمان.

### نمط الملفات الذهبية وتحديثها التلقائي (`-update` flag)

عند اختبار مخرجات نصية معقدة (مثل تقرير PDF، أو استجابة JSON ضخمة، أو شفرة HTML)، لا تضع النصوص في كود Go مباشرة، بل احفظها في ملف `testdata/report.golden`:

```go
package report_test

import (
 "flag"
 "os"
 "path/filepath"
 "testing"

 "github.com/google/go-cmp/cmp"
)

var update = flag.Bool("update", false, "update golden test files")

func TestGenerateReport(t *testing.T) {
 got := GenerateSystemReport()
 goldenFile := filepath.Join("testdata", "system_report.golden")

 if *update {
  if err := os.WriteFile(goldenFile, []byte(got), 0644); err != nil {
   t.Fatalf("failed to update golden file: %v", err)
  }
 }

 want, err := os.ReadFile(goldenFile)
 if err != nil {
  t.Fatalf("failed to read golden file: %v (run with -update to generate)", err)
 }

 if diff := cmp.Diff(string(want), got); diff != "" {
  t.Errorf("GenerateSystemReport() mismatch against golden file (-want +got):\n%s", diff)
 }
}
```

- تشغيل وتحديث المخرجات عند تعديل التصميم:

  ```bash
  go test -update ./...
  ```
