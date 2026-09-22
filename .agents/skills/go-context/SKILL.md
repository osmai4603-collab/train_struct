---
name: go-context
description: "Engineering and maintaining our project's custom Context infrastructure (internal/platform/context) based on global best practices. Covers Platform Layer architecture, typed RequestMetadata bundling, zero-allocation unexported keys, HTTP correlation middleware, safe detached background execution (WithoutCancel & panic recovery), deadline budgeting, and strict separation from struct states."
---

# مهارة نظام السياق المخصص للمشروع (Our Project's Custom Context Engineering)

تحدد هذه المهارة المعمارية القياسية الهندسية لبناء، وتطوير، وصيانة **نظام السياق المخصص لمشروعنا (`internal/platform/context`)** استناداً إلى أرقى الممارسات العالمية المعتمدة في كبرى الشركات التقنية (Google و Uber ومجتمع Go المتقدم).

تم تصميم هذا النظام ليكون حجر الزاوية المشترك لكافة طبقات المشروع، حيث يضمن توحيد سمات الطلبات، ومنع تسريب الـ Goroutines والذاكرة، وإدارة ميزانيات المهل الزمنية، وتوفير وسيط HTTP موحد، وتشغيل المهام الخلفية بأمان فائق دون أي اختناقات في الأداء.

---

## بنية ملفات النظام المخصص في مشروعنا (`Project Context Layout`)

تم بناء وتنظيم حزمة السياق الخاصة بمشروعنا داخل المسار القياسي `internal/platform/context/` وتتوزع وظائف النظام عبر الملفات التالية:

```text
train_struct/
├── internal/
│   └── platform/
│       └── context/
│           ├── doc.go          # التوثيق المعماري الرسمي للحزمة وقواعد التبعيات
│           ├── metadata.go     # كائن تجميع سمات الطلب (RequestMetadata) والمفاتيح الآمنة
│           ├── context.go      # محرك إدارة المهل، وفصل السياق (Detach)، والتشغيل الآمن (RunDetached)
│           ├── middleware.go   # وسيط HTTP لاستخراج الترويسات وحقن السياق في الطلبات الواردة
│           └── context_test.go # اختبارات الوحدة الشاملة وتأكيد التوافق مع t.Context()
│
└── .agents/skills/go-context/  # الدليل المعماري التخصصي للمهارة
    ├── SKILL.md
    ├── docs/                   # الأدلة النظرية والتطبيقية
    │   ├── 01_architecture_and_layering.md
    │   ├── 02_api_design_and_propagation.md
    │   ├── 03_cancellation_and_goroutine_leaks.md
    │   ├── 04_timeouts_and_deadline_budgeting.md
    │   ├── 05_values_and_type_safety.md
    │   ├── 06_ecosystem_and_background_tasks.md
    │   └── 07_testing_strategies.md
    ├── examples/               # أمثلة معيارية مستقلة
    └── references/             # مصفوفات الفحص والأنماط المضادة
        ├── antipatterns_matrix.md
        └── production_checklist.md
```

---

## 1. التموضع المعماري داخل المشروع: `internal/platform/context`

في المعمارية متعددة الطبقات لمشروعنا، ينتمي هذا النظام إلى **طبقة المنصة الأساسية (Platform / Foundation Layer)**:

```text
┌────────────────────────────────────────────────────────────────────────┐
│                        Transport / Delivery Layer                      │
│                  (internal/httphandlers, gRPC Handlers)                │
│       تطبق platformctx.Middleware لاستخراج الترويسات وحقن البيانات     │
└───────────────────────────────────┬────────────────────────────────────┘
                                    │ يستدعي
                                    ▼
┌────────────────────────────────────────────────────────────────────────┐
│                    Application / Use Cases / Services                  │
│                     (internal/usecases, internal/services)             │
│   يمرر ctx كأول معامل، ويفحص ميزانية الوقت عبر RequireMinimumBudget    │
└───────────────────────────────────┬────────────────────────────────────┘
                                    │ يستدعي
                                    ▼
┌────────────────────────────────────────────────────────────────────────┐
│                        Persistence / Storage Layer                     │
│                      (internal/storage, database/sql)                  │
│    يستخرج TenantID لعزل البيانات، وينفذ استعلامات *Context القابلة للإلغاء│
└───────────────────────────────────┬────────────────────────────────────┘
                                    │
                                    │ يعتمد الجميع على
                                    ▼
┌────────────────────────────────────────────────────────────────────────┐
│                 Platform / Foundation Layer (طبقة المنصة)               │
│                     internal/platform/context                          │
│                                                                        │
│  - تجميع سمات الطلب (RequestMetadata: ReqID, CorrID, TenantID, UserID) │
│  - وسيط HTTP Middleware لاستخراج وحقن الترويسات تلقائياً              │
│  - أدوات فصل المهام غير المتزامنة بأمان (Detach & RunDetached)          │
│  - حساب وإدارة ميزانيات المهل الزمنية (RemainingDeadline)             │
└────────────────────────────────────────────────────────────────────────┘
```

### قواعد التبعيات الصارمة لمشروعنا

1. **انعدام التبعيات نحو الطبقات العليا:** لا تستورد `internal/platform/context` أي بكج من `domain` أو `usecases` أو `services` أو `storage` أو `httphandlers`.
2. **الاعتماد على المكتبة القياسية حصراً:** تعتمد الحزمة على `context`, `net/http`, `time`, `errors`, `crypto/rand`.
3. **أمان الاستيراد الكامل:** يحق لكافة طبقات المشروع استيراد الحزمة تحت الاسم المستعار:

   ```go
   import platformctx "train/internal/platform/context"
   ```

---

## 2. المكونات الأساسية لنظام السياق الخاص بمشروعنا

### أ. كائن تجميع سمات الطلب الموحد (`metadata.go`)

لمنع تكوين شجرة بحث خطية بطيئة بقيمة $O(N)$ ناتجة عن تكرار استدعاء `context.WithValue`، يوفر نظامنا هيكل بيانات جامع:

```go
type RequestMetadata struct {
    RequestID     string `json:"request_id,omitempty"`
    CorrelationID string `json:"correlation_id,omitempty"`
    TenantID      string `json:"tenant_id,omitempty"`
    UserID        string `json:"user_id,omitempty"`
    ClientIP      string `json:"client_ip,omitempty"`
}
```

- **المفاتيح غير المصدّرة (Unexported Keys):** استخدام `type metadataKey struct{}` يمنع أي حزم خارجية من استبدال أو قراءة البيانات عشوائياً.
- **الحفاظ على عدم قابلية التعديل (Immutability):** عند تعديل أي خاصية (مثل `WithRequestID`) يتم إنشاء نسخة سطحية آمنة (Shallow Copy) لضمان عدم تأثر السياق الأب.

### ب. وسيط الـ HTTP القياسي للمشروع (`middleware.go`)

يقوم `platformctx.Middleware` بأتمتة المهام التالية عند بوابة الدخول:

1. قراءة أو توليد `X-Request-ID` فريد (32 حرف سداسي عشري آمن).
2. قراءة أو ضبط `X-Correlation-ID`.
3. استخراج `X-Tenant-ID` لعزل بيانات المستأجرين.
4. حقن `RequestMetadata` في سياق الطلب `r.Context()`.
5. إعادة كتابة المعرّفات في ترويسات الاستجابة للعميل لمطابقة السجلات بسهولة.

### ج. محرك المهام الخلفية الآمن (`RunDetached`)

عند الحاجة لتنفيذ مهمة غير متزامنة بعد إرسال الرد للعميل (كإرسال بريد أو تدقيق أمني)، يحظر تمرير `r.Context()` لأنه يُلغى فوراً. يوفر نظامنا الدالة:

```go
platformctx.RunDetached(ctx, 15*time.Second, func(detachedCtx stdctx.Context) error {
    // 1. يحتفظ ببيانات التتبع و Trace ID و Tenant ID
    // 2. معزول تماماً عن إلغاء سياق طلب الـ HTTP
    // 3. مقيد بمهلة 15 ثانية مستقلة لمنع التعليق
    // 4. محمي تلقائياً من انهيار التطبيق (Panic Recovery)
    return auditService.Record(detachedCtx, order)
}, func(err error) {
    logger.Error("async audit failed", "error", err)
})
```

### د. ميزانية المهل الزمنية والتحقق الاستباقي (`budget.go`)

يوفر نظامنا أدوات لمنع هدر الموارد والـ Fail-Fast السريع:

- `RemainingDeadline(ctx)`: يعيد المدة المتبقية بدقة.
- `HasSufficientBudget(ctx, minDuration)`: يتحقق مما إذا كان الوقت المتبقي كافياً.
- `RequireMinimumBudget(ctx, minDuration)`: يرجع الخطأ الصريح `ErrInsufficientBudget` إذا لم يتبقَ وقت كافٍ للشروع في العملية.

---

## 3. دليل الاستخدام عبر طبقات المشروع (Usage Across Layers)

### 1. في طبقة النقل (`internal/httphandlers`)

```go
// تفعيل الوسيط على مسارات الـ HTTP
r := http.NewServeMux()
handler := platformctx.Middleware(r)
```

### 2. في طبقة حالات الاستخدام (`internal/usecases`)

```go
func (uc *CheckoutUseCase) Execute(ctx context.Context, cartID string) error {
    // فحص الميزانية: إذا بقي أقل من 200ms، لا تبدأ عملية الدفع
    if err := platformctx.RequireMinimumBudget(ctx, 200*time.Millisecond); err != nil {
        return fmt.Errorf("checkout aborted: %w", err)
    }

    reqID, _ := platformctx.RequestIDFromContext(ctx)
    // تمرير السياق للمستودعات
    return uc.repo.SaveOrder(ctx, ...)
}
```

### 3. في طبقة التخزين (`internal/storage`)

```go
func (r *PostgresRepo) Find(ctx context.Context, id string) (*Order, error) {
    // استخراج معرّف المستأجر لعزل البيانات
    tenantID, _ := platformctx.TenantIDFromContext(ctx)
    
    // استعلام مقيد بالسياق لقطع الاتصال فور إلغاء العميل
    return r.db.QueryRowContext(ctx, "SELECT ... WHERE id = $1 AND tenant_id = $2", id, tenantID)
}
```

---

## 4. المبادئ الحاكمة لكود المشروع

1. **`ctx` هو المعامل الأول دائماً:** `func Do(ctx context.Context, ...) error`.
2. **حظر تخزين السياق في الـ Structs:** لا يُخزن في أي Service أو Repo.
3. **حظر تمرير `nil`:** استبداله بـ `context.Background()` في البدايات أو `context.TODO()` مؤقتاً.
4. **حتمية استدعاء `defer cancel()`:** فور إنشاء أي سياق بمهلة زمنية.
5. **الاستماع الدائم لـ `<-ctx.Done()` في الحلقات:** لمنع تسريب الـ Goroutines.
6. **الاعتماد الحصري على `platformctx` لبيانات السياق:** منع اختراع مفاتيح نصية عشوائية.

---

## الاعتماديات والتكاملات بين المهارات (Cross-Skill Dependencies & Integrations)

نظام السياق هو حجر الزاوية في طبقة المنصة: لا يعتمد على أي مهارة مشروع أخرى (قاعدة التبعية 2 تلزمه بالمكتبة القياسية حصراً)، بينما تعتمد عليه جميع مهارات المنصة الباقية:

```text
go-context   ─► go-errors        (استخراج RequestID لربط الأخطاء بمسار الطلب)
go-context   ─► go-logger-slog  (request_id / correlation_id / tenant_id لدوال *Context)
go-context   ─► go-postgres     (تمرير ctx وميزانية المهلة وإلغاء الاستعلامات)
go-context   ─► go-server-lifecycle (signal.NotifyContext، shutdown المحدود، تشغيل الـ workers عبر RunDetached)
go-context   ─► go-service-configuration (ميزانيات المهل المشتقة من قيم التكوين)
```

### المهارات التي تعتمد عليها هذه المهارة (Downstream Dependencies)

لا تعتمد `internal/platform/context` على أي مهارة مشروع أخرى؛ قاعدتها الحاكمة تلزمها بالمكتبة القياسية حصراً (`context`, `net/http`, `time`, `errors`, `crypto/rand`).

### التكامل مع المهارات الأخرى (Upstream Integrations)

| المهارة | نوع التكامل | نقاط التكامل الرئيسية |
| :--- | :--- | :--- |
| `go-errors` | `platformerr` يستخرج `RequestID` آلياً من السياق عند توفره | `platformctx.RequestIDFromContext(ctx)` |
| `go-logger-slog` | دوال `*Context` تلتقط معرّفات الترابط من الطلب | `logger.InfoContext/ErrorContext/LogAttrs` |
| `go-postgres` | `ctx` يمرر كأول معامل عبر واجهة `DBTX` ويقطع الاتصال فور الإلغاء | `RequireMinimumBudget`, `QueryRowContext`, `ExecContext` |
| `go-server-lifecycle` | إدارة الإشارات والمهل الزمنية وتنفيذ المهام الخلفية المعزولة بأمان | `signal.NotifyContext`, `context.WithTimeout`, `RunDetached` (مع panic recovery) |
| `go-service-configuration` | قيم المهل الزمنية في التكوين تغذي حسابات ميزانيات المهلة | `ShutdownTimeout`, `ReadTimeout`, `DrainDuration` |

---

## المراجع وقوائم التدقيق

- [مصفوفة الأنماط المضادة وتصحيحها](./references/antipatterns_matrix.md)
- [قائمة مراجعة الجاهزية للإنتاج](./references/production_checklist.md)
- [التفاصيل المعمارية لطبقة المنصة](./docs/01_architecture_and_layering.md)
