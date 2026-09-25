# التوثيق الشامل والمعماري لحزمة `context` في لغة Go (Standard Library)

مرحباً بك في الدليل المرجعي والهندسي لحزمة سياق التنفيذ القياسية `context` (`/usr/local/go/src/context`) في لغة Go (الإصدار الحديث Go 1.24+ / Go 1.27).

---

## 🎯 مقدمة ونظرة عامة

تُعد حزمة `context` من أهم الركائز الأساسية التي يعتمد عليها نموذج التزامن (Concurrency Model) والشبكات وتدفق البيانات في لغة Go الحديثة. تم ابتكار الحزمة في الأصل داخل شركة Google بواسطة Sameer Ajmani لحل معضلات إلغاء الاستعلامات الموزعة وإدارة ميزانيات الوقت، ثم أُدرجت رسمياً في المكتبة القياسية مع إطلاق Go 1.7.

يمثل كائن `context.Context` شجرة غير قابلة للتعديل (Immutable Directed Acyclic Graph) تنتقل عبر حدود الـ APIs، والـ Goroutines، والعمليات البرمجية لنقل ثلاث ركائز حيوية:

1. **إشارات الإلغاء الشلالي (Cancellation Signals):** لإشعار الـ Goroutines بالتوقف الفوري عن العمل وتفريغ الموارد.
2. **المهل الزمنية والمواعيد النهائية (Deadlines & Timeouts):** لتحديد الحد الأقصى المسموح به لعمر الطلب وميزانيات المعالجة.
3. **البيانات المقتصرة على دورة حياة الطلب (Request-Scoped Values):** لنقل المعرّفات التشخيصية والسمات الأمنية بأمان خيطي.

---

## 📚 خريطة وأقسام التوثيق

تم تقسيم هذا التوثيق المعماري الموسع إلى 5 ملفات تخصصية لتسهيل الاستيعاب والتعمق في كل جزئية:

| الملف | المحتوى ومحاور الدراسة |
| :--- | :--- |
| **[01. الفلسفة المعمارية ومبررات الوجود](file:///home/osm/StudioProjects/train_struct/docs/context/01_philosophy_and_problem_statement.md)** | • سياق النشأة التاريخية والمشاكل قبل Go 1.7.<br>• المشاكل الـ 5 الجوهرية التي تعالجها الحزمة بالتفصيل الرياضي والبرمجي (منع تسريب الـ Goroutines، الإلغاء الشلالي، حماية الخوادم من تراكم الطلبات الميتة، تتبع دورة الطلب).<br>• الفلسفة المعمارية: عدم قابلية التعديل (Immutability)، وبناء الأشجار الهرمية، والتمرير الصريح. |
| **[02. الواجهات والهياكل والأنواع الأساسية](file:///home/osm/StudioProjects/train_struct/docs/context/02_interfaces_and_core_types.md)** | • الواجهة العامة `Context` وشرح دوالها الأربع (`Deadline`, `Done`, `Err`, `Value`).<br>• الواجهات الداخلية غير المصدّرة (`canceler`, `afterFuncer`, `stringer`).<br>• الهياكل والأنواع الملموسة (`emptyCtx`, `cancelCtx`, `timerCtx`, `valueCtx`, `withoutCancelCtx`, `afterFuncCtx`, `stopCtx`).<br>• متغيرات الأخطاء الرسمية (`Canceled`, `DeadlineExceeded`). |
| **[03. الدوال العامة وإدارة دورة الحياة](file:///home/osm/StudioProjects/train_struct/docs/context/03_public_functions_and_lifecycle.md)** | • دوال الجذور: `Background()` و `TODO()`.<br>• دوال الإلغاء والأسباب: `WithCancel`, `WithCancelCause`, و `Cause`.<br>• دوال المهل الزمنية: `WithDeadline`, `WithDeadlineCause`, `WithTimeout`, `WithTimeoutCause`.<br>• دوال القيم: `WithValue`.<br>• الإضافات الحديثة (Go 1.20 / 1.21): `WithoutCancel` و `AfterFunc`.<br>• أمثلة كود تطبيقية ودقيقة لكل دالة مع شرح استدعاء `cancel()`. |
| **[04. التشريح الداخلي وآليات الأداء الفائق](file:///home/osm/StudioProjects/train_struct/docs/context/04_internals_and_performance.md)** | • آليات شجرة السياق والانتشار (Tree Propagation & Cascading).<br>• التخصيص الكسول (Lazy Allocation) للقنوات عبر `sync/atomic` لتحقيق سرعة فائقة.<br>• خوارزمية البحث الخطي المسطحة `value()` لمنع استهلاك الـ Stack والفيضان.<br>• التفكيك الذاتي ومنع التسريب عبر `removeChild`.<br>• العزل عن حزمة `fmt` لتخفيف وقت الإقلاع وحجم الثنائيات. |
| **[05. أفضل الممارسات الهندسية والأنماط المضادة](file:///home/osm/StudioProjects/train_struct/docs/context/05_best_practices_and_antipatterns.md)** | • القواعد الذهبية لمجتمع Go (قاعدة المعامل الأول، حظر التخزين في الـ Structs).<br>• إدارة المفاتيح الآمنة الخالية من التخصيص ومنع التصادم (Unexported Key Pattern).<br>• مصفوفة الأنماط المضادة (Anti-Patterns Matrix) وكيفية معالجتها.<br>• استراتيجيات الاختبارات والتوافق مع `testing.T.Context()` الحديثة في Go 1.24+. |

---

## 🏛️ نظرة هيكلية عامة على كائنات الحزمة

يوضح الرسم التالي كيفية تشابك واشتقاق كائنات السياق في حزمة `context`:

```text
                             [Context Interface]
                                      │
            ┌─────────────────────────┼─────────────────────────┐
            ▼                         ▼                         ▼
       emptyCtx                   cancelCtx                  valueCtx
      (No-op Base)             (Cancellation Root)     (Key-Value Store)
     ┌──────┴──────┐                  │
     ▼             ▼                  ▼
backgroundCtx   todoCtx            timerCtx
                               (Deadline/Timeout)
```

ومع التحديثات الحديثة (Go 1.20 و Go 1.21):

- `withoutCancelCtx`: يلتف حول أي سياق ليعزل إشارة الإلغاء مع الحفاظ الكامل على القيم (`Value()`).
- `afterFuncCtx`: يربط دالة استدعاء خلفية (`Callback`) تُنفذ تلقائياً في Goroutine منفصلة فور إلغاء السياق.

---

## 🔗 الارتباط بالمكتبة القياسية ومسار الكود المصدري

الكود المصدري الموثق يقع مباشرة في بيئة Go المحلية:

- مسار الكود: [`/usr/local/go/src/context/context.go`](file:///usr/local/go/src/context/context.go)
- حزم الاختبارات والأداء:
  - [`/usr/local/go/src/context/context_test.go`](file:///usr/local/go/src/context/context_test.go)
  - [`/usr/local/go/src/context/afterfunc_test.go`](file:///usr/local/go/src/context/afterfunc_test.go)
  - [`/usr/local/go/src/context/benchmark_test.go`](file:///usr/local/go/src/context/benchmark_test.go)
  - [`/usr/local/go/src/context/example_test.go`](file:///usr/local/go/src/context/example_test.go)
  - [`/usr/local/go/src/context/x_test.go`](file:///usr/local/go/src/context/x_test.go)
