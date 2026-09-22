# الدليل المعماري 05: استراتيجيات الاختبار واختبارات التكامل

## 1. هرم الاختبار لطبقة التخزين (Testing Pyramid)

تتطلب طبقة التخزين مستويين أساسيين من الاختبارات:
1. **اختبارات الوحدة المنطقية (Unit Tests)**:
   - سريعة جداً (ملي ثوانٍ).
   - لا تتطلب تشغيل حاوية أو خادم خارجي.
   - تختبر: دوال `TranslateError` لجميع قيم SQLSTATE، واختبار التراجع الأسي `WithRetry` مع محاكاة الأخطاء، وفحص صحة كائنات التكوين `Config`.
2. **اختبارات التكامل الحقيقية (Integration Tests with Testcontainers)**:
   - تشغل حاوية Docker حقيقية لمحرك PostgreSQL.
   - تختبر: صحة شفرات استعلامات SQL، عمل القيود الفريدة والمفاتيح الأجنبية، تنفيذ المعاملات، ودقة ترحيلات المخطط.

---

## 2. اختبارات التكامل باستخدام `Testcontainers-Go`

أفضل ممارسة عالمية هي استخدام `testcontainers-go` لتشغيل حاوية PostgreSQL سريعة مخصصة للاختبار:

```go
package storage_test

import (
    "context"
    "testing"
    "time"

    "train/internal/infrastructure/postgres"
    "train/internal/storage/user"

    "github.com/testcontainers/testcontainers-go"
    tcpostgres "github.com/testcontainers/testcontainers-go/modules/postgres"
    "github.com/testcontainers/testcontainers-go/wait"
)

func setupTestDB(t *testing.T) (*postgres.Config, func()) {
    t.Helper()
    ctx := context.Background()

    pgContainer, err := tcpostgres.Run(ctx,
        "postgres:16-alpine",
        tcpostgres.WithDatabase("test_db"),
        tcpostgres.WithUsername("test_user"),
        tcpostgres.WithPassword("test_pass"),
        testcontainers.WithWaitStrategy(
            wait.ForLog("database system is ready to accept connections").
                WithOccurrence(2).
                WithStartupTimeout(30*time.Second)),
    )
    if err != nil {
        t.Fatalf("failed to start container: %v", err)
    }

    connStr, err := pgContainer.ConnectionString(ctx, "sslmode=disable")
    if err != nil {
        t.Fatalf("failed to get connection string: %v", err)
    }

    cfg := &postgres.Config{
        DatabaseURL: connStr,
        MaxConns:    5,
        MinConns:    1,
    }

    cleanup := func() {
        _ = pgContainer.Terminate(context.Background())
    }

    return cfg, cleanup
}
```

---

## 3. محاكاة واجهة `postgres.DBTX` في اختبارات طبقة الـ Service

نظراً لأن مستودعات التخزين تعتمد على واجهة `DBTX` البسيطة، يمكن بسهولة اختبار طبقة الـ Services ومنطق الأعمال دون الحاجة لقاعدة بيانات حقيقية عن طريق إنشاء Mock بسيط أو استخدام أدوات المحاكاة:

```go
type MockDBTX struct {
    ExecFunc     func(ctx context.Context, sql string, arguments ...any) (pgconn.CommandTag, error)
    QueryRowFunc func(ctx context.Context, sql string, args ...any) pgx.Row
}
```

هذا الفصل يحافظ على نظافة الاختبارات وسرعة تشغيلها في مسار التكامل المستمر (CI/CD).
