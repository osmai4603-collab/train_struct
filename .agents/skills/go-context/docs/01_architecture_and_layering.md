# المعمارية وموقع طبقة السياق (`internal/platform/context`)

## 1. موضع السياق في المعمارية النظيفة (Clean / Hexagonal Architecture)

في المشاريع الكبيرة المبنية بلغة Go، يُعد مفهوم **السياق (`context.Context`)** كائناً عابراً للنطاقات (Cross-Cutting Concern). ينتقل السياق عمودياً عبر جميع الطبقات، بدءاً من نقطة وصول طلب الشبكة إلى نقطة تخزين البيانات في القرص أو قاعدة البيانات.

ولكن، أين يجب أن تُوضع الأدوات المساعدة، والمفاتيح الآمنة، وكائنات تجميع البيانات السياقية؟

**الموقع المعياري المعتمد هو: `internal/platform/context`**

```text
┌────────────────────────────────────────────────────────────────────────┐
│                        Transport / Delivery Layer                      │
│                (cmd/api, internal/httphandlers, gRPC, MQ)              │
│                                                                        │
│  - يستقبل طلب الـ HTTP / gRPC                                           │
│  - يستخرج المعرّفات من الترويسات (Headers / Metadata)                   │
│  - يحقن RequestMetadata في السياق عبر internal/platform/context        │
│  - يمرر السياق إلى طبقة حالات الاستخدام (Use Cases)                     │
└───────────────────────────────────┬────────────────────────────────────┘
                                    │
                                    ▼
┌────────────────────────────────────────────────────────────────────────┐
│                      Application / Use Cases Layer                     │
│                        (internal/usecases, services)                   │
│                                                                        │
│  - يستقبل ctx كأول معامل في كل دالة                                    │
│  - يدير ميزانية الوقت (Timeout Budgeting) للعمليات المعقدة            │
│  - يراقب إلغاء السياق (ctx.Done) أثناء العمليات الحسابية               │
│  - يستدعي المستودعات والخدمات الخارجية ممرراً نفس السياق                 │
└───────────────────────────────────┬────────────────────────────────────┘
                                    │
                                    ▼
┌────────────────────────────────────────────────────────────────────────┐
│                    Persistence / Infrastructure Layer                  │
│                     (internal/storage, database/sql)                   │
│                                                                        │
│  - ينفذ العمليات المقيدة بالسياق: QueryContext, ExecContext, BeginTx   │
│  - يستفيد من إلغاء السياق لإلغاء الاستعلام في محرك قاعدة البيانات فوراً │
└───────────────────────────────────┬────────────────────────────────────┘
                                    │
                                    │ يعتمد الجميع على
                                    ▼
┌────────────────────────────────────────────────────────────────────────┐
│                 Platform / Foundation Layer (طبقة المنصة)               │
│                     internal/platform/context                          │
│                                                                        │
│  1. المفاتيح الآمنة غير المصدّرة (Unexported Context Keys)             │
│  2. كائن تجميع سمات الطلب (RequestMetadata Bundle)                     │
│  3. أدوات فصل المهام الخلفية (context.WithoutCancel Helpers)          │
│  4. أدوات حساب ميزانية المهل المتبقية (RemainingDeadline)              │
│  5. دوال الإيقاف السلس وحصاد الإشارات (Graceful Shutdown)              │
└────────────────────────────────────────────────────────────────────────┘
```

---

## 2. قواعد التبعيات الصارمة (Strict Dependency Rules)

لتفادي أي حلقات استيراد دائرية (Circular Dependency Cycles) والحفاظ على نقاء التصميم:

1. **انعدام التبعيات نحو الطبقات العليا (Zero Upward Imports):**
   - حزمة `internal/platform/context` **لا تستورد** أي حزمة من `internal/domain` أو `internal/usecases` أو `internal/services` أو `internal/storage` أو `internal/httphandlers`.
   - الاعتماد يقتصر كلياً على مكتبات Go القياسية: `context` و `time` و `errors` و `os/signal`.

2. **التدفق وحيد الاتجاه (Unidirectional Flow):**
   - يمكن لكافة الطبقات العليا استيراد `internal/platform/context` بأمان تام.
   - مثال: طبقة `internal/httphandlers` تستورد الحزمة لإنشاء وحقن المعرفات، وطبقة `internal/storage` تستورد الحزمة لاستخراج معرّف المستأجر (`tenant_id`) للاستعلامات متعددة المستأجرين (Multi-Tenancy).

---

## 3. محتويات ومسؤوليات حزمة `internal/platform/context`

تحتوي حزمة `internal/platform/context` على المكونات الأساسية التالية:

| المكون | الوصف المعماري |
|:---|:---|
| **`RequestMetadata`** | بنية معطيات موحدة تجمع سمات الطلب (`RequestID`، `TenantID`، `UserID`، `CorrelationID`) لحقنها في عقدة واحدة بدل عدة عقد $O(N)$. |
| **`Unexported Keys`** | تعريف أنواع هياكل فارغة (`type contextKey struct{}`) لمنع تصادم المفاتيح بين الحزم. |
| **`Accessors` (Getters/Setters)** | دوال صريحة وآمنة نوعياً مثل `WithRequestMetadata` و `MetadataFromContext` و `WithRequestID` و `RequestIDFromContext`. |
| **`Detach` / `WithoutCancel`** | دالة مساعدة لإنشاء سياق مستقل للمهام الخلفية مع الحفاظ على سمات التتبع وإضافة مهلة زمنية مستقلة. |
| **`RemainingDeadline`** | دالة لفحص الوقت المتبقي قبل انتهاء المهلة لمساعدة طبقة الـ Use Cases في اتخاذ قرارات التراجع (Fail-Fast). |

---

## 4. مثال على التفاعل الطبقي المتكامل

### أ. في طبقة النقل (`internal/httphandlers`):
```go
package httphandlers

import (
    "net/http"
    
    platformctx "train/internal/platform/context"
)

func CorrelationMiddleware(next http.Handler) http.Handler {
    return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
        reqID := r.Header.Get("X-Request-ID")
        if reqID == "" {
            reqID = generateUUID()
        }

        meta := &platformctx.RequestMetadata{
            RequestID: reqID,
            TenantID:  r.Header.Get("X-Tenant-ID"),
        }

        // حقن البيانات في سياق الطلب عبر طبقة المنصة
        ctx := platformctx.WithRequestMetadata(r.Context(), meta)
        w.Header().Set("X-Request-ID", reqID)

        next.ServeHTTP(w, r.WithContext(ctx))
    })
}
```

### ب. في طبقة الخدمات وحالات الاستخدام (`internal/usecases`):
```go
package usecases

import (
    "context"
    "fmt"
    "time"
    
    platformctx "train/internal/platform/context"
)

type OrderUseCase struct {
    repo OrderRepository
}

func (uc *OrderUseCase) Execute(ctx context.Context, cmd CreateOrderCommand) error {
    // 1. التحقق من بقاء وقت كافٍ
    if remaining, ok := platformctx.RemainingDeadline(ctx); ok && remaining < 100*time.Millisecond {
        return fmt.Errorf("timeout budget exhausted: %w", context.DeadlineExceeded)
    }

    // 2. استخراج معرّف الطلب للسجلات دون كسر أمان الأنواع
    reqID, _ := platformctx.RequestIDFromContext(ctx)
    
    // 3. تمرير السياق للمستودع
    return uc.repo.Save(ctx, cmd)
}
```

### ج. في طبقة التخزين (`internal/storage`):
```go
package storage

import (
    "context"
    "database/sql"
    
    platformctx "train/internal/platform/context"
)

type PostgresOrderRepository struct {
    db *sql.DB
}

func (r *PostgresOrderRepository) Save(ctx context.Context, cmd CreateOrderCommand) error {
    // استخراج معرّف المستأجر لعزل البيانات
    meta, _ := platformctx.MetadataFromContext(ctx)
    tenantID := meta.TenantID

    query := "INSERT INTO orders (id, tenant_id, total) VALUES ($1, $2, $3)"
    _, err := r.db.ExecContext(ctx, query, cmd.ID, tenantID, cmd.Total)
    return err
}
```
