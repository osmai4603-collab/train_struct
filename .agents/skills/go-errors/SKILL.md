---
name: go-errors
description: "Engineering and maintaining our project's custom Error infrastructure (internal/platform/errors) based on global best practices. Covers Platform Layer architecture, Ben Johnson's Failure is your Domain pattern (3 error consumers), standard machine-readable codes, logical stack traces, failure domain isolation and repository translation, safe HTTP status mapping, log/slog LogValuer integration, multi-error aggregation, and strict anti-pattern prevention."
---

# مهارة نظام الأخطاء الموحد للمشروع (Our Project's Custom Error Engineering)

تحدد هذه المهارة المعمارية القياسية الهندسية لبناء، وتطوير، وصيانة **نظام الأخطاء المخصص لمشروعنا (`internal/platform/errors`)** استناداً إلى أرقى الممارسات العالمية المعتمدة في كبرى الشركات التقنية (Google و Uber ومجتمع Go المتقدم)، ونمط Ben Johnson المعماري المشهور عالمياً (*Failure is your Domain*).

تم تصميم هذا النظام ليكون أساس التعامل مع الأخطاء عبر كافة طبقات المشروع، حيث يفصل بصرامة بين احتياجات المستهلكين الثلاثة للخطأ (التطبيق، المستخدم النهائي، المشغل)، ويضمن عدم تسريب تفاصيل البنية التحتية، ويوفر توافقاً كاملاً مع `errors.Is` و `errors.As` و `log/slog` ووسائط HTTP.

---

## بنية ملفات النظام المخصص في مشروعنا (`Project Errors Layout`)

تم بناء وتنظيم حزمة الأخطاء الخاصة بمشروعنا داخل المسار القياسي `internal/platform/errors/` وتتوزع وظائف النظام عبر الملفات التالية:

```text
train_struct/
├── internal/
│   └── platform/
│       └── errors/
│           ├── doc.go          # التوثيق المعماري الرسمي للحزمة وقواعد التبعيات
│           ├── codes.go        # الأكواد المعيارية الثابتة (INTERNAL, NOT_FOUND, INVALID, ...)
│           ├── errors.go       # بنية Error الموحدة، والتغليف، وواجهة slog.LogValuer
│           ├── helpers.go      # دوال الفحص والاستخراج (ErrorCode, ErrorMessage, FromContext)
│           ├── constructors.go # دوال الإنشاء السريعة (NotFound, Invalid, E, ...)
│           ├── validation.go   # أخطاء التحقق المهيكلة (ValidationError, FieldViolation)
│           ├── http.go         # تعيين أكواد HTTP وكتابة الردود الآمنة للمستخدم النهائي
│           └── errors_test.go  # اختبارات الوحدة الشاملة وتأكيد التوافق
│
└── .agents/skills/go-errors/   # الدليل المعماري التخصصي للمهارة
    ├── SKILL.md
    ├── docs/                   # الأدلة المعمارية والتطبيقية
    │   ├── 01_architecture_and_failure_domains.md
    │   ├── 02_error_taxonomy_and_strategies.md
    │   ├── 03_wrapping_inspection_and_join.md
    │   ├── 04_concurrency_panic_and_security.md
    │   └── 05_testing_error_handling.md
    ├── examples/               # كود Go نموذجي جاهز للاستخدام والتطبيق
    │   ├── error_construction.go
    │   ├── repository_translation.go
    │   ├── http_error_handler.go
    │   ├── validation_error.go
    │   ├── concurrent_errors.go
    │   └── errors_test.go
    └── references/             # مصفوفات الفحص والأنماط المضادة
        ├── antipatterns_matrix.md
        └── production_checklist.md
```

---

## 1. التموضع المعماري داخل المشروع: `internal/platform/errors`

في المعمارية متعددة الطبقات لمشروعنا، ينتمي هذا النظام إلى **طبقة المنصة الأساسية (Platform / Foundation Layer)**:

```text
┌────────────────────────────────────────────────────────────────────────┐
│                        Transport / Delivery Layer                      │
│                  (internal/httphandlers, gRPC Handlers)                │
│       تستخدم WriteHTTPError لتحويل الخطأ إلى JSON آمن وحالة HTTP        │
└───────────────────────────────────┬────────────────────────────────────┘
                                    │ يستدعي
                                    ▼
┌────────────────────────────────────────────────────────────────────────┐
│                    Application / Use Cases / Services                  │
│                     (internal/usecases, internal/services)             │
│       تنسق العمليات وتغلف الأخطاء مع تحديد Op والأكواد المنطقية         │
└───────────────────────────────────┬────────────────────────────────────┘
                                    │ يستدعي
                                    ▼
┌────────────────────────────────────────────────────────────────────────┐
│                        Domain Layer (Business Logic)                   │
│                     (internal/domain, entities)                        │
│       تعرف أخطاء الأعمال النقية والمستقلة عن أي تقنيات تخزين           │
└───────────────────────────────────┬────────────────────────────────────┘
                                    │ يعتمد عليه
                                    ▼
┌────────────────────────────────────────────────────────────────────────┐
│                        Persistence / Infrastructure                    │
│                      (internal/storage, database/sql)                  │
│   ترجم أخطاء SQL/Redis/gRPC إلى أخطاء نطاق وتمنع تسريب تفاصيل التخزين   │
└───────────────────────────────────┬────────────────────────────────────┘
                                    │
                                    │ يعتمد الجميع على
                                    ▼
┌────────────────────────────────────────────────────────────────────────┐
│                 Platform / Foundation Layer (طبقة المنصة)               │
│                     internal/platform/errors                           │
│                                                                        │
│  - هيكل الخطأ الموحد Error (Code, Message, Op, Err, RequestID)         │
│  - الأكواد المعيارية (NOT_FOUND, INVALID, CONFLICT, TIMEOUT, ...)      │
│  - تكامل فائق مع log/slog عبر واجهة slog.LogValuer                     │
│  - ربط تلقائي بمعرّف الطلب RequestID عبر السياق context.Context         │
│  - محول HTTP موحد لحجب تفاصيل البنية التحتية عن العميل                  │
└────────────────────────────────────────────────────────────────────────┘
```

### قواعد التبعيات الصارمة لمشروعنا

1. **انعدام التبعيات نحو الطبقات العليا:** لا تستورد `internal/platform/errors` أي حزمة من `domain` أو `usecases` أو `services` أو `storage` أو `httphandlers`.
2. **التكامل مع طبقة المنصة الصديقة:** تتكامل الحزمة مع `internal/platform/context` لاستخراج `RequestID` آلياً عند توفره.
3. **أمان الاستيراد الكامل:** يحق لكافة طبقات المشروع استيراد الحزمة تحت الاسم المستعار:

```go
import platformerr "train/internal/platform/errors"
```

---

## 2. المبادئ السبعة الحاكمة لنظام الأخطاء في مشروعنا

1. **فصل المستهلكين الثلاثة للخطأ (Ben Johnson Pattern):**
   - **التطبيق (The Application):** يحتاج كوداً برمجياً ثابتاً (`Code`) لاتخاذ قرارات التوجيه والتفرع.
   - **المستخدم النهائي (The End User):** يحتاج رسالة مهذبة (`Message`) خالية تماماً من أسماء الجداول، استعلامات SQL، أو عناوين الـ IP.
   - **المشغل والمهندس (The Operator):** يحتاج مساراً منطقياً كاملاً (`Op` و `Err` و `RequestID`) للتشخيص في السجلات.

2. **التغليف الواعي (Intentional Wrapping):**
   - استخدم `%w` أو `platformerr.E(op, err)` عندما تريد أن يكون الخطأ جزءاً من العقد البرمجي المتاح للفحص بـ `errors.Is`.
   - استخدم `%v` لحجب تفاصيل التنفيذ الداخلي التي لا يجوز كشفها خارج حدود الحزمة.

3. **عزل نطاق الفشل في طبقة التخزين (Repository Failure Domain Isolation):**
   - يُحظر خروج أخطاء المحركات مثل `sql.ErrNoRows` أو `pgconn.PgError` مباشرة للطبقات العليا.
   - يجب ترجمتها في الـ Repository إلى أخطاء نطاق معروفة مثل `platformerr.NotFound` أو `platformerr.Conflict`.

4. **مبدأ "التسجيل أو الإرجاع" (Log OR Return, Never Both):**
   - إما أن تعالج الخطأ وتسجله (Handle & Log) عند حدود النظام (مثل HTTP Handler أو Worker)، أو تغلفه وترجعه للطبقة الأعلى.
   - الجمع بينهما يؤدي إلى تلوث السجلات بتكرارات مزعجة لنفس الحادثة.

5. **الأمان الصارم وتنقية البيانات (PII & Secret Redaction):**
   - لا تضع كلمات مرور، رموز مصادقة (Tokens)، أو أرقام سرية داخل نصوص الأخطاء أو دوال `fmt.Errorf`.

6. **التوافق التام مع معايير لغة Go القياسية:**
   - رسائل الأخطاء تبدأ بحروف صغيرة (lowercase) ولا تنتهي بنقطة (`.`).
   - الفحص يتم عبر `errors.Is` و `errors.As` ويُمنع استخدام `==` لمقارنة الأخطاء.

7. **تجميع الأخطاء المتعددة وإغلاق الموارد بأمان:**
   - استخدام `errors.Join` لجمع أخطاء `defer closer.Close()` والمهام المتزامنة دون فقدان أي خطأ.

---

## 3. دليل الاستخدام عبر طبقات المشروع (Usage Across Layers)

### 1. في طبقة التخزين (`internal/storage`) — ترجمة الأخطاء

```go
func (r *UserRepository) FindByID(ctx context.Context, id string) (*domain.User, error) {
    const op = "storage.UserRepository.FindByID"

    var user domain.User
    err := r.db.QueryRowContext(ctx, "SELECT id, name FROM users WHERE id = $1", id).Scan(&user.ID, &user.Name)
    if err != nil {
        if errors.Is(err, sql.ErrNoRows) {
            // ترجمة الخطأ وحجب الـ SQL
            return nil, platformerr.NotFound(op, "user not found", err)
        }
        return nil, platformerr.Internal(op, "database query failed", err)
    }

    return &user, nil
}
```

### 2. في طبقة حالات الاستخدام (`internal/usecases`) — إضافة السياق والتحقق

```go
func (uc *RegisterUserUseCase) Execute(ctx context.Context, input RegisterInput) error {
    const op = "usecases.RegisterUser"

    // فحص المدخلات وجمع الانتهاكات
    valErr := platformerr.NewValidationError("RegisterInput")
    if input.Email == "" {
        valErr.AddViolation("email", "email is required")
    }
    if len(input.Password) < 8 {
        valErr.AddViolation("password", "password must be at least 8 characters")
    }
    if valErr.HasViolations() {
        return valErr.AsError(op)
    }

    // استدعاء المستودع مع ربط السياق
    if err := uc.repo.Create(ctx, input); err != nil {
        return platformerr.E(op, err)
    }

    return nil
}
```

### 3. في طبقة النقل (`internal/httphandlers`) — تحويل الخطأ لرد آمن

```go
func (h *UserHandler) Register(w http.ResponseWriter, r *http.Request) {
    var input RegisterInput
    if err := json.NewDecoder(r.Body).Decode(&input); err != nil {
        platformerr.WriteHTTPError(w, r, platformerr.Invalid("handler.Register", "invalid json request body", err), h.logger)
        return
    }

    if err := h.useCase.Execute(r.Context(), input); err != nil {
        // كتابة الرد التلقائي، تعيين HTTP Status المناسب، والتسجيل في slog
        platformerr.WriteHTTPError(w, r, err, h.logger)
        return
    }

    w.WriteHeader(http.StatusCreated)
}
```

---

## 4. الاعتماديات والتكاملات بين المهارات (Cross-Skill Dependencies & Integrations)

نظام الأخطاء هو حجر الأساس في طبقة المنصة، وتستند إليه باقي مهارات المنصة لضمان اتساق معالجة الأخطاء عبر كافة الطبقات:

```text
go-context   ─► go-errors        (استخراج RequestID آلياً من السياق)
go-errors    ─► go-logger-slog  (تحويل الخطأ إلى حقول مهيكلة عبر slog.LogValuer)
go-errors    ─► go-postgres     (ترجمة SQLSTATE إلى أكواد المنصة)
go-errors    ─► go-server-lifecycle (محول HTTP الآمن وربط الحالة)
go-errors    ─► go-service-configuration (نمط تجميع الأخطاء المتعددة في تقرير التحقق)
```

### المهارات التي تعتمد عليها هذه المهارة (Downstream Dependencies)

| المهارة | نوع الاعتماد | نقاط التكامل الرئيسية |
| :--- | :--- | :--- |
| `go-context` | استخراج `RequestID` و `CorrelationID` آلياً من السياق عند توفّرها | `platformctx.RequestIDFromContext(ctx)` في بنّائات الخطأ و `WriteHTTPError` لربط الخطأ بمسار الطلب |

### التكامل مع المهارات الأخرى (Upstream Integrations)

| المهارة | نوع التكامل | نقاط التكامل الرئيسية |
| :--- | :--- | :--- |
| `go-logger-slog` | `*platformerr.Error` ينفّذ `slog.LogValuer` فيتحوّل تلقائياً إلى حقول مهيكلة `code, op, message, request_id, cause` | `logger.ErrorContext(ctx, ..., slog.Any("error", err))` مع احترام مبدأ "سجل أو أرجع" |
| `go-postgres` | ترجمة أخطاء SQLSTATE إلى أخطاء المنصة داخل حدود الـ Repository | `postgres.TranslateError` يحوّل `sql.ErrNoRows → NOT_FOUND(404)`, `23505 → CONFLICT(409)`, `40001/40P01 → UNAVAILABLE(503)` |
| `go-server-lifecycle` | محول HTTP موحد يحجب تفاصيل البنية التحتية عن العميل ويلزم طبقة النقل | `platformerr.WriteHTTPError(w, r, err, logger)` لتعيين الحالة الآمنة وتسجيل الخطأ |
| `go-service-configuration` | نمط تجميع أخطاء التحقق في تقرير موحد (بأسلوب `errors.Join`) | `ValidationReport` يجمع كافة مشاكل الإعدادات ويُوقف الإقلاع بتقرير تشخيصي واحد بدلاً من أول خطأ |

---

## 5. المراجع وقوائم التدقيق

- [المعمارية وعزل نطاق الفشل](./docs/01_architecture_and_failure_domains.md)
- [تصنيف الأخطاء واستراتيجيات الاستخدام](./docs/02_error_taxonomy_and_strategies.md)
- [ميكانيكا التغليف، الفحص، و errors.Join](./docs/03_wrapping_inspection_and_join.md)
- [التزامن والذعر وأمان البيانات](./docs/04_concurrency_panic_and_security.md)
- [استراتيجيات اختبار أنظمة الأخطاء](./docs/05_testing_error_handling.md)
- [مصفوفة الأنماط المضادة وتصحيحها](./references/antipatterns_matrix.md)
- [قائمة مراجعة الجاهزية للإنتاج](./references/production_checklist.md)
