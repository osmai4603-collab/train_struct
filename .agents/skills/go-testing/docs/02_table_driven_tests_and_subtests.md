# 02. الاختبارات الموجهة بالجداول والاختبارات الفرعية (Table-Driven Tests & Subtests)

## 1. تشريح جدول الاختبار المعياري (Anatomy of Table-Driven Tests)

يعتبر نمط **Table-Driven Tests** الهيكل الأساسي المعتمد في مكتبة Go القياسية وكبرى الشركات (Google, Uber) لتنظيم حالات الاختبار.

### الهيكل القياسي الموصى به

```go
package validator_test

import (
 "context"
 "errors"
 "testing"
)

func TestParsePort(t *testing.T) {
 t.Parallel() // 1. تفعيل الموازاة للمجموعة الأب

 // 2. تعريف شريحة مجهولة تمثل الجدول
 tests := []struct {
  name    string // اسم الحالة (توضيحي ومحدد)
  input   string // المدخلات
  want    int    // النتيجة المتوقعة
  wantErr bool   // هل نتوقع خطأ؟
 }{
  {
   name:    "valid standard port",
   input:   "8080",
   want:    8080,
   wantErr: false,
  },
  {
   name:    "valid boundary port min",
   input:   "1",
   want:    1,
   wantErr: false,
  },
  {
   name:    "valid boundary port max",
   input:   "65535",
   want:    65535,
   wantErr: false,
  },
  {
   name:    "invalid port out of range",
   input:   "70000",
   want:    0,
   wantErr: true,
  },
  {
   name:    "invalid non-numeric port",
   input:   "http",
   want:    0,
   wantErr: true,
  },
 }

 for _, tc := range tests {
  tc := tc // حماية مسبقة لنطاق المتغير في الحلقات
  t.Run(tc.name, func(t *testing.T) {
   t.Parallel() // 3. تفعيل الموازاة لكل حالة فرعية

   got, err := ParsePort(tc.input)

   // التحقق من حالة الخطأ
   if (err != nil) != tc.wantErr {
    t.Fatalf("ParsePort(%q) error = %v, wantErr %v", tc.input, err, tc.wantErr)
   }

   // التحقق من القيمة المعادة في حال عدم وجود خطأ
   if !tc.wantErr && got != tc.want {
    t.Errorf("ParsePort(%q) = %d, want %d", tc.input, got, tc.want)
   }
  })
 }
}
```

---

## 2. القواعد الصارمة لإدارة دورة الحياة والموارد

### 2.1 إلزامية `t.Cleanup` وحظر `defer` داخل الاختبارات الفرعية

في لغة Go، عبارة `defer` مرتبطة بدورة حياة الدالة الحاضنة (`enclosing function`). عند استخدام `defer` داخل حلقة اختبار فرعي `t.Run`:

1. **لا ينفذ التنظيف فور انتهاء الحالة الفرعية!** بل ينتظر حتى تنتهي كل الحالات وتخرج دالة الاختبار الرئيسية.
2. عند تشغيل الحالات بالتوازي عبر `t.Parallel()`، سيتأخر تنظيف الموارد مما يؤدي لاستنزاف الذاكرة، أو تضارب المنافذ، أو إغلاق موارد بينما حالات أخرى ما زالت تستخدمها.

```go
// ❌ نمط خاطئ: defer يتأخر حتى خروج الدالة الكبرى
t.Run(tc.name, func(t *testing.T) {
    res := acquireResource()
    defer res.Release() // لن ينفذ عند انتهاء هذا الاختبار الفرعي!
})

//  النمط المعياري الصحيح: ينفذ التنظيف فور خروج الاختبار الفرعي
t.Run(tc.name, func(t *testing.T) {
    res := acquireResource()
    t.Cleanup(func() {
        res.Release() // مضمون التنفيذ فور انتهاء tc.name فقط
    })
})
```

### 2.2 دوال المساعدة (`t.Helper()`)

عند بناء دوال تحقق مشتركة، يجب استدعاء `t.Helper()` كأول سطر في الدالة. هذا يوجه محرك Go لتجاهل إطار المكدس (Stack Frame) الحالي ونسبة الخطأ إلى السطر الذي استدعى الدالة داخل الاختبار:

```go
func assertPositive(t *testing.T, val int) {
    t.Helper() // إلزامي!
    if val <= 0 {
        t.Fatalf("expected positive value, got %d", val)
    }
}
```

### 2.3 سياق الاختبار التلقائي (`t.Context()` في Go 1.24+)

في بيئات Go 1.24+، يُحظر استخدام `context.Background()` أو `context.TODO()` في الاختبارات التي تطلق Goroutines أو استعلامات شبكية:

```go
func TestAsyncService(t *testing.T) {
    // سياق ملغى تلقائياً فور انتهاء TestAsyncService
    ctx := t.Context()

    err := service.ExecuteBackgroundJob(ctx)
    if err != nil {
        t.Fatalf("job failed: %v", err)
    }
}
```

### 2.4 تغيير المجلدات الآمن (`t.Chdir()` في Go 1.24+)

تغيير مجلد العمل الحالي عبر `os.Chdir()` يلوث بيئة العملية ككل ويتعارض مع `t.Parallel()`. أتاحت Go 1.24 التابع `t.Chdir(path)` الذي يقوم بتغيير المجلد بأمان وإعادته لأصله فور انتهاء الاختبار.
