---
name: go-testing
description: "Engineering and maintaining our project's testing infrastructure and quality engineering based on global best practices, Google Go Style Guide, Uber Go Style Guide, SWE Book, and Go 1.24+ features. Covers table-driven tests, t.Parallel & t.Cleanup lifecycle, failure messages (got before want), google/go-cmp vs testify, in-memory thread-safe fakes over mocks, Testcontainers for real ephemeral databases, concurrency race detector (-race), goroutine leak detection (uber-go/goleak), Go 1.24+ synctest virtual time bubbles, benchmarking & benchstat statistical analysis, native fuzzing (testing.F), property-based testing (pgregory.net/rapid), package architecture (white-box vs black-box & export_test.go), golden files, and CI/CD quality gates."
---

# مهارة هندسة وجودة الاختبارات في Go (`go-testing`)

تحدد هذه المهارة المعمارية القياسية الهندسية لتصميم، وهيكلة، وتنفيذ، وصيانة **منظومة الاختبارات وضمان الجودة (Testing & Quality Engineering)** في مشاريع Go الموزعة الكبيرة، استناداً إلى أرقى المعايير العالمية الرسمية الصادرة عن **فريق تطوير Go الرسمي (Go Core Team)**، ودليل **[Google Go Style Guide: Decisions & Best Practices](https://google.github.io/styleguide/go/decisions#testing)**، وكتاب **[Software Engineering at Google (SWE Book: Chapters 11-14)](https://abseil.io/resources/swe-book/html/ch13.html)**، ودليل **[Uber Go Style Guide](https://github.com/uber-go/guide/blob/master/style.md#testing)**، وأحدث المزايا البرمجية في **Go 1.24+** (مثل `t.Context()` وحزمة `testing/synctest`).

تم تصميم هذه المهارة لتكون الدليل الإرشادي الموحد لجميع مستويات الاختبار في المشروع: بدءاً من اختبارات الوحدة السريعة فائقة الموازاة، واختبارات التكامل المعتمدة على حاويات Docker الحقيقية عبر Testcontainers، واختبارات التزامن وخلو الشيفرة من سباق البيانات وتسريب الـ Goroutines، وصولاً إلى قياس الأداء المعياري (Benchmarking)، والاختبار العشوائي الأصيل (Native Fuzzing)، واختبار الخصائص الرياضية (Property-Based Testing).

---

## بنية ملفات المهارة والحزمة في المشروع (`Skill & Directory Layout`)

```text
train_struct/
├── docs/
│   └── analysis/
│       └── go_testing_best_practices.md # التحليل المعماري النظري المرجعي الشامل
│
└── .agents/skills/go-testing/           # الدليل المعماري التخصصي للمهارة
    ├── SKILL.md                         # هذا الملف: وثيقة التوجيهات الهندسية الأساسية
    ├── docs/                            # الأدلة المعمارية والتطبيقية التفصيلية
    │   ├── 01_architecture_and_test_pyramid.md
    │   ├── 02_table_driven_tests_and_subtests.md
    │   ├── 03_assertions_and_go_cmp_deep_dive.md
    │   ├── 04_test_doubles_fakes_vs_mocks.md
    │   ├── 05_integration_testing_testcontainers.md
    │   ├── 06_concurrency_race_and_synctest.md
    │   ├── 07_benchmarking_profiling_and_benchstat.md
    │   ├── 08_fuzzing_and_property_based_testing.md
    │   └── 09_package_architecture_and_golden_files.md
    ├── examples/                        # كود Go نموذجي جاهز ومطابق للمواصفات
    │   ├── table_driven_test.go
    │   ├── gocmp_comparison_test.go
    │   ├── threadsafe_inmemory_fake.go
    │   ├── testcontainers_postgres_test.go
    │   ├── synctest_virtual_clock_test.go
    │   ├── benchmark_benchstat_test.go
    │   ├── fuzz_roundtrip_test.go
    │   └── golden_files_test.go
    └── references/                      # مصفوفات الفحص والأنماط المضادة
        ├── antipatterns_matrix.md
        └── production_checklist.md
```

---

## 1. الفلسفة الهندسية ومبادئ الاختبار الصارمة

1. **الاختبار كود عادي (Tests are ordinary Go code):** لا تستخدم أطر عمل سحرية أو لغات نطاق مخصصة (DSL). التحكم بالتدفق يتم باستخدام بنى اللغة الطبيعية (`if`, `for`, `switch`).
2. **اختبار الحالة والسلوك وليس تفاصيل التنفيذ (State & Behavior over Interactions):** اختبر المدخلات والمخرجات وحالة النظام؛ تجنب الـ Mocks التي تفحص عدد مرات استدعاء الدوال الداخلية أو ترتيبها، واعتمد على الـ In-Memory Fakes الحقيقية.
3. **الفشل التشخيصي الواضح (Got before Want):** صياغة رسائل الأخطاء دوماً بالصيغة: `Func(args) = got, want want`.
4. **استمرارية الفحص (Keep Going on Failure):** تفضيل `t.Errorf` على `t.Fatalf` حتى تظهر جميع المشاكل في دورة واحدة، إلا في حال تعذر إكمال الاختبار (مثل فشل إنشاء الاتصال بقاعدة البيانات).
5. **إلزامية كاشف السباق (`-race`):** لا يُقبل أي كود في بيئة الإنتاج ما لم يجتز فحص `go test -race` بنجاح تام.
6. **مكافحة تسريب الـ Goroutines:** استخدام `uber-go/goleak` للتحقق من إغلاق كافة خيوط التنفيذ والقنوات.
7. **إدارة دورة حياة الموارد عبر `t.Cleanup`:** حظر استخدام `defer` لتنظيف الموارد داخل الاختبارات الفرعية `t.Run`.
8. **دوال مساعدة دقيقة:** وضع `t.Helper()` إلزامياً في كل دالة مساعدة لتوثيق رقم السطر الفعلي المستدعي عند الفشل.

---

## 2. مصفوفة مستويات الاختبار وأدواتها المعتمدة

| مستوى الاختبار | النطاق والهدف | الأداة المعتمدة | وسم البناء / الأمر |
| :--- | :--- | :--- | :--- |
| **Unit Tests** | الدوال، المنطق النقي، والحزم المعزولة. | `testing` + `google/go-cmp` + Fakes | `go test -race ./...` |
| **Integration Tests** | تكامل المستودعات مع قواعد البيانات والوسائط. | `testcontainers-go` (PostgreSQL/Redis) | `go test -race -tags=integration ./...` |
| **Concurrency Tests** | فحص تسريب الخيوط والتزامن والوقت الافتراضي. | `uber-go/goleak` + `testing/synctest` | `go test -race -count=50 ./...` |
| **Benchmarks** | قياس سرعة المعالجة واستهلاك الذاكرة ومقارنتها. | `testing.B` + `benchstat` | `go test -bench=. -benchmem` |
| **Fuzz Tests** | فحص بروتوكولات التشفير والمحللات اللغوية بمدخلات عشوائية. | Go Native Fuzzing (`testing.F`) | `go test -fuzz=FuzzName -fuzztime=1m` |
| **Property Tests** | فحص القوانين الحسابية والخصائص المعقدة. | `pgregory.net/rapid` | `go test ./...` |

---

## 3. إرشادات العمل وتوجيهات التنفيذ السريع للمطورين

### 3.1 عند كتابة اختبارات الوحدة (Unit Tests)

- استخدم نمط **Table-Driven Tests** مع شريحة مجهولة من بنية الاختبار (`[]struct`).
- فعّل الموازاة عبر `t.Parallel()` في الاختبار الرئيسي والاختبارات الفرعية.
- قارن الهياكل المعقدة بواسطة `cmp.Diff(want, got, opts...)` بدلاً من كتابة عشرات جمل `assert`.
- استخدم `t.Context()` في Go 1.24+ كبديل مباشر لـ `context.Background()` لضمان إلغاء العمليات فور انتهاء الاختبار.

### 3.2 عند الحاجة لبدائل الاختبار (Test Doubles)

- ابنِ **In-Memory Fake** بسيط ومحمي بقفل `sync.RWMutex` بدلاً من توليد Mocks ضخمة بواسطة `mockery` أو `gomock`.
- طبق مبدأ: *"المستهلك هو من يُعرّف الواجهة"*؛ عرّف واجهات صغيرة تحوي فقط التوابع التي تحتاجها الخدمة (`1-3 توابع`).
- قصر استخدام الـ Mocks/Spies فقط على التحقق من الآثار الجانبية غير القابلة للرصد (مثل إرسال بريد إلكتروني أو رسالة SMS).

### 3.3 عند كتابة اختبارات التكامل (Integration Tests)

- لا تستخدم `go-sqlmock` للتحقق من استعلامات الـ SQL المعقدة.
- استخدم `testcontainers-go` لتشغيل حاوية Postgres حقيقية عابرة مع استدعاء `t.Cleanup` لإتلافها.
- أضف وسم البناء في أول سطر من الملف: `//go:build integration`.

### 3.4 عند اختبار التزامن والمهل الزمنية

- لا تستخدم `time.Sleep` في الاختبارات أبداً!
- في Go 1.24+، استخدم حزمة `testing/synctest` عبر الدالة `synctest.Run(func() { ... })` لتشغيل الكود في فقاعة وقت افتراضية تسافر عبر الزمن فورياً.
- افحص تسريب الـ Goroutines عبر `defer goleak.VerifyNone(t)` أو عبر `goleak.VerifyTestMain(m)`.

---

## 4. قائمة مراجع المهارة التفصيلية

- [01. المعمارية وهرم الاختبارات](docs/01_architecture_and_test_pyramid.md)
- [02. الاختبارات الموجهة بالجداول والاختبارات الفرعية](docs/02_table_driven_tests_and_subtests.md)
- [03. المقارنات العميقة: go-cmp مقابل testify](docs/03_assertions_and_go_cmp_deep_dive.md)
- [04. بدائل الاختبار: Fakes مقابل Mocks](docs/04_test_doubles_fakes_vs_mocks.md)
- [05. اختبارات التكامل والحاويات العابرة (Testcontainers)](docs/05_integration_testing_testcontainers.md)
- [06. اختبارات التزامن، كاشف السباق، و synctest](docs/06_concurrency_race_and_synctest.md)
- [07. اختبارات الأداء، التحليل البروفيلي، و benchstat](docs/07_benchmarking_profiling_and_benchstat.md)
- [08. الاختبار العشوائي واختبار الخصائص](docs/08_fuzzing_and_property_based_testing.md)
- [09. معمارية الحزم والملفات الذهبية](docs/09_package_architecture_and_golden_files.md)
- [مصفوفة الأنماط المضادة الشائعة](references/antipatterns_matrix.md)
- [قائمة مراجعة الجاهزية للإنتاج](references/production_checklist.md)
