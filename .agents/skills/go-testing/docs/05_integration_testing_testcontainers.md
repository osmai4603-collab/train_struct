# 05. اختبارات التكامل والبيئات الحقيقية (Integration Testing & Testcontainers)

## 1. لماذا يُعتبر `go-sqlmock` نمطاً مضاداً في الأنظمة المعقدة؟

مكتبة `go-sqlmock` تقوم بمحاكاة واجهة مشغل الـ SQL، ولكنها تعاني من عيوب قاتلة في المشاريع الكبيرة:

1. **تجاهل الأخطاء التركيبية (Syntax Errors):** إذا كتبت `SELEECT` بدلاً من `SELECT`، سينجح اختبار `sqlmock` إذا طابقت النص في التوقعات!
2. **عدم فحص القيود (Constraints):** لا تفحص المفاتيح الأجنبية، ولا القيود الفريدة (`UNIQUE`)، ولا الـ Checks.
3. **عدم دعم المزايا المتقدمة:** تفشل في محاكاة خصائص PostgreSQL المتقدمة مثل JSONB، والبحث النصي الكامل (Full-Text Search)، والأقفال الذرية (`SELECT FOR UPDATE`).

---

## 2. المعيار الصناعي: Testcontainers for Go

باستخدام `testcontainers-go`، نقوم بتشغيل محرك قاعدة بيانات حقيقي مطابق للإنتاج داخل حاوية Docker مؤقتة أثناء تشغيل الاختبار، مع ضمان تدميرها تلقائياً عند الانتهاء.

### 2.1 هيكل اختبار تكاملي نموذجي لـ PostgreSQL

```go
//go:build integration

package repository_test

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

func setupPostgres(t *testing.T) *sql.DB {
 t.Helper()
 ctx := t.Context()

 container, err := postgres.Run(ctx,
  "postgres:16-alpine",
  postgres.WithDatabase("test_db"),
  postgres.WithUsername("test_user"),
  postgres.WithPassword("test_pass"),
  testcontainers.WithWaitStrategy(
   wait.ForLog("database system is ready to accept connections").
    WithOccurrence(2).
    WithStartupTimeout(15*time.Second),
  ),
 )
 if err != nil {
  t.Fatalf("failed to start postgres container: %v", err)
 }

 t.Cleanup(func() {
  // تدمير الحاوية بعد انتهاء الاختبار بالكامل
  if err := container.Terminate(context.Background()); err != nil {
   t.Logf("failed to terminate container: %v", err)
  }
 })

 connStr, err := container.ConnectionString(ctx, "sslmode=disable")
 if err != nil {
  t.Fatalf("failed to get connection string: %v", err)
 }

 db, err := sql.Open("pgx", connStr)
 if err != nil {
  t.Fatalf("failed to open database connection: %v", err)
 }

 t.Cleanup(func() {
  _ = db.Close()
 })

 return db
}
```

---

## 3. استراتيجيات إدارة ونظافة الحالة بين الاختبارات (State Cleanliness)

عند تشغيل عدة اختبارات على نفس قاعدة البيانات، هناك استراتيجيتان رئيسيتان:

1. **استراتيجية التراجع بالمعاملات (Transaction Rollback):**
   - فتح معاملة (`db.BeginTx()`) في بداية كل اختبار.
   - تمرير الـ `tx` للمستودع.
   - استدعاء `t.Cleanup(func() { _ = tx.Rollback() })`.
   - **الميزة:** فائقة السرعة.
   - **العيب:** لا تدعم الكود الذي يقوم بفتح معاملات متداخلة أو استدعاءات خارجية مستقلة.

2. **استراتيجية التفريغ السريع (Fast Truncate):**
   - تشغيل جدول البيانات الحقيقي.
   - بعد كل اختبار، تنفيذ: `TRUNCATE TABLE users, orders CASCADE;`.
   - **الميزة:** واقعية 100% وتسمح باختبار المعاملات الكاملة.

---

## 4. تنظيم وسوم البناء (Build Tags)

لحماية المطورين من بطء تشغيل الحاويات أثناء التعديلات السريعة، يتم عزل جميع ملفات التكامل بوسم البناء:

```go
//go:build integration
```

- تشغيل اختبارات الوحدة السريعة فقط:

  ```bash
  go test ./...
  ```

- تشغيل اختبارات التكامل:

  ```bash
  go test -tags=integration -v ./...
  ```
