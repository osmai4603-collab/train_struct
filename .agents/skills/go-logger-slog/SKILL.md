---
name: go-logger-slog
description: "Production-grade structured logging for Go HTTP services and backend applications using log/slog. Covers frontend/backend architecture, explicit dependency injection, zero-allocation hot paths (LogAttrs & Enabled checks), context-aware HTTP middleware, custom handler pipelines, sensitive data protection (LogValuer & generic Sensitive[T]), automated testing with testing/slogtest, and anti-pattern prevention."
---

# مهارة تسجيل الأحداث المهيكل في Go (Go Structured Logging with `log/slog`)

تحدد هذه المهارة المعمارية القياسية والممارسات العالمية المعتمدة لتسجيل الأحداث المهيكل (Structured Logging) في خدمات وتطبيقات Go للإنتاج باستخدام الحزمة الرسمية `log/slog` (المدعومة ابتداءً من Go 1.21). تضمن هذه المهارة توحيد أنماط السجلات عبر الخدمات، وعزل الواجهة عن التنفيذ، والأداء الفائق بدون استهلاك إضافي للذاكرة (Zero-Allocation)، وحماية الأسرار من التسرب، وربط السجلات بالسياق الزمني والطلبات (Correlation Tracing).

---

## بنية المهارة ومحتوياتها

```text
.agents/skills/go-logger-slog/
├── SKILL.md                                  # الوثيقة المرجعية الشاملة للأنماط والقواعد
├── docs/                                     # الأدلة المعمارية التخصصية المتعمقة
│   ├── 01_architecture_and_handlers.md       # معمارية Frontend/Backend وخيارات الـ Handlers
│   ├── 02_dependency_injection_and_context.md# حقن التبعيات الصريح ونقل الـ Context
│   ├── 03_performance_and_zero_allocations.md# هندسة الأداء العالي ومسارات Hot Paths
│   ├── 04_custom_handlers_and_middleware.md  # تصميم الـ Middleware والـ Custom Handlers
│   ├── 05_security_and_sensitive_data.md     # حجب الأسرار ونمط Sensitive[T]
│   └── 06_testing_strategies.md              # استراتيجيات اختبار السجلات والامتثال لـ slogtest
├── examples/                                 # كود Go نموذجي جاهز للاستخدام
│   ├── factory.go                            # تهيئة الـ Logger حسب البيئة (JSON/Text)
│   ├── middleware.go                         # HTTP Logging Middleware مع Trace ID
│   ├── custom_handler.go                     # Handler مخصص للتنقيح وحجب البيانات
│   ├── sensitive_type.go                     # نوع آمن لحجب الأسرار عبر LogValuer
│   ├── hotpath.go                            # أداء مثالي باستخدام LogAttrs و Enabled
│   └── logger_test.go                        # اختبارات السجلات وتطبيق slogtest.TestHandler
└── references/                               # القوائم والمصفوفات المرجعية السريعة
    ├── production_checklist.md               # قائمة مراجعة الجاهزية للإنتاج
    └── antipatterns_matrix.md                # مصفوفة الأنماط المضادة وحلولها الاصطلاحية
```

---

## المبادئ الستة لتسجيل الأحداث في بيئات الإنتاج

1. **فصل الواجهة عن التنفيذ (Frontend/Backend Separation)**:
   يستخدم كود التطبيق واجهة `*slog.Logger` كـ Frontend لإرسال الأحداث، بينما يتولى `slog.Handler` كـ Backend معالجة التنسيق والإخراج (`JSONHandler` للإنتاج و `TextHandler` للتطوير المحلي).
2. **الحقن الصريح للتبعية (Explicit Dependency Injection)**:
   يُمرر `*slog.Logger` صراحةً في دوال البناء (`NewService(logger, ...)`) مع تحديد نطاق فرعي للمكون عبر `.With(slog.String("component", "..."))`، ويُحظر الاعتماد على السجل العام `slog.Default()` في الخدمات الداخلية.
3. **الربط بالسياق والمعرفات المترابطة (Context & Correlation Tracing)**:
   تُستخدم دوال السياق دائماً (`InfoContext`، `ErrorContext`، `LogAttrs`) لتمرير `context.Context`، مما يتيح استخراج `trace_id` و `request_id` وربط الأحداث بمسار تدفق الطلب عبر الشبكة.
4. **الأداء الفائق بدون تخصيص ذاكرة في المسارات الساخنة (Zero-Allocation Hot Paths)**:
   في الحلقات أو المسارات البرمجية كثيفة التكرار، يتم استبدال أزواج `key, value` غير المحددة بالدالة `slog.LogAttrs` واستخدام الأنواع الصريحة مثل `slog.String` و `slog.Int` لتجنب تخصيص الذاكرة على الـ Heap، مع فحص `logger.Enabled()` قبل الحسابات المكلفة.
5. **الأمان الصارم للبيانات الحساسة (Strict Data Redaction)**:
   لا تُسجل كلمات المرور أو الرموز السرية أو بيانات الهوية الشخصية (PII) كنصوص صريحة مطلقاً. تُغلّف في أنواع مخصصة تنفّذ واجهة `slog.LogValuer` لتظهر دائماً بصيغة `***`، أو عبر Handler مخصص لحجب المفاتيح المحددة.
6. **مبدأ "التسجيل أو الإرجاع" (Log OR Return, Never Both)**:
   الخطأ إما أن يُسجل ويُعالج في مكانه، أو يُغلّف (`fmt.Errorf("...: %w", err)`) ويُعاد للمستدعي ليسجله في الطبقة العليا (Top Layer). يُحظر التسجيل والإرجاع معاً لتجنب التكرار المزعج في السجلات.

---

## المعمارية: فصل Frontend عن Backend

```text
┌─────────────────────────────────────────────────────────────────┐
│                       Application Code                          │
│        logger.InfoContext(ctx, "msg", "key", val)                │
│        logger.LogAttrs(ctx, LevelInfo, "msg", Attr...)          │
└────────────────────────────────┬────────────────────────────────┘
                                 │
                        ┌────────▼────────┐
                        │   slog.Logger   │  ◄── Frontend (API)
                        │   (واجهة العميل) │
                        └────────┬────────┘
                                 │
                        ┌────────▼────────┐
                        │  slog.Handler   │  ◄── Backend (Interface)
                        │    (التنفيذ)    │
                        └────────┬────────┘
                                 │
           ┌─────────────────────┼─────────────────────┐
           ▼                     ▼                     ▼
     JSONHandler            TextHandler          Custom Handler
   (بيئات الإنتاج)        (التطوير المحلي)    (Redaction / Routing)
```

راجع الدليل التفصيلي: [01_architecture_and_handlers.md](./docs/01_architecture_and_handlers.md)

---

## القواعد والتوجيهات السريعة (Quick Rules)

### 1. التهيئة والحقن (Initialization & DI)

- في `main.go`، يتم بناء `slog.Handler` وفق متغيرات البيئة (`ENV=production` تستخدم JSON، وغير ذلك يستخدم Text).
- يتم إنشاء الـ Logger الأساسي وتمريره للخدمات عبر نمط البناء:

  ```go
  svc := user.NewService(logger, db)
  ```

- راجع: [examples/factory.go](./examples/factory.go) و [docs/02_dependency_injection_and_context.md](./docs/02_dependency_injection_and_context.md).

### 2. تسجيل الأحداث داخل المكونات (Component Logging)

- خصص لكل مكوّن logger فرعي يحمل اسمه:

  ```go
  compLogger := baseLogger.With(slog.String("component", "OrderService"))
  ```

- استخدم دوال `Context` دائماً لربط السجل بسياق الطلب:

  ```go
  compLogger.InfoContext(ctx, "order created", "order_id", orderID)
  ```

### 3. الأداء والمسارات الساخنة (Performance & Hot Paths)

- استبدل:

  ```go
  // ❌ بطيء: يسبب heap allocation بسبب interface{}
  logger.Info("processed", "count", n, "rate", r)
  ```

- بـ:

  ```go
  // ✅ فائق السرعة: zero-allocation
  logger.LogAttrs(ctx, slog.LevelInfo, "processed",
      slog.Int("count", n),
      slog.Float64("rate", r),
  )
  ```

- افحص الجاهزية قبل العمليات الثقيلة:

  ```go
  if logger.Enabled(ctx, slog.LevelDebug) {
      logger.DebugContext(ctx, "heavy diagnostics", "dump", computeStateDump())
  }
  ```

- راجع: [examples/hotpath.go](./examples/hotpath.go) و [docs/03_performance_and_zero_allocations.md](./docs/03_performance_and_zero_allocations.md).

### 4. حماية الأسرار (Sensitive Data Protection)

- أي نوع يحتوي على سر (كلمة مرور، مفتاح API، رمز JWT) يجب أن ينفذ واجهة `slog.LogValuer`:

  ```go
  type SecretString string
  func (s SecretString) LogValue() slog.Value {
      return slog.StringValue("***")
  }
  ```

- أو باستخدام الحاوية العامة (Generic Container) `Sensitive[T]`:

  ```go
  token := sensitive.New("super-secret-token")
  logger.Info("client authenticated", "token", token) // يظهر token="***"
  ```

- راجع: [examples/sensitive_type.go](./examples/sensitive_type.go) و [docs/05_security_and_sensitive_data.md](./docs/05_security_and_sensitive_data.md).

### 5. وسيط الـ HTTP والتتبع (HTTP Middleware & Tracing)

- يقوم Middleware الطلبات بإنشاء معرف ترابط فريد (`request_id`)، وتضمين Logger محدد النطاق في سياق الطلب `r.Context()`، وتسجيل زمن المعالجة وحالة الاستجابة بعد انتهائها.
- راجع: [examples/middleware.go](./examples/middleware.go) و [docs/04_custom_handlers_and_middleware.md](./docs/04_custom_handlers_and_middleware.md).

### 6. التحقق والاختبار (Testing & slogtest)

- للتحقق من مخرجات السجلات في اختبارات الوحدة، استخدم `bytes.Buffer` مع `slog.NewJSONHandler(&buf, nil)`.
- للتحقق من توافق أي Handler مخصص مع متطلبات Go القياسية، شغّل الفحص الإلزامي `slogtest.TestHandler`.
- راجع: [examples/logger_test.go](./examples/logger_test.go) و [docs/06_testing_strategies.md](./docs/06_testing_strategies.md).

### 7. التكامل مع نظام الأخطاء الموحد (`platformerr`)

- يتكامل `log/slog` تلقائياً مع نظام أخطاء المشروع (`internal/platform/errors` / مهارة `go-errors`) من خلال تنفيذ واجهة `slog.LogValuer` في `*platformerr.Error`.
- عند تسجيل خطأ عبر:

  ```go
  logger.ErrorContext(ctx, "operation failed", slog.Any("error", err))
  ```

  يقوم `log/slog` تلقائياً بتحويل الخطأ إلى مجموعة حقول مهيكلة (`code`, `op`, `message`, `request_id`, `cause`) دون أي جهد إضافي، مع احترام قاعدة "سجل أو أرجع" (Log OR Return, Never Both).
- راجع مهارة الأخطاء التخصصية: [go-errors](../go-errors/SKILL.md).

---

## الاعتماديات والتكاملات بين المهارات (Cross-Skill Dependencies & Integrations)

تشكّل مهارة التسجيل نقطة تقاطع مع باقي مهارات المنصة: تستمد بيانات الترابط من السياق، وتستهلك نظام الأخطاء لعرض الحقول المهيكلة، وتخدم طبقات الإقلاع والتخزين والخادم:

```text
go-context          ─► go-logger-slog     (request_id / correlation_id / tenant_id من السياق)
go-errors           ─► go-logger-slog     (slog.LogValuer → حقول مهيكلة عند تسجيل الأخطاء)
go-logger-slog      ─► go-postgres        (تسجيل تهيئة المجمع والفحوصات والمقاييس)
go-logger-slog      ─► go-server-lifecycle (سجلات متعددة الأهداف، توجيه حسب حالة HTTP، كبت فحوصات الصحة)
go-logger-slog      ─► go-service-configuration (تسجيل آمن للتكوين المحجوب Redacted)
```

### المهارات التي تعتمد عليها هذه المهارة (Downstream Dependencies)

| المهارة | نوع الاعتماد | نقاط التكامل الرئيسية |
| :--- | :--- | :--- |
| `go-context` | كل دوال `*Context` تستخرج معرّفات الترابط من سياق الطلب الذي يبنيه `platformctx.Middleware` | `InfoContext/ErrorContext/LogAttrs` تحمل `request_id`, `correlation_id`, `tenant_id` تلقائياً |
| `go-errors` | تسجيل الأخطاء كحقول مهيكلة بدلاً من نصوص خام، مع احترام مبدأ "سجل أو أرجع" | `slog.Any("error", platformerr)` عبر واجهة `LogValuer` |

### التكامل مع المهارات الأخرى (Upstream Integrations)

| المهارة | نوع التكامل | نقاط التكامل الرئيسية |
| :--- | :--- | :--- |
| `go-server-lifecycle` | تهيئة لوجر متعدد الأهداف (طرفية ملوّنة + ملفات JSON دوّارة)، توجيه 5xx → ERROR و 4xx → WARN، كبت فحوصات `/livez` و `/readyz`، ومزامنة الملفات في Phase 8 | `initLogger()`, `isatty`, `<PREFIX>_LOG_COLOR`, `logs/app.log`, `logs/error.log` |
| `go-postgres` | تسجيل تهيئة المجمع ومقاييسه وفحوصات جاهزيته بلوغرات فرعية تحمل `component=postgres` | `logger.With(slog.String("component", "postgres"))` |
| `go-service-configuration` | ضمان عدم تسريب الأسرار عند تسجيل التكوين | `engine.Redact(cfg)` ثم `logger.Info("config loaded", "config", redacted)` |

---

## المراجع وقوائم التدقيق

- [مهارة الأخطاء الموحدة للمشروع (go-errors)](../go-errors/SKILL.md)
- [قائمة مراجعة الجاهزية للإنتاج](./references/production_checklist.md)
- [مصفوفة الأنماط المضادة وتصحيحها](./references/antipatterns_matrix.md)
