# أفضل الممارسات العالمية لاختبار البرمجيات (Testing) في مشاريع Go الكبيرة

> **تاريخ البحث والتوثيق:** 2026-09-24  
> **المصادر الرسمية والمعايير المعتمدة:**  
>
> - **المراجع الرسمية للغة Go:**
>   - [pkg.go.dev/testing](https://pkg.go.dev/testing) — التوثيق الرسمي لحزمة الاختبار القياسية (`T`, `B`, `F`, `PB`)
>   - [testing/synctest](https://pkg.go.dev/testing/synctest) — حزمة Go 1.24+ الثورية لاختبار التزامن والوقت الافتراضي (`synctest.Run`)
>   - [testing/quick](https://pkg.go.dev/testing/quick) — الحزمة القياسية للاختبارات القائمة على الخصائص (Property-Based Testing)
>   - [net/http/httptest](https://pkg.go.dev/net/http/httptest) — أدوات محاكاة خوادم وعملاء HTTP القياسية في Go
>   - [Go Official Blog: Subtests and Sub-benchmarks](https://go.dev/blog/subtests) — معايير تقسيم الاختبارات والموازاة (Marcel van Lohuizen)
>   - [Go Official Security: Fuzzing](https://go.dev/security/fuzz) & [Fuzzing Tutorial](https://go.dev/doc/tutorial/fuzz) — الاختبار العشوائي الأصيل (Katie Hockman)
>   - [Go Wiki: Table-Driven Tests](https://go.dev/wiki/TableDrivenTests) — النمط المعياري المعتمد للاختبارات في Go
>   - [Go Blog: Introducing the Go Race Detector](https://go.dev/blog/race-detector) — كاشف سباق البيانات المعياري
> - **الأدلة الهندسية لكبرى الشركات العالمية:**
>   - [Google Go Style Guide: Testing Best Practices & Decisions](https://google.github.io/styleguide/go/best-practices#test-doubles) — معايير Google للاختبارات الصارمة وبدائل الاختبار
>   - [Google Software Engineering at Google (SWE Book)](https://abseil.io/resources/swe-book/html/ch13.html) — الفصول 11 إلى 14 (Unit Testing & Test Doubles)
>   - [Uber Go Style Guide](https://github.com/uber-go/guide/blob/master/style.md#testing) — قواعد الجداول، عزل الاختبارات، ومكافحة التعقيد
> - **المكتبات والأدوات القياسية المعتمدة في بيئات الإنتاج:**
>   - [google/go-cmp](https://github.com/google/go-cmp) — أداة Google القياسية للمقارنة العميقة وحساب الفروقات النصية (`cmp.Diff`)
>   - [testcontainers/testcontainers-go](https://github.com/testcontainers/testcontainers-go) — المعيار الصناعي لاختبارات التكامل باستخدام حاويات Docker المؤقتة
>   - [uber-go/goleak](https://github.com/uber-go/goleak) — كاشف تسريب الـ Goroutines في الاختبارات
>   - [pgregory.net/rapid](https://github.com/flyingmutant/rapid) — مكتبة فحص الخصائص المتقدمة (State-Machine Fuzzing & Shrinking)
>   - [stretchr/testify](https://github.com/stretchr/testify) — مكتبة الـ Assertions والمحاكاة الأكثر انتشاراً ومحددات استخدامها

---

## جدول المحتويات

1. [الفلسفة المعمارية للاختبار في Go وهرم الاختبارات (Architecture & Testing Philosophy)](#1-الفلسفة-المعمارية-للاختبار-في-go-وهرم-الاختبارات-architecture--testing-philosophy)
2. [الأنماط الأساسية والمعايير المعتمدة في حزمة `testing` القياسية](#2-الأنماط-الأساسية-والمعايير-المعتمدة-في-حزمة-testing-القياسية)
3. [معركة أدوات المقارنة والتحقق: `go-cmp` مقابل `testify`](#3-معركة-أدوات-المقارنة-والتحقق-go-cmp-مقابل-testify)
4. [استراتيجيات العزل وبدائل الاختبار (Test Doubles): Fakes, Stubs, Mocks, Spies](#4-استراتيجيات-العزل-وبدائل-الاختبار-test-doubles-fakes-stubs-mocks-spies)
5. [اختبارات التكامل والبيئات الحقيقية (Integration Testing & Testcontainers)](#5-اختبارات-التكامل-والبيئات-الحقيقية-integration-testing--testcontainers)
6. [اختبار التزامن وسباق البيانات (Concurrency, Race Detection & Synctest)](#6-اختبار-التزامن-وسباق-البيانات-concurrency-race-detection--synctest)
7. [اختبارات الأداء والقياس المقارن (Benchmarking, Profiling & Benchstat)](#7-اختبارات-الأداء-والقياس-المقارن-benchmarking-profiling--benchstat)
8. [الاختبار العشوائي والاختبار القائم على الخصائص (Fuzzing & Property-Based Testing)](#8-الاختبار-العشوائي-والاختبار-القائم-على-الخصائص-fuzzing--property-based-testing)
9. [هندسة المعمارية وتنظيم حزم الاختبار (Architecture & Code Organization)](#9-هندسة-المعمارية-وتنظيم-حزم-الاختبار-architecture--code-organization)
10. [مصفوفة الأنماط المضادة الشائعة في اختبارات Go (Anti-Patterns Matrix)](#10-مصفوفة-الأنماط-المضادة-الشائعة-في-اختبارات-go-anti-patterns-matrix)
11. [قائمة مراجعة الجاهزية للإنتاج وبيئات الـ CI/CD (Production Readiness Checklist)](#11-قائمة-مراجعة-الجاهزية-للإنتاج-وبيئات-الـ-cicd-production-readiness-checklist)
12. [المصادر والمراجع الرسمية والمعايير الصناعية](#12-المصادر-والمراجع-الرسمية-والمعايير-الصناعية)

---

## 1. الفلسفة المعمارية للاختبار في Go وهرم الاختبارات (Architecture & Testing Philosophy)

تتميز لغة Go بفلسفة تصميمية فريدة في منظومة الاختبارات تختلف جذرياً عن اللغات الكائنية الأخرى (مثل Java أو C# أو Ruby). في Go، **الاختبارات هي كود برمجي عادي وليست لغة نطاق خاصة (No separate DSL)**. لا توجد تعليقات توضيحية خفية (No Annotations/Decorators)، ولا يوجد انعكاس سحري (No Magic Reflection)، ولا أطر عمل ضخمة تفرض قيوداً تعسفية.

### 1.1 مبادئ فلسفة الاختبار في مجتمع Go الرسمي و Google

1. **الاختبار كود عادي (Tests are ordinary Go code):** التحكم في تدفق الاختبار يتم عبر أدوات اللغة الأساسية (`if`, `for`, `switch`).
2. **اختبار السلوك والحالة وليس تفاصيل التنفيذ (Test State & Behavior, Not Interactions):** التركيز على المدخلات والمخرجات النهائية وحالة النظام بدلاً من تتبع كل استدعاء تابع داخلي.
3. **الفشل الواضح والمباشر (Clear, Actionable Diagnostic Failures):** يجب أن تخبرك رسالة الفشل بدقة: ماذا حدث؟ ماذا كان متوقعاً؟ وأين وقع الخطأ؟ دون الحاجة لفتح المصحح (Debugger).
4. **مواصلة التنفيذ عند الفشل (Keep Going on Failure):** تفضيل `t.Errorf` على `t.Fatalf` حتى تُظهر الاختبارات جميع المشاكل في دورة تشغيل واحدة ما لم يكن الفشل مانعاً للمتابعة.

```text
┌────────────────────────────────────────────────────────────────────────┐
│               هرم الاختبارات المعياري للأنظمة الموزعة في Go            │
├────────────────────────────────────────────────────────────────────────┤
│                                  ▲                                     │
│                                 / \                                    │
│                                / E \         (نهاية إلى نهاية - بطيئة)  │
│                               / 2 E \        End-to-End Tests          │
│                              /───────\       (Smoke & Critical Flows)  │
│                             /         \                                │
│                            /Integration\     (بيئات حقيقية - Docker)   │
│                           /    Tests    \    Testcontainers & Postgres │
│                          /───────────────\                             │
│                         /                 \  (حزمة testing + go-cmp)   │
│                        /    Unit Tests     \ سريعة، موازية، ومستقلة     │
│                       /─────────────────────\ Table-Driven & Pure Logic│
└────────────────────────────────────────────────────────────────────────┘
```

### 1.2 مصفوفة مقارنة مستويات الاختبار في المشاريع الضخمة

| المعيار الهندسي | اختبارات الوحدة (Unit Tests) | اختبارات التكامل (Integration Tests) | اختبارات النهاية للنهاية (E2E Tests) |
| :--- | :--- | :--- | :--- |
| **النطاق (Scope)** | دالة، بنية (`struct`)، أو حزمة منفردة. | عدة مكونات مع قواعد بيانات حقيقية وخوادم وسيطة. | النظام ككل (HTTP, Gateway, DB, Cache, Workers). |
| **وسيط العزل (Isolation)** | بدائل الاختبار السريعة في الذاكرة (Fakes/Stubs). | حاويات Docker عابرة عبر Testcontainers. | بيئة تجريبية مطابقة للإنتاج (Staging/Ephemeral Cluster). |
| **زمن التشغيل (Speed)** | أجزاء من الألف من الثانية (Microseconds). | ثوانٍ معدودة لكل مجموعة اختبار (1-5s). | دقائق متعددة (Minutes). |
| **الاعتمادية (Determinism)** | قطعية 100% (Deterministic). | عالية جداً عند استخدام حاويات معزولة. | عرضة للتقلب والهشاشة الشبكية (Flaky prone). |
| **التكلفة والصيانة** | منخفضة جداً وسهلة التحديث. | متوسطة وتتطلب إدارة مخططات وقواعد بيانات. | باهظة وتتطلب مراقبة بيئات وبنية تحتية كاملة. |
| **وسوم البناء (Build Tags)** | تشمل الحزم الافتراضية بدون وسوم. | `//go:build integration` | `//go:build e2e` |

---

## 2. الأنماط الأساسية والمعايير المعتمدة في حزمة `testing` القياسية

توفر حزمة `testing` في مكتبة Go القياسية كل ما يلزم لبناء اختبارات فائقة القوة والأداء دون الحاجة لأي مكتبة خارجية.

### 2.1 نمط الاختبارات الموجهة بالجداول (Table-Driven Tests)

يُعتبر نمط Table-Driven Tests هو المعيار المعتمد في مكتبة Go القياسية وفي Google و Uber لتنظيم حالات الاختبار المتعددة لنفس التابع.

```go
package service_test

import (
 "context"
 "errors"
 "testing"

 "github.com/google/go-cmp/cmp"
)

type User struct {
 ID    string
 Email string
 Age   int
}

// دالة التحقق من صحة المستخدم
func ValidateUser(u User) error {
 if u.Email == "" {
  return errors.New("email is required")
 }
 if u.Age < 18 {
  return errors.New("user must be an adult")
 }
 return nil
}

func TestValidateUser(t *testing.T) {
 t.Parallel() // تفعيل التشغيل الموازي للمجموعة الرئيسية

 // جدول الحالات: شريحة مجهولة من بنية الاختبار
 tests := []struct {
  name    string
  input   User
  wantErr error
 }{
  {
   name:    "valid adult user",
   input:   User{ID: "usr_1", Email: "alex@example.com", Age: 25},
   wantErr: nil,
  },
  {
   name:    "missing email returns error",
   input:   User{ID: "usr_2", Email: "", Age: 30},
   wantErr: errors.New("email is required"),
  },
  {
   name:    "underage user returns error",
   input:   User{ID: "usr_3", Email: "teen@example.com", Age: 16},
   wantErr: errors.New("user must be an adult"),
  },
 }

 for _, tc := range tests {
  tc := tc // حماية مسبقة لنطاق المتغير (رغم تصحيحها تلقائياً في Go 1.22+)
  t.Run(tc.name, func(t *testing.T) {
   t.Parallel() // تشغيل كل حالة فرعية بالتوازي!

   err := ValidateUser(tc.input)

   // التحقق من الخطأ بصياغة Got before Want
   if tc.wantErr != nil {
    if err == nil {
     t.Fatalf("ValidateUser(%+v) = nil, want error %q", tc.input, tc.wantErr)
    }
    if err.Error() != tc.wantErr.Error() {
     t.Errorf("ValidateUser(%+v) error = %q, want %q", tc.input, err.Error(), tc.wantErr.Error())
    }
    return
   }

   if err != nil {
    t.Fatalf("ValidateUser(%+v) unexpected error: %v", tc.input, err)
   }
  })
 }
}
```

### 2.2 القواعد الذهبية لـ Table-Driven Tests (وفق Uber و Google)

1. **عزل كل حالة تماماً:** لا تجعل حالة اختبار تعتمد على مخرجات حالة سابقة في الجدول.
2. **الامتناع عن تعقيد الجدول بالمنطق الشرطي (Avoid Test Logic in Tables):** إذا كانت بعض الحالات تتطلب استدعاء توابع مختلفة كلياً أو إعدادات شديدة التباين، افصلها في دالة `Test...` مستقلة بدلاً من ملء الجدول بدوال مجهولة وحقول اختيارية.
3. **تسميات واضحة ومقروءة:** يجب أن يكون حقل `name` يعبر عن السيناريو والنتيجة المتوقعة.

### 2.3 إدارة الموارد والتنظيف (`t.Cleanup` مقابل `defer`)

في اختبارات Go الفرعية (`t.Run`)، استخدام `defer` قد يؤدي إلى كوارث غير متوقعة لأن `defer` ينفذ عند خروج الدالة الحاضنة وليس عند انتهاء الاختبار الفرعي، كما أنه يتعارض تماماً مع `t.Parallel()`.

> [!IMPORTANT]
> **القاعدة الصارمة:** استخدم دائماً `t.Cleanup()` بدلاً من `defer` داخل دوال المساعدة والاختبارات الفرعية.

```go
func setupTestDatabase(t *testing.T) *Database {
 t.Helper() // إعلام المترجم بأن هذه دالة مساعدة لتصحيح رقم السطر عند الفشل

 db := connectEphemeralDB()
 
 // تسجيل عملية التنظيف
 t.Cleanup(func() {
  if err := db.Close(); err != nil {
   t.Logf("failed to close database: %v", err)
  }
 })

 return db
}
```

### 2.4 دوال المساعدة وتتبع الأخطاء بدقة (`t.Helper()`)

عندما تفشل عملية فحص داخل دالة مساعدة، فإن عدم وضع `t.Helper()` يجعل سجل الأخطاء يشير إلى السطر داخل الدالة المساعدة بدلاً من السطر الفعلي داخل الاختبار الذي قام باستدعائها:

```go
func assertNoError(t *testing.T, err error) {
 t.Helper() // يوجه مؤشر تتبع الخطأ (Stack Trace) للسطر المستدعي في TestMain
 if err != nil {
  t.Fatalf("unexpected error received: %v", err)
 }
}
```

### 2.5 إدارة الأدلة المؤقتة والمسارات (`t.TempDir` و `t.Chdir` في Go 1.24+)

- **`t.TempDir()`:** تنشئ مجلداً مؤقتاً فريداً لكل اختبار، وتقوم بحذفه تلقائياً عند انتهاء الاختبار دون الحاجة لكتابة كود حذف يدوي.
- **`t.Chdir(dir)` (ميزة Go 1.24+ الرسمية):** تُغير مجلد العمل الحالي أثناء تنفيذ الاختبار، وتعيده تلقائياً إلى المجلد الأصلي بمجرد انتهاء الاختبار، متفادية الأنماط المضادة لتغيير `os.Chdir` يدوياً.

### 2.6 السياق الإلغائي المرتبط بدورة حياة الاختبار (`t.Context()` في Go 1.24+)

أضافت Go 1.24 التابع `t.Context()` (وكذلك `b.Context()` و `f.Context()`) لتوفير `context.Context` ملغى تلقائياً بمجرد انتهاء الاختبار، مما يمنع تسريب الـ Goroutines المستندة إلى الوقت أو الشبكة:

```go
func TestWorkerProcess(t *testing.T) {
 // هذا السياق يلغى فور انتهاء TestWorkerProcess تلقائياً!
 ctx := t.Context()

 worker := NewWorker()
 err := worker.Start(ctx)
 if err != nil {
  t.Fatalf("worker failed: %v", err)
 }
}
```

### 2.7 صياغة رسائل الفشل النموذجية: قاعدة "Got before Want"

تفرض معايير Google الرسمية في Go صياغة موحدة لرسائل الفشل لتوحيد قراءة السجلات عبر آلاف المهندسين:

```text
// النمط الخاطئ والشائع في اللغات الأخرى:
"Expected 200, but got 404"
"Assertion failed: want != got"

// النمط المعياري القياسي في Go:
FunctionName(args) = got, want want
"GetUser(123) = nil, want valid User"
"CalculateDiscount(100) = 15, want 20"
```

---

## 3. معركة أدوات المقارنة والتحقق: `go-cmp` مقابل `testify`

تشهد بيئة Go نقاشاً هندسياً واسعاً حول استخدام مكتبات التأكيد الخارجية (Assertion Libraries) مثل `testify` في مقابل الاعتماد الحصري على مكتبة Go القياسية مع أداة `google/go-cmp`.

```text
┌────────────────────────────────────────────────────────────────────────┐
│                   مقارنة الفلسفة بين go-cmp و testify                  │
├──────────────────────────┬─────────────────────────────────────────────┤
│ google/go-cmp            │ stretchr/testify                            │
├──────────────────────────┼─────────────────────────────────────────────┤
│ • فلسفة Go النقية        │ • فلسفة xUnit الكلاسيكية (DSL خارجي)       │
│ • تفاصيل فروقات دقيقة    │ • فحص تطابق سريع وبسيط                     │
│ • تحكم صارم في الحقول    │ • تكرار جمل Assert/Require على كل حقل      │
│ • فحص عند الفشل فقط      │ • استخدام مكثف للانعكاس (Reflection)       │
└──────────────────────────┴─────────────────────────────────────────────┘
```

### 3.1 لماذا تحظر Google و الشركات الكبرى مكتبات الـ Assertions التقليدية؟

وفق وثيقة Google Go Style Decisions:

1. **فقدان السياق الهندسي (Loss of Context):** استدعاء `assert.Equal(t, a, b)` ينتج رسالة عامة مثل `Not equal: 1 != 2` دون توضيح ماذا يمثل الرقم 1 وماذا يمثل الرقم 2 وما هي الدالة التي أنتجته.
2. **الإيقاف المبكر المفاجئ (Premature Termination):** إساءة استخدام `require` بدلاً من `assert` توقف الاختبار عند أول خطأ، مما يحجب بقية الفحوصات المفيدة ويطيل زمن دورة التعديل والاختبار (Feedback Loop).
3. **قلب ترتيب المعاملات (Inverted Arguments):** يخلط المطورون دائماً بين `assert.Equal(t, expected, actual)` و `assert.Equal(t, actual, expected)` مما يجعل السجلات مضللة تماماً.

### 3.2 التحليل المتعمق لمكتبة `google/go-cmp`

أنتجت Google مكتبة `go-cmp` خصيصاً للتغلب على عيوب `reflect.DeepEqual` التي لا توضح أين يقع الفرق وتفشل في مقارنة الحقول غير المصدرة أو المقارنات التقريبية للأرقام العشرية.

```go
package domain_test

import (
 "testing"
 "time"

 "github.com/google/go-cmp/cmp"
 "github.com/google/go-cmp/cmp/cmpopts"
)

type AuditLog struct {
 ID        string
 Action    string
 UserID    string
 CreatedAt time.Time
 metadata  map[string]string // حقل غير مصدر
}

func TestAuditLogCreation(t *testing.T) {
 got := CreateAuditLog("login", "usr_99")
 
 want := &AuditLog{
  ID:        "any-id", // سيتم تجاهله في المقارنة
  Action:    "login",
  UserID:    "usr_99",
  CreatedAt: time.Now(), // سيتم تجاهله
  metadata:  map[string]string{"ip": "127.0.0.1"},
 }

 // خيارات المقارنة المتقدمة من cmpopts
 opts := cmp.Options{
  // تجاهل الحقول الحساسة للوقت والمعرفات العشوائية
  cmpopts.IgnoreFields(AuditLog{}, "ID", "CreatedAt"),
  
  // السماح بمقارنة الحقول غير المصدرة (Unexported Fields) للبنية
  cmp.AllowUnexported(AuditLog{}),
  
  // مساواة الشرائح الفارغة مع الشرائح غير المهيأة (nil vs empty slice)
  cmpopts.EquateEmpty(),
 }

 // حساب الفروقات بدقة: (-want +got)
 if diff := cmp.Diff(want, got, opts...); diff != "" {
  t.Errorf("CreateAuditLog() mismatch (-want +got):\n%s", diff)
 }
}
```

عندما تفشل هذه المقارنة، ينتج `cmp.Diff` نصاً رسومياً دقيقاً مطابقاً لفروقات Git (`git diff`):

```diff
CreateAuditLog() mismatch (-want +got):
  &domain.AuditLog{
    Action: "login",
-   UserID: "usr_99",
+   UserID: "usr_100",
    metadata: {"ip": "127.0.0.1"},
  }
```

### 3.3 متى وكيف تستخدم `testify` بأمان في المشاريع الكبيرة؟

إذا كان فريقك يفضل سرعة كتابة `testify`، يجب فرض القواعد التالية عبر الـ Linters والـ Code Review:

1. **استخدم `require` للشروط القبلية فقط (Pre-conditions):**
   - مثل: التحقق من نجاح إنشاء قاعدة البيانات، أو عدم وجود خطأ قبل فحص المؤشر (`require.NoError(t, err)` يمنع وقوع `nil pointer panic` في السطر التالي).
2. **استخدم `assert` للمقارنات النهائية (Invariants & Validations):**
   - حتى تستمر بقية الفحوصات في العمل في حال فشل أحد الحقول.
3. **أضف رسائل توضيحية دائماً:**
   - لا تترك الرسالة فارغة: `assert.Equal(t, wantID, gotID, "user ID must match database record")`.

```go
func TestUserRetrieval(t *testing.T) {
 user, err := repo.FindByID("usr_123")
 
 // شرط مسبق: إذا حدث خطأ أو كان المؤشر فارغاً، يجب التوقف فوراً
 require.NoError(t, err, "fetching user from repo failed")
 require.NotNil(t, user, "returned user pointer must not be nil")

 // فحوصات حالة: يمكن إجراؤها تباعاً
 assert.Equal(t, "usr_123", user.ID)
 assert.Equal(t, "active", user.Status)
}
```

---

## 4. استراتيجيات العزل وبدائل الاختبار (Test Doubles): Fakes, Stubs, Mocks, Spies

في هندسة البرمجيات الاحترافية (كما فصّلها مهندسو Google في كتاب *Software Engineering at Google*، الفصل 13)، هناك تصنيف صارم لبدائل الاختبار (Test Doubles) يُساء فهمه كثيراً في مجتمع المطورين:

```text
┌────────────────────────────────────────────────────────────────────────┐
│             تصنيف بدائل الاختبار (Test Doubles Taxonomy)               │
├──────────────┬─────────────────────────────────────────────────────────┤
│ النوع        │ الوظيفة وآلية العمل                                     │
├──────────────┼─────────────────────────────────────────────────────────┤
│ Dummy        │ كائن يُمرر فقط لإكمال معاملات التابع دون أن يُستخدم فعلياً│
├──────────────┼─────────────────────────────────────────────────────────┤
│ Stub         │ كائن يعيد إجابات ثابتة ومجهزة مسبقاً (Hard-coded values) │
├──────────────┼─────────────────────────────────────────────────────────┤
│ Spy          │ كائن يُسجل العمليات التي تمت عليه لفحصها لاحقاً         │
├──────────────┼─────────────────────────────────────────────────────────┤
│ Mock         │ كائن يتوقع استدعاءات معينة ويفشل إن لم تحدث (Interaction)│
├──────────────┼─────────────────────────────────────────────────────────┤
│ Fake         │ تطبيق حقيقي كامل للواجهة، لكنه يعمل بذاكرة خفيفة وسريعة │
└──────────────┴─────────────────────────────────────────────────────────┘
```

### 4.1 تفضيل Google للـ Fakes على الـ Mocks

> [!TIP]
> **قاعدة Google الذهبية:** "Prefer Real Implementations over Fakes, and prefer Fakes over Mocks".

لماذا تتجنب الفرق الهندسية الكبرى الـ Mocks التوليدية (مثل `mockery` أو `gomock`) كلما أمكن؟

1. **الهشاشة الشديدة (Brittle Tests):** تربط الـ Mocks الاختبار بتفاصيل التنفيذ الدقيقة (عدد مرات الاستدعاء، ترتيب استدعاء الدوال الداخلية). إذا قمت بإعادة هيكلة الكود (Refactoring) لتحسين الأداء دون تغيير النتيجة، ستفشل اختبارات الـ Mocks فوراً!
2. **صيانة مكلفة:** تحديث الواجهة يتطلب إعادة توليد أو تعديل كل شجرة التوقعات (`Expect(...)`).
3. **الـ Fakes تحاكي السلوك الحقيقي:** الـ Fake (مثل مستودع بيانات في الذاكرة) يختبر السلوك الفعلي؛ يمكنك إضافة بيانات، استرجاعها، تحديثها، والتأكد من بقاء الحالة متسقة عبر كامل العملية.

### 4.2 بناء In-Memory Fake احترافي وآمن تزامنياً (Production-Grade Fake)

```go
package testutil

import (
 "context"
 "fmt"
 "sync"
)

type User struct {
 ID    string
 Email string
}

// واجهة المستودع التي يحتاجها كود الإنتاج
type UserRepository interface {
 Save(ctx context.Context, u *User) error
 FindByID(ctx context.Context, id string) (*User, error)
 Delete(ctx context.Context, id string) error
}

// تطبيق Fake حقيقي في الذاكرة مع حماية كاملة من الـ Race Condition
type FakeUserRepository struct {
 mu    sync.RWMutex
 users map[string]*User
 
 // خطافات اختيارية لمحاكاة أخطاء الشبكة أثناء الاختبار
 ErrOnSave error
}

func NewFakeUserRepository() *FakeUserRepository {
 return &FakeUserRepository{
  users: make(map[string]*User),
 }
}

func (f *FakeUserRepository) Save(ctx context.Context, u *User) error {
 f.mu.Lock()
 defer f.mu.Unlock()

 if f.ErrOnSave != nil {
  return f.ErrOnSave
 }

 // حفظ نسخة عميقة لمنع تلاعب الاختبار بالمؤشر الخارجي
 f.users[u.ID] = &User{ID: u.ID, Email: u.Email}
 return nil
}

func (f *FakeUserRepository) FindByID(ctx context.Context, id string) (*User, error) {
 f.mu.RLock()
 defer f.mu.RUnlock()

 u, exists := f.users[id]
 if !exists {
  return nil, fmt.Errorf("user %s not found", id)
 }

 return &User{ID: u.ID, Email: u.Email}, nil
}

func (f *FakeUserRepository) Delete(ctx context.Context, id string) error {
 f.mu.Lock()
 defer f.mu.Unlock()

 delete(f.users, id)
 return nil
}
```

### 4.3 متى تصبح الـ Mocks ضرورية؟

تكون الـ Mocks أو الـ Spies مبررة في حالة واحدة أساسية: **التحقق من الآثار الجانبية غير القابلة للقراءة كحالة (Unobservable Side Effects)**:

- التحقق من إرسال بريد إلكتروني تنبيهي عبر بوابة خارجية (SendGrid/SES).
- التحقق من إرسال إشعار دفع (Push Notification).
- التحقق من نشر حدث في وسيط رسائل (Event Bus/Kafka).

### 4.4 مبدأ تصميم الواجهات: "المستهلك هو من يُعرّف الواجهة"

في لغة Go، الواجهات ضمنية (Implicit Interfaces). النمط المضاد الأكثر شيوعاً هو أن تقوم الحزمة المصدرة بتعريف واجهة ضخمة لكل دوالها:

```go
// ❌ نمط مضاد: تعريف الواجهة بجانب التطبيق في حزمة البنية التحتية
package postgres

type DatabaseRepository interface {
    // 50 دالة مختلفة لا يحتاجها المستخدم كلها!
    GetUser(...)
    GetOrder(...)
    SaveInvoice(...)
}
```

```go
//  المعيار الصحيح في Go: المستهلك يعرف واجهة صغيرة تضم فقط ما يحتاجه
package orderservice

type OrderStore interface {
    SaveOrder(ctx context.Context, order *Order) error
}

type Service struct {
    store OrderStore // سهل جداً عمل Fake أو Stub له في الاختبار
}
```

---

## 5. اختبارات التكامل والبيئات الحقيقية (Integration Testing & Testcontainers)

في التطبيقات المعقدة، يفشل اختبار الوحدة المعزول في كشف أعقد المشاكل الإنتاجية:

- استعلامات SQL ذات تركيب خاطئ أو غير مدعوم في محرك الـ DB.
- خرق قيود المفاتيح الأجنبية (Foreign Key Constraints) وتضارب الفهارس الفريدة.
- أقفال التزامن ومعاملات الـ Deadlocks في قواعد البيانات.
- عمليات الـ Serialization المعقدة للـ JSONB وأنواع البيانات المخصصة.

### 5.1 معضلة `go-sqlmock`: لماذا يُعتبر نمطاً مضاداً للأنظمة الحقيقية؟

مكتبة `DATA-DOG/go-sqlmock` تختبر شيئاً واحداً فقط: **أنك كتبت نفس نص الـ SQL الذي توقعت أنك كتبته!**

- إذا غيرت بنية الجدول في الـ Migrations، سينجح اختبار `sqlmock` ويفشل النظام في الإنتاج.
- إذا كان في استعلام الـ SQL خطأ لغوي (Syntax Error)، لن يكتشفه `sqlmock`.
- لذلك، يُوصى عالمياً بالتخلي عن `sqlmock` واستبدالها باختبارات تكامل تعتمد على حاويات قاعدة بيانات حقيقية.

### 5.2 المعيار الصناعي: Testcontainers for Go

تتيح مكتبة `testcontainers-go` تشغيل حاوية Docker حقيقية (مثل PostgreSQL أو Redis) خلال أجزاء من الثانية أثناء الاختبار، ثم إتلافها بالكامل بمجرد انتهاء الاختبار.

```go
//go:build integration

package postgres_test

import (
 "context"
 "database/sql"
 "testing"
 "time"

 _ "github.com/jackc/pgx/v5/stdlib"
 "github.com/testcontainers/testcontainers-go"
 "github.com/testcontainers/testcontainers-go/modules/postgres"
 "github.com/testcontainers/testcontainers-go/wait"
)

func startPostgresContainer(t *testing.T) *sql.DB {
 t.Helper()
 ctx := t.Context()

 // تشغيل حاوية بوستجريس عابرة
 pgContainer, err := postgres.Run(ctx,
  "postgres:16-alpine",
  postgres.WithDatabase("testdb"),
  postgres.WithUsername("postgres"),
  postgres.WithPassword("secret"),
  testcontainers.WithWaitStrategy(
   wait.ForLog("database system is ready to accept connections").
    WithOccurrence(2).
    WithStartupTimeout(15*time.Second)),
 )
 if err != nil {
  t.Fatalf("failed to start postgres container: %v", err)
 }

 // ضمان تنظيف وإغلاق الحاوية فور انتهاء الاختبار
 t.Cleanup(func() {
  if err := pgContainer.Terminate(context.Background()); err != nil {
   t.Logf("failed to terminate container: %v", err)
  }
 })

 connStr, err := pgContainer.ConnectionString(ctx, "sslmode=disable")
 if err != nil {
  t.Fatalf("failed to get connection string: %v", err)
 }

 db, err := sql.Open("pgx", connStr)
 if err != nil {
  t.Fatalf("failed to open database: %v", err)
 }

 t.Cleanup(func() {
  _ = db.Close()
 })

 return db
}

func TestUserRepository_Integration(t *testing.T) {
 db := startPostgresContainer(t)

 // تشغيل الـ Migrations الفعلية
 setupSchema(t, db)

 repo := NewPostgresUserRepository(db)

 t.Run("insert and query user successfully", func(t *testing.T) {
  ctx := t.Context()
  err := repo.Create(ctx, &User{ID: "usr_10", Email: "alex@example.com"})
  if err != nil {
   t.Fatalf("Create() unexpected error: %v", err)
  }

  u, err := repo.GetByID(ctx, "usr_10")
  if err != nil {
   t.Fatalf("GetByID() unexpected error: %v", err)
  }

  if u.Email != "alex@example.com" {
   t.Errorf("GetByID() email = %q, want %q", u.Email, "alex@example.com")
  }
 })
}
```

### 5.3 عزل وسوم البناء (Build Tags) لاختبارات التكامل

لكي لا تؤثر اختبارات التكامل الثقيلة على سرعة التغذية الراجعة السريعة أثناء التطوير اليومي، يجب عزلها بوسم بناء في السطر الأول من الملف:

```go
//go:build integration
```

- تشغيل اختبارات الوحدة فقط (سريعة جداً):

  ```bash
  go test ./...
  ```

- تشغيل اختبارات التكامل:

  ```bash
  go test -v -tags=integration ./...
  ```

---

## 6. اختبار التزامن وسباق البيانات (Concurrency, Race Detection & Synctest)

لغة Go مبنية أصلاً حول الـ Concurrency والـ Goroutines، مما يجعل الأنظمة عرضة لثغرات سباق البيانات (Data Races)، وحالات التوقف التام (Deadlocks)، وتسريب الـ Goroutines.

### 6.1 كاشف سباق البيانات المعياري (`go test -race`)

أداة `-race` مدمجة مباشرة في مترجم Go وتعتمد على محرك ThreadSanitizer. تقوم بإضافة تحقيقات برمجية (Instrumentation) على كل قراءة وكتابة في الذاكرة لكشف تضارب العمليات المتزامنة التي تفتقر للمزامنة (`sync.Mutex` أو القنوات).

> [!CAUTION]
> **قاعدة إلزامية في الـ CI:** يجب دائماً وأبداً تشغيل الاختبارات في خط أنابيب الدمج المستمر (CI) باستخدام الراية `-race`:
>
> ```bash
> go test -race -count=1 ./...
> ```
>
> *ملاحظة:* يضيف `-race` عبئاً إضافياً على المعالج بنسبة 2x إلى 10x وعلى الذاكرة بنسبة 5x إلى 20x، لذا لا يُستخدم في الإنتاج، ولكنه إلزامي في الاختبارات والـ Staging.

### 6.2 كشف تسريب الـ Goroutines باستخدام `uber-go/goleak`

أحد أخطر العيوب الصامتة في Go هو تسريب الـ Goroutines: تشغيل Goroutine ينتظر قناة لا تُغلق أبداً أو اتصال شبكي دون مهلة، مما يؤدي إلى استنزاف الذاكرة تدريجياً وانهيار الخدمة (OOM Kill).

توفر مكتبة `go.uber.org/goleak` فحصاً دقيقاً لجدول الـ Goroutines النشطة:

```go
package service_test

import (
 "testing"

 "go.uber.org/goleak"
)

func TestMain(m *testing.M) {
 // فحص تسريب الـ Goroutines عبر كامل الحزمة بعد انتهاء جميع الاختبارات
 goleak.VerifyTestMain(m)
}

func TestConcurrentWorker(t *testing.T) {
 // أو يمكن فحص كل اختبار بشكل فردي
 defer goleak.VerifyNone(t)

 w := NewWorker()
 w.Start()
 w.Stop() // إذا نسي المطور إيقاف حلقة الـ worker، سيفشل goleak الاختبار فوراً!
}
```

### 6.3 الثورة الهندسية في Go 1.24+: حزمة `testing/synctest`

تاريخياً، كان اختبار الكود المتزامن المعتمد على الوقت (Timers, Tickers, Backoffs) كابوساً: إما أن تجعل الاختبارات بطيئة باستخدام `time.Sleep` حقيقي، أو تصنع تجريدات معقدة للساعة (Clock Abstractions).

قدمت Go 1.24 حزمة تجريبية ثورية: `testing/synctest` تُنشئ **"فقاعة وقت افتراضية" (Virtual Time Bubble)**:

1. يتم تشغيل الكود المتزامن داخل الدالة `synctest.Run(func() { ... })`.
2. يتم تجميد الوقت الحقيقي واستبداله بوقت افتراضي.
3. عندما تتوقف جميع الـ Goroutines داخل الفقاعة وتنتظر (Blocked on sleep or channels)، يقفز الوقت الافتراضي فوراً إلى لحظة استيقاظ الـ Timer التالي دون أي انتظار في العالم الواقعي!

```go
package async_test

import (
 "testing"
 "testing/synctest"
 "time"
)

// دالة تعتمد على الانتظار والمهل الزمنية
func RetryWithBackoff(fn func() bool) bool {
 for attempt := 0; attempt < 3; attempt++ {
  if fn() {
   return true
  }
  time.Sleep(10 * time.Second) // انتظار 10 ثوانٍ حقيقية في الكود العادي!
 }
 return false
}

func TestRetryWithBackoff_Fast(t *testing.T) {
 // يتطلب تفعيل GOEXPERIMENT=synctest أثناء البناء
 synctest.Run(func() {
  attempts := 0
  success := RetryWithBackoff(func() bool {
   attempts++
   return attempts == 3
  })

  // ينتهي هذا الاختبار في أجزاء من الملي ثانية بدلاً من 20 ثانية!
  if !success {
   t.Errorf("expected success, got failure")
  }
  if attempts != 3 {
   t.Errorf("attempts = %d, want 3", attempts)
  }
 })
}
```

---

## 7. اختبارات الأداء والقياس المقارن (Benchmarking, Profiling & Benchstat)

توفر حزمة `testing` آلية قياس أداء أصلية وعالية الدقة عبر بنية `testing.B`.

### 7.1 التشريح الدقيق للـ Benchmark وضبط الميقاتي

يقوم محرك الاختبار بتشغيل الدالة مع قيمة `b.N` متغيرة تلقائياً حتى تستقر العينة الإحصائية (افتراضياً لمدة ثانية واحدة).

```go
package parser_test

import (
 "bytes"
 "runtime"
 "testing"
)

// متغير عام لمنع المترجم من إلغاء الكود الميت (Dead Code Elimination)
var benchmarkResult []byte

func BenchmarkJSONParsing(b *testing.B) {
 payload := []byte(`{"id": "12345", "name": "Antigravity", "active": true}`)

 // 1. تفعيل تقارير تخصيص الذاكرة (Allocations & Bytes per op)
 b.ReportAllocs()

 // 2. إيقاف الميقاتي أثناء التجهيز الأولي المكلف
 b.ResetTimer()

 var res []byte
 for i := 0; i < b.N; i++ {
  // العملية المطلوب قياس أدائها بدقة
  res = FastParse(payload)
 }

 // 3. تخزين النتيجة في متغير عام واستخدام KeepAlive لمنع تحسينات المترجم
 benchmarkResult = res
 runtime.KeepAlive(res)
}
```

### 7.2 القياس المتوازي للأداء (`b.RunParallel`)

لقياس قدرة التابع على التوسع واستغلال تعدد الأنوية وتفادي أقفال الـ Lock Contention:

```go
func BenchmarkConcurrentCacheGet(b *testing.B) {
 cache := NewThreadSafeCache()
 cache.Set("key", "value")

 b.ReportAllocs()
 b.ResetTimer()

 b.RunParallel(func(pb *testing.PB) {
  // pb.Next تعيد true طالما بقيت دورات متبقية في ميزانية الاختبار
  for pb.Next() {
   _ = cache.Get("key")
  }
 })
}
```

### 7.3 المقارنة الإحصائية للتطوير والتحسين باستخدام `benchstat`

الاعتماد على نظرة عابرة لأرقام الـ Benchmark عرضة للخطأ بسبب تقلب ضغط المعالج. الأداة الرسمية المعتمدة هي `golang.org/x/perf/cmd/benchstat`.

#### الخطوات الإنتاجية لتقييم تحسين كود (Optimization Proof)

1. **تسجيل الأداء الأساسي (Base Branch):**

   ```bash
   go test -bench=BenchmarkJSONParsing -count=10 > old.txt
   ```

2. **تطبيق التعديلات البرمجية وتشغيل الاختبار (New Branch):**

   ```bash
   go test -bench=BenchmarkJSONParsing -count=10 > new.txt
   ```

3. **المقارنة الإحصائية عبر `benchstat`:**

   ```bash
   benchstat old.txt new.txt
   ```

ينتج عن ذلك تقرير علمي يوضح نسبة التحسن مع الـ p-value للتأكد من أن الفارق ذو دلالة إحصائية وليس صدفة:

```text
goos: linux
goarch: amd64
pkg: myapp/parser
                 │   old.txt   │               new.txt               │
                 │   sec/op    │   sec/op     vs base                │
JSONParsing-16     145.2n ± 2%   102.1n ± 1%  -29.68% (p=0.000 n=10)

                 │   old.txt   │               new.txt               │
                 │    B/op     │    B/op      vs base                │
JSONParsing-16       48.00 ± 0%    0.00 ± 0%  -100.00% (p=0.000 n=10)
```

---

## 8. الاختبار العشوائي والاختبار القائم على الخصائص (Fuzzing & Property-Based Testing)

في معالجة المدخلات الخارجية (Parsers, Protocols, Cryptography, Serialization)، تفشل عقول المطورين في تخيل كل الحالات الشاذة المحتملة (Corrupted payloads, Buffer Overflows, Unicode edge-cases).

### 8.1 الاختبار العشوائي الأصيل (Go Native Fuzzing `testing.F`)

مدمج مباشرة في لغة Go منذ الإصدار 1.18. يقوم المحرك بتوليد مدخلات عشوائية ومراقبة مسارات التنفيذ وتغطية الكود (Coverage-guided fuzzing) لاكتشاف الانهيارات (Panics) أو استهلاك الذاكرة اللانهائي.

```go
package codec_test

import (
 "bytes"
 "testing"
)

// دالة التشفير وفك التشفير المطلوب فحصها
func Encode(data []byte) []byte { /* ... */ }
func Decode(data []byte) ([]byte, error) { /* ... */ }

func FuzzCodecRoundTrip(f *testing.F) {
 // 1. إضافة مدخلات بذرة أولية معروفة (Seed Corpus)
 f.Add([]byte("hello world"))
 f.Add([]byte(""))
 f.Add([]byte("\x00\xff\xfe\xfd"))
 f.Add(bytes.Repeat([]byte("A"), 1024))

 // 2. تشغيل الحلقة العشوائية
 f.Fuzz(func(t *testing.T, original []byte) {
  encoded := Encode(original)
  decoded, err := Decode(encoded)
  if err != nil {
   t.Fatalf("failed to decode validly encoded data: %v", err)
  }

  if !bytes.Equal(original, decoded) {
   t.Fatalf("Round-trip mismatch: got %v, want %v", decoded, original)
  }
 })
}
```

تشغيل الـ Fuzzing في سطر الأوامر:

```bash
# تشغيل الفحص العشوائي لمدة دقيقة واحدة للبحث عن أي انهيار
go test -fuzz=FuzzCodecRoundTrip -fuzztime=1m
```

إذا وجد المحرك حالة تفشل الكود، فإنه يقوم بـ:

1. تقليص المدخل لأصغر حجم مسبب للخطأ (Minimization).
2. حفظ الحالة الفاشلة في ملف داخل مجلد `testdata/fuzz/FuzzCodecRoundTrip/...`.
3. يتحول هذا الملف تلقائياً إلى اختبار وحدة عادي يتم تشغيله في كل دورة `go test` لضمان عدم حدوث انتكاسة برمجية (Regression).

### 8.2 الاختبار القائم على الخصائص (Property-Based Testing عبر `rapid`)

بينما يعتمد الـ Fuzzing على توليد عشوائي خام للمصفوفات، فإن الـ Property-Based Testing يفحص النظريات الرياضية والخصائص الثابتة للكود (Invariants) عبر توليد هياكل بيانات معقدة (Structs, Trees, State Machines).

الحزمة الأقوى عالمياً في Go حالياً هي `pgregory.net/rapid` (تتفوق بمراحل على `testing/quick` القديمة):

```go
package collections_test

import (
 "sort"
 "testing"

 "pgregory.net/rapid"
)

func TestSortProperties(t *testing.T) {
 rapid.Check(t, func(t *rapid.T) {
  // توليد شريحة عشوائية من الأعداد الصحيحة بأطوال وقيم متنوعة
  slice := rapid.SliceOf(rapid.Int()).Draw(t, "slice")

  sorted := make([]int, len(slice))
  copy(sorted, slice)
  sort.Ints(sorted)

  // خاصية 1: يجب أن تحافظ الشريحة على نفس الطول
  if len(sorted) != len(slice) {
   t.Fatalf("length changed: got %d, want %d", len(sorted), len(slice))
  }

  // خاصية 2: يجب أن تكون العناصر مرتبة تصاعدياً دوماً
  for i := 1; i < len(sorted); i++ {
   if sorted[i-1] > sorted[i] {
    t.Fatalf("elements not sorted at index %d: %d > %d", i, sorted[i-1], sorted[i])
   }
  }
 })
}
```

---

## 9. هندسة المعمارية وتنظيم حزم الاختبار (Architecture & Code Organization)

في المشاريع الكبيرة التي تحتوي على مئات الحزم وملايين أسطر الكود، يصبح تنظيم ملفات الاختبار عاملاً حاسماً لمنع دورات الاستيراد (Import Cycles) والحفاظ على نظافة الـ API العام.

### 9.1 الاختبارات الصندوق الأبيض (White-box) مقابل الصندوق الأسود (Black-box)

```text
┌────────────────────────────────────────────────────────────────────────┐
│                   مقارنة بين White-Box و Black-Box Tests               │
├──────────────────────────┬─────────────────────────────────────────────┤
│ White-Box (داخل الحزمة)  │ Black-Box (خارج الحزمة)                     │
├──────────────────────────┼─────────────────────────────────────────────┤
│ • `package foo`          │ • `package foo_test`                        │
│ • يرى المتغيرات والحقول  │ • يرى فقط الواجهة العامة المصدرة            │
│   غير المصدرة            │   (Exported API)                            │
│ • قد يسبب دورات استيراد  │ • يستحيل أن يسبب دورات استيراد              │
│ • مخصص لاختبار الخوارزميات│ • المعيار المفضل للمكتبات وحزم الأعمال      │
└──────────────────────────┴─────────────────────────────────────────────┘
```

> [!TIP]
> **توصية Google:** اكتب معظم اختباراتك بنمط الصندوق الأسود (`package foo_test`) لضمان أنك تختبر الحزمة كما سيراها ويستخدمها المطورون الخارجيون.

### 9.2 نمط `export_test.go` السحري (The Export Pattern)

ماذا لو كنت تستخدم `package foo_test` (الصندوق الأسود) ولكنك تحتاج فحص متغير داخلي أو استبدال تابع داخلي لأغراض الاختبار فقط دون تصديره للمستخدمين؟

الحل المعياري في Go القياسية هو إنشاء ملف اسمه `export_test.go` داخل الحزمة الأصلية (`package foo`):

- هذا الملف لن يتم تضمينه في ملفات الإنتاج النهائية (`go build` تتجاهل ملفات `_test.go`).
- يقوم الملف بتصدير ما تحتاجه حصرياً أثناء مرحلة الاختبار:

```go
// ملف: internal/auth/export_test.go
package auth

// تصدير دالة داخلية خاصة لتمكين حزمة auth_test من فحصها
var (
 ExportedValidateTokenInternal = validateTokenInternal
 ExportedResetGlobalRateLimiter = resetGlobalRateLimiter
)
```

```go
// ملف: internal/auth/auth_test.go
package auth_test

import (
 "testing"
 "myapp/internal/auth"
)

func TestTokenInternal(t *testing.T) {
 // الوصول للدالة الداخلية عبر البوابة المصدرة للاختبار فقط
 res := auth.ExportedValidateTokenInternal("dummy_token")
 if !res {
  t.Errorf("expected token to be valid")
 }
}
```

### 9.3 تنظيم ملفات الاختبار والبيانات المرجعية `testdata/` و Golden Files

- أي مجلد يحمل الاسم `testdata` تتجاهله أداة بناء Go تماماً عند بناء الحزم الثنائية.
- يُستخدم لحفظ الملفات الثابتة، مخرجات الـ JSON المتوقعة، وشهادات الـ SSL التجريبية.

#### نمط الملفات الذهبية (Golden Files Pattern with `-update`)

عند اختبار مخرجات نصية أو معقدة (مثل توليد شفرة HTML أو قوالب معقدة)، نقوم بحفظ النتيجة الصحيحة في ملف `testdata/output.golden`:

```go
package renderer_test

import (
 "flag"
 "os"
 "path/filepath"
 "testing"

 "github.com/google/go-cmp/cmp"
)

// راية مخصصة لتحديث الملفات الذهبية عبر سطر الأوامر
var updateGolden = flag.Bool("update", false, "update .golden files")

func TestRenderComplexReport(t *testing.T) {
 got := RenderReport(sampleData)
 goldenPath := filepath.Join("testdata", "report.golden")

 if *updateGolden {
  if err := os.WriteFile(goldenPath, []byte(got), 0644); err != nil {
   t.Fatalf("failed to update golden file: %v", err)
  }
 }

 want, err := os.ReadFile(goldenPath)
 if err != nil {
  t.Fatalf("failed to read golden file: %v", err)
 }

 if diff := cmp.Diff(string(want), got); diff != "" {
  t.Errorf("RenderReport() mismatch against golden file (-want +got):\n%s", diff)
 }
}
```

- لتحديث المخرجات عند تغيير التصميم عمداً:

  ```bash
  go test -update ./...
  ```

---

## 10. مصفوفة الأنماط المضادة الشائعة في اختبارات Go (Anti-Patterns Matrix)

| # | النمط المضاد (Anti-Pattern) | لماذا يُعتبر خطأً جسيماً؟ | الحل المعياري والبديل الصحيح |
| :- | :--- | :--- | :--- |
| **1** | استخدام `time.Sleep` لانتظار العمليات المتزامنة | يسبب اختبارات متقلبة (Flaky Tests) ويبطئ الـ CI بدون داعٍ | استخدام `sync.WaitGroup`، أو فحص القنوات، أو `testing/synctest` في Go 1.24+. |
| **2** | نسيان `t.Parallel()` في الاختبارات الآمنة | إهدار إمكانيات عتاد الـ CI وإطالة زمن الاختبارات لعشرات الدقائق | تفعيل `t.Parallel()` على مستوى الدالة والـ Subtests مع حماية المتغيرات. |
| **3** | استخدام `defer` للتنظيف داخل `t.Run` | الـ `defer` ينفذ عند خروج الدالة الكبرى وليس الاختبار الفرعي | استخدام `t.Cleanup(fn)` دوماً لإدارة دورة حياة الموارد. |
| **4** | الاعتماد على `go-sqlmock` للأنظمة المعقدة | يختبر نصوص الـ SQL فقط ويتجاهل القيود والأخطاء الحقيقية | استخدام `testcontainers-go` مع حاوية Postgres مؤقتة حقيقية. |
| **5** | توليد Mocks ضخمة لكل الواجهات في النظام | ربط الاختبارات بتفاصيل التنفيذ وانهيارها عند كل Refactoring | تفضيل الـ Fakes الخفيفة في الذاكرة على الـ Mocks التوليدية. |
| **6** | استخدام `t.Fatal` في دوال المساعدة دون `t.Helper()` | ظهور رقم السطر داخل الدالة المساعدة بدلاً من مكان الاستدعاء الفعلي | إضافة `t.Helper()` في بداية كل دالة مساعدة للاختبار. |
| **7** | تجاهل `-race` في خطوط أنابيب الـ CI | مرور سباقات بيانات قاتلة تقود لانهيارات صامتة في الإنتاج | فرض تشغيل `go test -race ./...` إلزامياً قبل أي دمج للكود. |
| **8** | استدعاء `os.Setenv` دون استعادتها | تلوث حالة بيئة النظام وتأثير الاختبار على الاختبارات التالية | استخدام `t.Setenv("KEY", "VAL")` التي تتكفل بالاستعادة تلقائياً. |
| **9** | نسيان استدعاء `b.ResetTimer()` في الـ Benchmark | احتساب وقت تهيئة الموارد الثقيلة ضمن زمن العملية البرمجية | استدعاء `b.ResetTimer()` بعد الانتهاء من التجهيز مباشرة وقبل حلقة القياس. |
| **10** | كتابة جداول اختبار بحقول شرطية معقدة | تحويل جدول الاختبار إلى لغة برمجية معقدة يصعب قراءتها وصيانتها | تقسيم الحالات المتباينة إلى دوال `Test...` منفصلة وواضحة. |

---

## 11. قائمة مراجعة الجاهزية للإنتاج وبيئات الـ CI/CD (Production Readiness Checklist)

قبل اعتماد كود الخدمة وتمرير الـ Pull Requests في المشاريع الكبرى، تأكد من استيفاء هذه البنود:

```text
┌────────────────────────────────────────────────────────────────────────┐
│               قائمة التحقق الهندسية لجودة ومنظومة الاختبار             │
├────────────────────────────────────────────────────────────────────────┤
│ [ ] تشغيل كاشف السباق (-race) إلزامياً في كل عملية بناء بالـ CI       │
│ [ ] عزل اختبارات التكامل الثقيلة بوسم //go:build integration           │
│ [ ] التحقق من عدم تسريب الـ Goroutines عبر goleak.VerifyTestMain       │
│ [ ] استبدال defer داخل الدوال الفرعية والمساعدة بـ t.Cleanup           │
│ [ ] وضع t.Helper() في كل دالة مساعدة لضمان دقة أرقام أسطر الأخطاء      │
│ [ ] استخدام t.TempDir() و t.Setenv() بدلاً من التعديل اليدوي في البيئة │
│ [ ] كتابة رسائل الفشل بصياغة Got before Want: Got = %v, Want = %v      │
│ [ ] استخدام google/go-cmp للمقارنات المعقدة وتجاهل الحقول العشوائية    │
│ [ ] اعتماد Fakes حقيقية في الذاكرة لقواعد البيانات بدلاً من sqlmock    │
│ [ ] تفعيل أدوات الفحص الصارمة (Linters) في golangci-lint               │
└────────────────────────────────────────────────────────────────────────┘
```

### 11.1 تكوين أدوات الفحص الصارمة في `golangci-lint`

يجب تفعيل محللات الجودة المتخصصة في كود الاختبار داخل `.golangci.yml`:

```yaml
linters:
  enable:
    - thelper      # يكتشف نسيان وضع t.Helper() في دوال المساعدة
    - tparallel    # يرصد أخطاء استخدام t.Parallel() الشائعة مع الحلقات
    - testifylint  # يصحح الاستخدام الخاطئ لمكتبة testify ويرتب got قبل want
    - paralleltest # يرصد الاختبارات التي تفتقد لـ t.Parallel()
    - wastedassign # يكتشف المتغيرات المهدرة في الاختبارات
```

### 11.2 أمر الـ Makefile النموذجي لتشغيل جميع مستويات الاختبار

```makefile
.PHONY: test test-race test-integration test-bench test-fuzz

# اختبارات الوحدة السريعة مع كاشف السباق
test:
 go test -race -v -timeout=60s ./...

# اختبارات التكامل التي تتطلب Docker و Testcontainers
test-integration:
 go test -race -v -tags=integration -timeout=10m ./...

# اختبارات الأداء وتخصيص الذاكرة
test-bench:
 go test -run=^$$ -bench=. -benchmem ./...

# فحص تسريب الـ Goroutines ومشاكل التزامن بتكرار الاختبار 50 مرة
test-stress:
 go test -race -count=50 ./internal/...
```

---

## 12. المصادر والمراجع الرسمية والمعايير الصناعية

1. **المكتبة القياسية والمقالات الرسمية لـ Go:**
   - توثيق حزمة `testing`: [https://pkg.go.dev/testing](https://pkg.go.dev/testing)
   - توثيق حزمة `testing/synctest`: [https://pkg.go.dev/testing/synctest](https://pkg.go.dev/testing/synctest)
   - مقال مدونة Go حول الاختبارات الفرعية: [Using Subtests and Sub-benchmarks](https://go.dev/blog/subtests)
   - مقال مدونة Go حول كاشف سباق البيانات: [Introducing the Go Race Detector](https://go.dev/blog/race-detector)
   - دليل الاختبار العشوائي: [Fuzzing Tutorial](https://go.dev/doc/tutorial/fuzz)
2. **أدلة المعايير الهندسية الصادرة من الشركات العالمية:**
   - الدليل الإرشادي لمعايير Go في Google: [Google Go Style Guide: Decisions](https://google.github.io/styleguide/go/decisions#testing)
   - أفضل ممارسات الاختبار وبدائل الاختبار في Google: [Google Go Style Best Practices](https://google.github.io/styleguide/go/best-practices#test-doubles)
   - كتاب هندسة البرمجيات في Google (فصل بدائل الاختبار): [Software Engineering at Google (SWE Book: Chapter 13)](https://abseil.io/resources/swe-book/html/ch13.html)
   - دليل شركة Uber في لغة Go: [Uber Go Style Guide: Testing](https://github.com/uber-go/guide/blob/master/style.md#testing)
3. **أبرز الأدوات والمكتبات مفتوحة المصدر:**
   - مكتبة المقارنات الدقيقة من Google: [github.com/google/go-cmp](https://github.com/google/go-cmp)
   - حاويات الاختبار الرسمية: [testcontainers.com/modules/postgres](https://testcontainers.com/modules/postgres/)
   - كاشف تسريب الـ Goroutines: [github.com/uber-go/goleak](https://github.com/uber-go/goleak)
   - حزمة فحص الخصائص المتقدمة: [github.com/flyingmutant/rapid](https://github.com/flyingmutant/rapid)
   - أداة التحليل الإحصائي للأداء: [golang.org/x/perf/cmd/benchstat](https://pkg.go.dev/golang.org/x/perf/cmd/benchstat)
