# الدليل المعماري 04: ترحيل المخططات وفحص الصحة والمقاييس

## 1. إدارة ترحيل المخططات المضمنة (`Embedded Migrations`)

في التطبيقات الحديثة المعدة للنشر عبر الحاويات (Docker / Kubernetes)، يُفضل تضمين ملفات ترحيل SQL مباشرة داخل الملف الثنائي المترجم للخدمة (Binary) عبر مكتبة Go القياسية `embed`.

### المزايا

- **نشر ثنائي موحد (Single Self-Contained Binary)**: لا داعي لنسخ مجلدات ملفات SQL الخارجية إلى صورة Docker أو القلق من فقدانها.
- **تزامن الشفرة والمخطط (Code & Schema Synchronization)**: عند بدء تشغيل الإصدار الجديد من الخدمة، يتم تطبيق أي جداول أو أعمدة جديدة تلقائياً قبل استقبال أي طلبات.
- **تكامل مع نفس المجمع**: تستخدم الدالة `stdlib.OpenDBFromPool(pool)` لإعادة استخدام نفس اتصالات المجمع المفتوحة لعمليات الترحيل بدلاً من فتح اتصالات إضافية.

### بنية ملفات الترحيل القياسية

تُسمى ملفات الترحيل بتسلسل زمني ثابت (Timestamp UTC) متبوعاً باسم العملية واتجاهها:

```text
migrations/
├── 20260921000001_create_users_table.up.sql
├── 20260921000001_create_users_table.down.sql
├── 20260921000002_create_orders_table.up.sql
└── 20260921000002_create_orders_table.down.sql
```

### استخدام `postgres.RunMigrations`

```go
package main

import (
    "embed"
    "train/internal/infrastructure/postgres"
)

//go:embed migrations/*.sql
var migrationFiles embed.FS

func run(ctx context.Context) error {
    pool, err := postgres.NewPool(ctx, cfg, logger)
    if err != nil {
        return err
    }
    defer postgres.ClosePool(pool, logger)

    // تشغيل الترحيلات التلقائي
    if err := postgres.RunMigrations(ctx, pool, migrationFiles, "migrations", logger); err != nil {
        return err
    }
    
    // بدء خادم HTTP...
    return nil
}
```

---

## 2. فحص صحة قاعدة البيانات (`Liveness & Readiness Probing`)

توفر الحزمة كائن `postgres.HealthChecker` لفحص نبضات القلب وللربط بنقاط فحص الصحة لمراقبي الحاويات (Kubernetes Probes):

```go
checker := postgres.NewHealthChecker(pool)

// نقطة فحص الجاهزية /readyz
http.HandleFunc("/readyz", func(w http.ResponseWriter, r *http.Request) {
    if err := checker.Check(r.Context()); err != nil {
        http.Error(w, "database unavailable", http.StatusServiceUnavailable)
        return
    }
    w.WriteHeader(http.StatusOK)
    w.Write([]byte("ready"))
})
```

---

## 3. جمع مقاييس المجمع اللحظية (`Pool Telemetry & Stats`)

تتيح الدالة `checker.Stats()` استخراج لقطة سريعة لحالة المجمع وتطبيق واجهة `slog.LogValuer` لتسجيل المقاييس بصيغة مهيكلة أو إرسالها لـ Prometheus:

```go
stats := checker.Stats()

logger.InfoContext(ctx, "database telemetry snapshot",
    slog.Any("pool_stats", stats),
)
```

المقاييس المتوفرة:

- `TotalConns`: إجمالي عدد الاتصالات التي يديرها المجمع حالياً.
- `AcquiredConns`: عدد الاتصالات المستعارة المشغولة حالياً بتنفيذ استعلامات.
- `IdleConns`: عدد الاتصالات الخاملة الجاهزة في المجمع لاستقبال العمليات.
- `MaxConns`: الحد الأقصى المسموح به للمجمع.
- `EmptyAcquireCount`: عدد المرات التي طلب فيها استعلام اتصالاً وكان المجمع ممتلئاً بالكامل مما اضطره للانتظار (**مؤشر رئيسي على الحاجة لزيادة MaxConns أو تحسين فترات الاستعلام**).
- `AcquireDuration`: إجمالي الوقت المستغرق في انتظار تحرير الاتصالات.
