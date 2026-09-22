# أفضل الممارسات العالمية للتعامل مع PostgreSQL في مشاريع Go الكبيرة

> **تاريخ البحث:** 2026-09-21
>
> **المصادر الرسمية المعتمدة:**
>
> - [go.dev/doc/database](https://go.dev/doc/database/) — التوثيق الرسمي لـ Go للتعامل مع قواعد البيانات
> - [pkg.go.dev/database/sql](https://pkg.go.dev/database/sql) — مرجع المكتبة القياسية
> - [github.com/jackc/pgx](https://github.com/jackc/pgx) — المشغّل الرسمي الموصى به لـ PostgreSQL
> - [pkg.go.dev/github.com/jackc/pgx/v5](https://pkg.go.dev/github.com/jackc/pgx/v5) — توثيق pgx v5
> - [sqlc.dev](https://sqlc.dev) — أداة توليد الكود من SQL
> - [postgresql.org/docs](https://www.postgresql.org/docs/) — التوثيق الرسمي لـ PostgreSQL
> - [go.dev/doc/effective_go](https://go.dev/doc/effective_go) — Effective Go
> - [opentelemetry.io](https://opentelemetry.io/) — معيار المراقبة والتتبع

---

## جدول المحتويات

1. [حزمة الأدوات الموصى بها (Recommended Stack)](#1-حزمة-الأدوات-الموصى-بها)
2. [إدارة الاتصالات (Connection Management)](#2-إدارة-الاتصالات)
3. [البنية المعمارية (Architecture)](#3-البنية-المعمارية)
4. [إدارة المعاملات (Transaction Management)](#4-إدارة-المعاملات)
5. [معالجة الأخطاء (Error Handling)](#5-معالجة-الأخطاء)
6. [آلية إعادة المحاولة (Retry Strategy)](#6-آلية-إعادة-المحاولة)
7. [ترحيل المخطط (Schema Migrations)](#7-ترحيل-المخطط)
8. [الأداء العالي (High Performance)](#8-الأداء-العالي)
9. [الأمان (Security)](#9-الأمان)
10. [المراقبة والتتبع (Observability)](#10-المراقبة-والتتبع)
11. [الاختبارات (Testing)](#11-الاختبارات)
12. [ميزات PostgreSQL المتقدمة مع pgx](#12-ميزات-postgresql-المتقدمة-مع-pgx)
13. [الأنماط المضادة (Anti-Patterns)](#13-الأنماط-المضادة)
14. [ملخص القرارات (Decision Matrix)](#14-ملخص-القرارات)

---

## 1. حزمة الأدوات الموصى بها

| الوظيفة | الأداة الموصى بها | البديل | السبب |
| :--- | :--- | :--- | :--- |
| **المشغّل (Driver)** | `github.com/jackc/pgx/v5` | `lib/pq` (قديم) | أداء أعلى، صيانة نشطة، دعم كامل لميزات PostgreSQL |
| **تجميع الاتصالات** | `pgxpool` | `database/sql` | تخزين Prepared Statements تلقائياً، فحص صحة أفضل |
| **التفاعل مع SQL** | `sqlc` | `squirrel` / Raw SQL | توليد كود آمن الأنواع (type-safe) من SQL خام |
| **ترحيل المخطط** | `golang-migrate` أو `goose` | `atlas` | مستقران ومجربان في الإنتاج |
| **المراقبة** | OpenTelemetry + `otelpgx` | Prometheus مباشر | معيار صناعي للتتبع الموزع |

### لماذا pgx بدلاً من database/sql؟

```text
database/sql:
  ✅ محايد (vendor-neutral)
  ✅ جزء من المكتبة القياسية
  ❌ لا يدعم COPY protocol
  ❌ لا يدعم LISTEN/NOTIFY
  ❌ لا يدعم الأنواع المخصصة بشكل طبيعي
  ❌ boilerplate أكثر في الـ scanning

pgx (مباشر بدون database/sql):
  ✅ دعم كامل لبروتوكول PostgreSQL الموسع
  ✅ تخزين Prepared Statements تلقائي وشفاف
  ✅ دعم COPY, LISTEN/NOTIFY, Custom Types
  ✅ أداء أفضل بنسبة 20-40% حسب المعايير
  ✅ تكامل أصلي مع OpenTelemetry
```

### لماذا sqlc بدلاً من ORM؟

```text
ORM (مثل GORM):
  ✅ سرعة التطوير الأولية
  ❌ يخفي الاستعلامات غير الكفؤة
  ❌ مشكلة N+1 شائعة
  ❌ صعوبة تصحيح SQL المعقد (CTEs, Window Functions)
  ❌ "سحر" يصعب تتبعه

sqlc:
  ✅ أمان الأنواع في وقت الترجمة
  ✅ تحكم كامل في SQL
  ✅ أداء Raw SQL مع راحة ORM
  ✅ لا overhead في وقت التشغيل
  ✅ SQL هو المصدر الوحيد للحقيقة
```

---

## 2. إدارة الاتصالات

### 2.1 إعداد pgxpool

```go
package database

import (
    "context"
    "fmt"
    "os"
    "time"

    "github.com/jackc/pgx/v5/pgxpool"
)

// NewPool ينشئ connection pool مضبوط للإنتاج.
func NewPool(ctx context.Context) (*pgxpool.Pool, error) {
    config, err := pgxpool.ParseConfig(os.Getenv("DATABASE_URL"))
    if err != nil {
        return nil, fmt.Errorf("parsing database config: %w", err)
    }

    // === إعدادات التجميع ===
    config.MaxConns = 25                           // الحد الأقصى للاتصالات المفتوحة
    config.MinConns = 5                            // الحد الأدنى للاتصالات الجاهزة
    config.MaxConnLifetime = 1 * time.Hour         // أقصى عمر للاتصال (يمنع الاتصالات القديمة)
    config.MaxConnIdleTime = 5 * time.Minute       // أقصى وقت خمول
    config.HealthCheckPeriod = 30 * time.Second    // فترة فحص الصحة

    pool, err := pgxpool.NewWithConfig(ctx, config)
    if err != nil {
        return nil, fmt.Errorf("creating connection pool: %w", err)
    }

    // التحقق من الاتصال عند بدء التشغيل
    if err := pool.Ping(ctx); err != nil {
        pool.Close()
        return nil, fmt.Errorf("pinging database: %w", err)
    }

    return pool, nil
}
```

### 2.2 قواعد ذهبية لإدارة الاتصالات

| القاعدة | التفصيل |
| :--- | :--- |
| **نسخة واحدة** | أنشئ `*pgxpool.Pool` مرة واحدة وشاركها عبر التطبيق بأكمله |
| **لا اتصال لكل طلب** | لا تفتح اتصالاً جديداً لكل HTTP request |
| **استخدم Context دائماً** | مرر `context.Context` لكل عملية قاعدة بيانات |
| **Ping عند البدء** | تحقق من صحة الاتصال عند بدء تشغيل التطبيق |
| **حساب إجمالي الاتصالات** | `(عدد النسخ) × (MaxConns لكل نسخة) ≤ PostgreSQL max_connections` |
| **أغلق Rows دائماً** | `defer rows.Close()` فوراً بعد `pool.Query` |

### 2.3 المعاملات (Parameters) الموصى بها

```text
┌─────────────────────────┬──────────────────────────────────────────────────────┐
│ SetMaxOpenConns (25)    │ يمنع إغراق PostgreSQL بالاتصالات                     │
├─────────────────────────┼──────────────────────────────────────────────────────┤
│ SetMaxIdleConns (25)    │ يساوي أو قريب من MaxOpen لتقليل churn               │
├─────────────────────────┼──────────────────────────────────────────────────────┤
│ SetConnMaxLifetime (1h) │ إعادة تدوير الاتصالات دورياً                         │
├─────────────────────────┼──────────────────────────────────────────────────────┤
│ SetConnMaxIdleTime (5m) │ إغلاق الاتصالات الخاملة                              │
└─────────────────────────┴──────────────────────────────────────────────────────┘
```

> [!WARNING]
> **الافتراضيات الخطيرة:**
>
> - `MaxIdleConns` الافتراضي غالباً `2` — يسبب connection churn تحت الحمل
> - `MaxOpenConns` الافتراضي غير محدود — يمكن أن يُسقط قاعدة البيانات في ذروة الحركة

---

## 3. البنية المعمارية

### 3.1 هيكل المشروع الموصى به

```text
myapp/
├── cmd/
│   └── api/
│       └── main.go              # نقطة الدخول
├── internal/
│   ├── domain/                   # الكيانات وواجهات Repository
│   │   ├── user.go               # type User struct { ... }
│   │   └── repository.go        # type UserRepository interface { ... }
│   ├── repository/               # تطبيقات PostgreSQL
│   │   └── postgres/
│   │       ├── user_repo.go      # PostgresUserRepository
│   │       └── queries/          # ملفات sqlc المولدة
│   ├── service/                  # منطق الأعمال (يعتمد على domain interfaces)
│   │   └── user_service.go
│   └── platform/
│       └── database/             # إعداد pgxpool المشترك
│           └── pool.go
├── migrations/                   # ملفات ترحيل المخطط
│   ├── 20250921_create_users.up.sql
│   └── 20250921_create_users.down.sql
└── sqlc/                         # استعلامات sqlc المصدرية
    ├── sqlc.yaml
    ├── users.sql
    └── orders.sql
```

### 3.2 نمط Repository (الفصل المعماري)

```go
// ═══════════════════════════════════════════════════════
// الطبقة 1: Domain Layer — لا تعرف شيئاً عن pgx
// ═══════════════════════════════════════════════════════

// internal/domain/repository.go
package domain

import (
    "context"

    "github.com/google/uuid"
)

// User هو كيان المجال الخالص.
type User struct {
    ID    uuid.UUID
    Name  string
    Email string
}

// UserRepository يحدد العقد الذي تعتمد عليه طبقة الخدمة.
type UserRepository interface {
    GetByID(ctx context.Context, id uuid.UUID) (*User, error)
    Create(ctx context.Context, user *User) error
    List(ctx context.Context, limit, offset int) ([]*User, error)
}


// ═══════════════════════════════════════════════════════
// الطبقة 2: Repository Layer — تعرف pgx
// ═══════════════════════════════════════════════════════

// internal/repository/postgres/user_repo.go
package postgres

import (
    "context"

    "github.com/google/uuid"
    "github.com/jackc/pgx/v5/pgxpool"

    "myapp/internal/domain"
)

// PostgresUserRepository ينفذ domain.UserRepository.
type PostgresUserRepository struct {
    pool *pgxpool.Pool  // حقن التبعية، وليس متغير عام
}

// NewUserRepository مُنشئ يقبل الـ pool كتبعية.
func NewUserRepository(pool *pgxpool.Pool) *PostgresUserRepository {
    return &PostgresUserRepository{pool: pool}
}

func (r *PostgresUserRepository) GetByID(ctx context.Context, id uuid.UUID) (*domain.User, error) {
    query := `SELECT id, name, email FROM users WHERE id = $1`
    row := r.pool.QueryRow(ctx, query, id)

    var user domain.User
    err := row.Scan(&user.ID, &user.Name, &user.Email)
    if err != nil {
        return nil, err  // يتم ترجمة الخطأ في طبقة أعلى
    }
    return &user, nil
}


// ═══════════════════════════════════════════════════════
// الطبقة 3: Service Layer — تعتمد على الواجهة فقط
// ═══════════════════════════════════════════════════════

// internal/service/user_service.go
package service

import (
    "context"

    "github.com/google/uuid"

    "myapp/internal/domain"
)

type UserService struct {
    repo domain.UserRepository  // واجهة، وليس تطبيق PostgreSQL مباشر
}

func NewUserService(repo domain.UserRepository) *UserService {
    return &UserService{repo: repo}
}

func (s *UserService) GetUser(ctx context.Context, id uuid.UUID) (*domain.User, error) {
    return s.repo.GetByID(ctx, id)
}
```

### 3.3 واجهة مرنة للمعاملات

```go
// DBTX واجهة يرضيها كلا من *pgxpool.Pool و pgx.Tx
// مما يسمح للـ repository بالعمل داخل وخارج المعاملات بسلاسة
type DBTX interface {
    Query(ctx context.Context, sql string, args ...any) (pgx.Rows, error)
    QueryRow(ctx context.Context, sql string, args ...any) pgx.Row
    Exec(ctx context.Context, sql string, args ...any) (pgconn.CommandTag, error)
}

type Repository struct {
    db DBTX  // يمكن أن يكون Pool أو Tx
}
```

---

## 4. إدارة المعاملات

### 4.1 النمط الآمن الموصى به

```go
// ExecTx ينفذ دالة داخل معاملة قاعدة بيانات.
// إذا أرجعت الدالة خطأ، يتم التراجع تلقائياً.
func (r *Repository) ExecTx(ctx context.Context, fn func(pgx.Tx) error) error {
    tx, err := r.pool.Begin(ctx)
    if err != nil {
        return fmt.Errorf("beginning transaction: %w", err)
    }
    // defer Rollback آمن: إذا تم Commit بنجاح، لا يفعل شيئاً
    defer tx.Rollback(ctx)

    if err := fn(tx); err != nil {
        return err
    }

    return tx.Commit(ctx)
}
```

### 4.2 استخدام مع sqlc

```go
// TransferMoney مثال على معاملة مع sqlc
func (s *Service) TransferMoney(ctx context.Context, fromID, toID int64, amount float64) error {
    return s.repo.ExecTx(ctx, func(tx pgx.Tx) error {
        // ربط الاستعلامات المولدة بالمعاملة
        q := s.queries.WithTx(tx)

        // خصم من المرسل
        if err := q.DeductBalance(ctx, fromID, amount); err != nil {
            return fmt.Errorf("deducting from sender: %w", err)
        }

        // إضافة للمستلم
        if err := q.AddBalance(ctx, toID, amount); err != nil {
            return fmt.Errorf("adding to receiver: %w", err)
        }

        return nil // يتم Commit تلقائياً في ExecTx
    })
}
```

### 4.3 قواعد المعاملات

> [!IMPORTANT]
>
> - **استخدم `defer tx.Rollback()` دائماً** بعد `Begin` — إذا تم `Commit` بنجاح، يكون الـ Rollback المؤجل بلا تأثير
> - **لا تخلط** دوال المعاملات في `database/sql` مع عبارات SQL للمعاملات (`BEGIN`/`COMMIT`)
> - **اجعل المعاملات قصيرة** — معاملة طويلة = أقفال طويلة = تدهور الأداء
> - **لا تسرّب SQL إلى طبقة الخدمة** — استخدم نمط Unit of Work أو ExecTx

---

## 5. معالجة الأخطاء

### 5.1 التمييز بين أنواع أخطاء PostgreSQL

```go
import (
    "errors"

    "github.com/jackc/pgx/v5"
    "github.com/jackc/pgx/v5/pgconn"
)

// أخطاء PostgreSQL SQLSTATE المهمة
const (
    UniqueViolation     = "23505"  // انتهاك التفرد
    ForeignKeyViolation = "23503"  // انتهاك المفتاح الأجنبي
    CheckViolation      = "23514"  // انتهاك قيد CHECK
    NotNullViolation    = "23502"  // انتهاك NOT NULL
    SerializationFail   = "40001"  // فشل التسلسل (يتطلب إعادة محاولة)
    DeadlockDetected    = "40P01"  // كشف Deadlock (يتطلب إعادة محاولة)
)

// translateDBError يترجم أخطاء pgx إلى أخطاء النطاق.
func translateDBError(err error) error {
    if err == nil {
        return nil
    }

    // التحقق من "لم يتم العثور على سجل"
    if errors.Is(err, pgx.ErrNoRows) {
        return ErrNotFound  // خطأ نطاق مخصص
    }

    // التحقق من أخطاء PostgreSQL المحددة
    var pgErr *pgconn.PgError
    if errors.As(err, &pgErr) {
        switch pgErr.Code {
        case UniqueViolation:
            return fmt.Errorf("%w: %s", ErrConflict, pgErr.ConstraintName)
        case ForeignKeyViolation:
            return fmt.Errorf("%w: %s", ErrInvalidReference, pgErr.ConstraintName)
        case SerializationFail, DeadlockDetected:
            return fmt.Errorf("%w: %s", ErrTransient, pgErr.Message)
        default:
            return fmt.Errorf("database error [%s]: %w", pgErr.Code, err)
        }
    }

    return fmt.Errorf("unexpected database error: %w", err)
}
```

### 5.2 قواعد معالجة الأخطاء

| القاعدة | التفصيل |
| :--- | :--- |
| **تحقق من `ErrNoRows`** | لا تعامل "لم يتم العثور" كخطأ نظام عام |
| **افحص SQLSTATE** | استخدم `errors.As` لاستخراج `*pgconn.PgError` |
| **لا تسرّب التفاصيل** | لا تمرر رسائل أخطاء قاعدة البيانات الخام للمستخدم النهائي |
| **ترجم في الحدود** | ترجم أخطاء pgx إلى أخطاء النطاق في طبقة Repository فقط |
| **ميّز العابر من الدائم** | أخطاء `40001` و `40P01` عابرة (يمكن إعادة المحاولة) |

---

## 6. آلية إعادة المحاولة

### 6.1 Exponential Backoff مع Jitter

```go
import (
    "context"
    "errors"
    "math"
    "math/rand"
    "time"

    "github.com/jackc/pgx/v5/pgconn"
)

// RetryConfig إعدادات إعادة المحاولة.
type RetryConfig struct {
    MaxRetries  int
    BaseDelay   time.Duration
    MaxDelay    time.Duration
}

// DefaultRetryConfig إعدادات افتراضية معقولة.
var DefaultRetryConfig = RetryConfig{
    MaxRetries: 3,
    BaseDelay:  100 * time.Millisecond,
    MaxDelay:   5 * time.Second,
}

// isTransient يحدد ما إذا كان الخطأ عابراً ويمكن إعادة المحاولة.
func isTransient(err error) bool {
    var pgErr *pgconn.PgError
    if errors.As(err, &pgErr) {
        switch pgErr.Code {
        case "40001", // Serialization failure
             "40P01": // Deadlock detected
            return true
        }
    }
    return false
}

// WithRetry يعيد محاولة الدالة عند أخطاء عابرة فقط.
func WithRetry(ctx context.Context, cfg RetryConfig, fn func() error) error {
    var lastErr error
    for attempt := 0; attempt <= cfg.MaxRetries; attempt++ {
        lastErr = fn()
        if lastErr == nil {
            return nil
        }

        // لا تعيد المحاولة إلا للأخطاء العابرة
        if !isTransient(lastErr) {
            return lastErr
        }

        // حساب التأخير مع jitter
        delay := time.Duration(float64(cfg.BaseDelay) * math.Pow(2, float64(attempt)))
        if delay > cfg.MaxDelay {
            delay = cfg.MaxDelay
        }
        jitter := time.Duration(rand.Int63n(int64(delay)))
        delay = delay/2 + jitter

        select {
        case <-ctx.Done():
            return ctx.Err()
        case <-time.After(delay):
            // متابعة المحاولة التالية
        }
    }
    return fmt.Errorf("max retries exceeded: %w", lastErr)
}
```

### 6.2 متى تعيد المحاولة ومتى لا تفعل

```text
أعد المحاولة ✅:
  • 40001 — Serialization failure
  • 40P01 — Deadlock detected
  • أخطاء اتصال الشبكة المؤقتة

لا تعيد المحاولة ❌:
  • 23505 — Unique violation (خطأ منطقي)
  • 23503 — Foreign key violation (بيانات غير صالحة)
  • 42601 — Syntax error (خطأ برمجي)
  • أي خطأ غير عابر
```

> [!CAUTION]
> **Idempotency أمر حاسم:** إذا فشلت المعاملة بعد `COMMIT` ولكن قبل وصول التأكيد للعميل،
> قد تؤدي إعادة المحاولة لتنفيذ العملية مرتين. استخدم **مفاتيح Idempotency** لمنع ذلك.

---

## 7. ترحيل المخطط

### 7.1 المقارنة بين الأدوات

| المعيار | golang-migrate | goose |
| :--- | :--- | :--- |
| **النهج** | CLI-first | Library-first |
| **لغة الترحيل** | SQL فقط | SQL + Go code |
| **مصادر الملفات** | ملفات, GitHub, S3, وغيرها | ملفات محلية |
| **الاعتماد في الإنتاج** | واسع جداً | واسع |
| **المرونة البرمجية** | متوسطة | عالية |

### 7.2 أفضل الممارسات للترحيل

#### التسمية

```text
✅ استخدم Timestamps:
   20250921030200_create_users_table.up.sql
   20250921030200_create_users_table.down.sql

❌ لا تستخدم أرقاماً تسلسلية:
   001_create_users.sql  ← يسبب تعارضات عند عمل فريق
```

#### القواعد الذهبية

```text
1. لا تعدّل ترحيلاً مطبقاً أبداً
   → إذا أخطأت، أنشئ ترحيلاً جديداً يصحح الخطأ (roll-forward)

2. اجعل الترحيل ذرياً (Atomic)
   → لف الترحيل في معاملة (PostgreSQL يدعم DDL في معاملات)

3. اجعل الترحيل قابلاً للتكرار (Idempotent)
   → استخدم CREATE TABLE IF NOT EXISTS
   → تحقق من وجود الفهارس/الأعمدة قبل الإنشاء

4. اكتب ترحيل "هبوط" (down) دائماً
   → حتى لو نادراً ما تستخدمه

5. ضمّن الملفات في الثنائي (embed)
   → استخدم go:embed لتضمين ملفات الترحيل في البرنامج
```

#### التضمين باستخدام go:embed

```go
import "embed"

//go:embed migrations/*.sql
var migrationFS embed.FS

// استخدم migrationFS مع golang-migrate أو goose
// هذا يضمن أن ملفات الترحيل متاحة دائماً مع التطبيق
```

### 7.3 التكامل مع CI/CD

```text
┌──────────────┐     ┌──────────────┐     ┌──────────────┐
│  Development │────▶│   Staging    │────▶│  Production  │
│              │     │              │     │              │
│ • كتابة SQL  │     │ • تشغيل      │     │ • تشغيل      │
│ • اختبار     │     │   الترحيل    │     │   الترحيل    │
│   محلي       │     │ • اختبار     │     │ • مراقبة     │
│              │     │   التكامل    │     │              │
└──────────────┘     └──────────────┘     └──────────────┘
```

---

## 8. الأداء العالي

### 8.1 الإدراج الجماعي باستخدام CopyFrom

```go
// CopyFrom يستخدم بروتوكول COPY الخاص بـ PostgreSQL
// وهو أسرع بكثير من INSERT متعددة

func (r *Repository) BulkInsertUsers(ctx context.Context, users []*domain.User) (int64, error) {
    rows := make([][]any, len(users))
    for i, u := range users {
        rows[i] = []any{u.ID, u.Name, u.Email, u.CreatedAt}
    }

    count, err := r.pool.CopyFrom(
        ctx,
        pgx.Identifier{"users"},                             // اسم الجدول
        []string{"id", "name", "email", "created_at"},       // أسماء الأعمدة
        pgx.CopyFromRows(rows),                               // مصدر البيانات
    )
    if err != nil {
        return 0, fmt.Errorf("bulk insert users: %w", err)
    }
    return count, nil
}
```

### 8.2 Batch Queries باستخدام SendBatch

```go
// SendBatch يجمع عدة استعلامات ويرسلها في رحلة واحدة للشبكة
func (r *Repository) GetMultipleResources(ctx context.Context, userID, orderID uuid.UUID) (*User, *Order, error) {
    batch := &pgx.Batch{}
    batch.Queue("SELECT id, name, email FROM users WHERE id = $1", userID)
    batch.Queue("SELECT id, total, status FROM orders WHERE id = $1", orderID)

    results := r.pool.SendBatch(ctx, batch)
    defer results.Close()

    var user User
    err := results.QueryRow().Scan(&user.ID, &user.Name, &user.Email)
    if err != nil {
        return nil, nil, err
    }

    var order Order
    err = results.QueryRow().Scan(&order.ID, &order.Total, &order.Status)
    if err != nil {
        return nil, nil, err
    }

    return &user, &order, nil
}
```

### 8.3 مقارنة أساليب الإدراج

```text
┌───────────────────────┬───────────────┬──────────────────────────────────┐
│ الأسلوب               │ سرعة نسبية   │ متى تستخدمه                      │
├───────────────────────┼───────────────┼──────────────────────────────────┤
│ INSERT واحد           │ 1x            │ سجل واحد                         │
│ INSERT متعدد القيم    │ ~5x           │ عشرات السجلات                     │
│ SendBatch             │ ~10x          │ استعلامات متنوعة في رحلة واحدة    │
│ CopyFrom              │ ~50x          │ آلاف/ملايين السجلات              │
└───────────────────────┴───────────────┴──────────────────────────────────┘
```

---

## 9. الأمان

### 9.1 منع SQL Injection

```go
// ═══════════════════════════════════════════════════════
// ❌ خطير — عرضة لـ SQL Injection
// ═══════════════════════════════════════════════════════
query := fmt.Sprintf("SELECT * FROM users WHERE username = '%s'", username)
conn.Query(ctx, query)

// ═══════════════════════════════════════════════════════
// ✅ آمن — استعلام ذو معاملات (Parameterized)
// ═══════════════════════════════════════════════════════
query := "SELECT * FROM users WHERE username = $1"
conn.Query(ctx, query, username)
```

> [!IMPORTANT]
> pgx يستخدم بروتوكول PostgreSQL الموسع (Extended Protocol) افتراضياً،
> مما يفصل بنية الاستعلام عن البيانات تلقائياً. لا حاجة لاستدعاء `Prepare` يدوياً.

### 9.2 قائمة التحقق الأمني

| المهمة | الإجراء |
| :--- | :--- |
| **استعلامات SQL** | استخدم `$1, $2` — لا تستخدم `fmt.Sprintf` أو التسلسل النصي أبداً |
| **الاتصالات** | فرض SSL/TLS لكل حركة مرور قاعدة البيانات (`sslmode=verify-full`) |
| **التحكم في الوصول** | أدوار غير superuser بأقل صلاحيات مطلوبة |
| **المصادقة** | استخدم `SCRAM-SHA-256` في `pg_hba.conf` |
| **التحقق من المدخلات** | تحقق على مستوى التطبيق (نوع البيانات، الشكل) |
| **التحديثات** | حدّث PostgreSQL و pgx إلى آخر إصدار ثانوي |
| **سجل التدقيق** | فعّل `pg_audit` لتتبع النشاط |

---

## 10. المراقبة والتتبع

### 10.1 التتبع باستخدام OpenTelemetry

```go
import (
    "github.com/jackc/pgx/v5/pgxpool"
    "github.com/pgx-contrib/pgxotel"
)

func NewPoolWithTracing(ctx context.Context, databaseURL string) (*pgxpool.Pool, error) {
    config, err := pgxpool.ParseConfig(databaseURL)
    if err != nil {
        return nil, err
    }

    // تثبيت OTel tracer — يلتقط تلقائياً:
    //   • وقت تنفيذ الاستعلام و span metadata
    //   • أحداث اكتساب الاتصال ودورة حياة التجميع
    //   • عمليات Batch و Prepared Statements
    //   • معدلات الأخطاء ومعاملات الاستعلام
    config.ConnConfig.Tracer = pgxotel.NewTracer()

    // إعدادات التجميع
    config.MaxConns = 25
    config.MinConns = 5

    return pgxpool.NewWithConfig(ctx, config)
}
```

### 10.2 مقاييس التجميع (Pool Metrics)

```go
// جمع وتصدير مقاييس pgxpool.Stat() إلى Prometheus
func collectPoolMetrics(pool *pgxpool.Pool) {
    stat := pool.Stat()

    // مقاييس حيوية يجب مراقبتها:
    // stat.AcquiredConns()  — الاتصالات المستخدمة حالياً
    // stat.IdleConns()      — الاتصالات الخاملة
    // stat.TotalConns()     — إجمالي الاتصالات
    // stat.EmptyAcquireCount() — عدد مرات عدم توفر اتصال (مؤشر خطير!)
    // stat.AcquireDuration() — وقت الانتظار لاكتساب اتصال
}
```

### 10.3 المراقبة على مستوى PostgreSQL

```text
pg_stat_statements (امتداد أساسي):
  → تحديد الاستعلامات البطيئة
  → البيانات عالية التكرار
  → نسب cache hit

Prometheus Postgres Exporter:
  → CPU, I/O, الأقفال
  → تأخر النسخ (replication lag)
  → مقاييس لا يمكن لـ pgx رؤيتها من جانب التطبيق
```

### 10.4 السجلات المنظمة

```go
// استخدم slog مع سياق منظم
slog.ErrorContext(ctx, "database query failed",
    "query_name", "GetUserByID",
    "duration_ms", elapsed.Milliseconds(),
    "error_code", pgErr.Code,
    "error", err,
)
```

> [!WARNING]
> **تجنب "الإفراط في التتبع":**
> لا تلتقط معاملات SQL الخام في بيئات عالية الحركة — قد تسرّب PII
> (معلومات تعريف شخصية) أو تضخم تخزين بيانات التتبع.

---

## 11. الاختبارات

### 11.1 اختبارات التكامل مع Testcontainers

```go
package repository_test

import (
    "context"
    "testing"

    "github.com/testcontainers/testcontainers-go"
    "github.com/testcontainers/testcontainers-go/modules/postgres"
    "github.com/testcontainers/testcontainers-go/wait"
)

func setupTestDB(t *testing.T) *pgxpool.Pool {
    t.Helper()
    ctx := context.Background()

    // تشغيل PostgreSQL في حاوية Docker
    container, err := postgres.Run(ctx,
        "postgres:16-alpine",  // ثبّت الإصدار — لا تستخدم latest
        postgres.WithDatabase("testdb"),
        postgres.WithUsername("test"),
        postgres.WithPassword("test"),
        testcontainers.WithWaitStrategy(
            wait.ForLog("database system is ready to accept connections").
                WithOccurrence(2)),
    )
    if err != nil {
        t.Fatal(err)
    }
    t.Cleanup(func() { container.Terminate(ctx) })

    connStr, _ := container.ConnectionString(ctx, "sslmode=disable")
    pool, err := pgxpool.New(ctx, connStr)
    if err != nil {
        t.Fatal(err)
    }
    t.Cleanup(pool.Close)

    // تشغيل ترحيلات المخطط ضد الحاوية
    runMigrations(t, connStr)

    return pool
}
```

### 11.2 أفضل ممارسات الاختبار

| الممارسة | التفصيل |
| :--- | :--- |
| **حاوية Singleton** | شارك حاوية واحدة عبر جميع الاختبارات لتسريع التنفيذ |
| **tmpfs للأداء** | `docker run --tmpfs /var/lib/postgresql/data` يلغي تأخر القرص |
| **تعطيل المتانة** | `-c fsync=off -c full_page_writes=off` للاختبارات فقط |
| **عزل البيانات** | `TRUNCATE` الجداول بين الاختبارات أو استخدم معاملة مع Rollback |
| **منافذ ديناميكية** | لا تستخدم منافذ ثابتة — تجنب التعارض في CI/CD |
| **إصدار ثابت** | `postgres:16-alpine` وليس `postgres:latest` |
| **تطابق المخطط** | شغّل نفس أداة الترحيل ضد الحاوية لمطابقة بيئة الإنتاج |

### 11.3 اختبارات الوحدة مع الواجهات

```go
// بفضل نمط Repository والواجهات، يمكن بسهولة إنشاء mocks:

type MockUserRepository struct {
    GetByIDFunc func(ctx context.Context, id uuid.UUID) (*domain.User, error)
}

func (m *MockUserRepository) GetByID(ctx context.Context, id uuid.UUID) (*domain.User, error) {
    return m.GetByIDFunc(ctx, id)
}

func TestUserService_GetUser(t *testing.T) {
    expectedUser := &domain.User{ID: uuid.New(), Name: "Ali"}

    mock := &MockUserRepository{
        GetByIDFunc: func(ctx context.Context, id uuid.UUID) (*domain.User, error) {
            return expectedUser, nil
        },
    }

    service := NewUserService(mock) // لا حاجة لقاعدة بيانات حقيقية!
    user, err := service.GetUser(context.Background(), expectedUser.ID)

    assert.NoError(t, err)
    assert.Equal(t, expectedUser.Name, user.Name)
}
```

---

## 12. ميزات PostgreSQL المتقدمة مع pgx

### 12.1 LISTEN/NOTIFY (Pub/Sub في الوقت الحقيقي)

```go
// ═══════════════════════════════════════════════════════
// الجانب SQL: إعداد الإشعارات التلقائية
// ═══════════════════════════════════════════════════════

// CREATE OR REPLACE FUNCTION notify_changes() RETURNS TRIGGER AS $$
// BEGIN
//   PERFORM pg_notify('data_changes', row_to_json(NEW)::text);
//   RETURN NEW;
// END;
// $$ LANGUAGE plpgsql;
//
// CREATE TRIGGER trigger_notify_changes
// AFTER INSERT ON my_table
// FOR EACH ROW EXECUTE PROCEDURE notify_changes();


// ═══════════════════════════════════════════════════════
// الجانب Go: الاستماع للإشعارات
// ═══════════════════════════════════════════════════════

func Listen(ctx context.Context, conn *pgx.Conn) error {
    // 1. الاشتراك في القناة
    _, err := conn.Exec(ctx, "LISTEN data_changes")
    if err != nil {
        return fmt.Errorf("listen: %w", err)
    }

    for {
        // 2. انتظار الإشعارات (يحجب)
        notification, err := conn.WaitForNotification(ctx)
        if err != nil {
            return fmt.Errorf("waiting for notification: %w", err)
        }

        fmt.Printf("Channel: %s, Payload: %s\n",
            notification.Channel, notification.Payload)
    }
}
```

> [!IMPORTANT]
> **قواعد LISTEN/NOTIFY:**
>
> - استخدم اتصال `pgx.Conn` مخصص للاستماع (وليس `pgxpool`)
> - الإشعارات **لا تُحفظ** — إذا كان المستمع معطلاً، تُفقد الرسالة
> - حد الحمولة 8000 بايت — أرسل ID فقط واجلب البيانات الكاملة عند الحاجة
> - استخدم `github.com/jackc/pgxlisten` لإدارة إعادة الاتصال تلقائياً

### 12.2 الأنواع المخصصة (Custom Types)

```go
// تسجيل أنواع PostgreSQL المخصصة (مثل enums, composite types)
func registerCustomTypes(ctx context.Context, pool *pgxpool.Pool) error {
    conn, err := pool.Acquire(ctx)
    if err != nil {
        return err
    }
    defer conn.Release()

    // تحميل النوع من قاعدة البيانات
    dataType, err := conn.Conn().LoadType(ctx, "order_status")
    if err != nil {
        return fmt.Errorf("loading type order_status: %w", err)
    }

    // تسجيل النوع في خريطة الأنواع
    conn.Conn().TypeMap().RegisterType(dataType)

    return nil
}
```

### 12.3 JSONB

```go
// pgx v5 يدعم JSONB بشكل طبيعي:
//   • map[string]any → JSONB تلقائياً
//   • أي struct قابل لـ JSON marshaling
//   • []any للمصفوفات

type UserPreferences struct {
    Theme    string `json:"theme"`
    Language string `json:"language"`
    Timezone string `json:"timezone"`
}

// الإدراج مع JSONB — لا حاجة لترميز يدوي
_, err := pool.Exec(ctx,
    "INSERT INTO users (id, name, preferences) VALUES ($1, $2, $3)",
    userID, name, prefs, // prefs من نوع UserPreferences
)
```

---

## 13. الأنماط المضادة

### ❌ أنماط يجب تجنبها

| النمط المضاد | المشكلة | البديل الصحيح |
| :--- | :--- | :--- |
| **متغير قاعدة بيانات عام** | يجعل الاختبار صعباً، يخفي التبعيات | حقن التبعية عبر المُنشئ |
| **اتصال جديد لكل طلب** | يستنزف الموارد، يبطئ الأداء | استخدم Connection Pool |
| **`fmt.Sprintf` في SQL** | SQL Injection | استعلامات ذات معاملات (`$1`) |
| **تجاهل `rows.Close()`** | تسريب اتصالات → استنفاد التجميع | `defer rows.Close()` فوراً |
| **تجاهل `ErrNoRows`** | يُعامل "غير موجود" كخطأ 500 | تحقق صريح وأرجع خطأ نطاق |
| **معاملات طويلة** | أقفال ممتدة → تدهور الأداء | اجعل المعاملات قصيرة ومحددة |
| **`MaxOpenConns` غير محدود** | إسقاط قاعدة البيانات في ذروة الحركة | ضع حداً مناسباً (مثلاً 25) |
| **استخدام `latest` في Docker** | اختبارات غير حتمية | ثبّت الإصدار (`postgres:16-alpine`) |
| **ORM للاستعلامات المعقدة** | يخفي عدم الكفاءة، مشكلة N+1 | sqlc أو Raw SQL |
| **ترحيلات يدوية في الإنتاج** | عدم تزامن المخطط مع الكود | أداة ترحيل + CI/CD |
| **LISTEN على pgxpool** | لا يعمل — يحتاج اتصال مخصص | `pgx.Conn` مستقل |

---

## 14. ملخص القرارات

### مصفوفة القرارات السريعة

```text
┌──────────────────────────────────┬────────────────────────────────────┐
│ السؤال                          │ القرار                              │
├──────────────────────────────────┼────────────────────────────────────┤
│ أي مشغّل PostgreSQL أستخدم؟     │ pgx v5 (بدون database/sql)         │
│ أي أداة SQL أستخدم؟             │ sqlc لتوليد كود آمن الأنواع        │
│ كيف أدير الاتصالات؟             │ pgxpool مع إعدادات مضبوطة          │
│ كيف أنظم الكود؟                 │ Repository Pattern + DI            │
│ كيف أدير المعاملات؟             │ ExecTx مع defer Rollback           │
│ كيف أعالج الأخطاء؟              │ ترجمة SQLSTATE → أخطاء نطاق        │
│ كيف أدير المخطط؟                │ golang-migrate أو goose + embed    │
│ كيف أدخل بيانات جماعية؟         │ pgx.CopyFrom (بروتوكول COPY)       │
│ كيف أراقب الأداء؟               │ OpenTelemetry + pgxpool.Stat()     │
│ كيف أختبر؟                      │ Testcontainers + واجهات Mock       │
│ كيف أؤمّن الاستعلامات؟          │ معاملات $1 + أقل صلاحيات           │
│ كيف أستمع للتغييرات؟            │ LISTEN/NOTIFY مع pgx.Conn مخصص    │
└──────────────────────────────────┴────────────────────────────────────┘
```

### رسم بياني للبنية المعمارية الكاملة

```text
                    ┌─────────────────┐
                    │   HTTP Handler  │
                    │   (cmd/api)     │
                    └────────┬────────┘
                             │ يستدعي
                    ┌────────▼────────┐
                    │  Service Layer  │
                    │ (domain logic)  │
                    │                 │
                    │ يعتمد على       │
                    │ Repository      │
                    │ interfaces فقط  │
                    └────────┬────────┘
                             │ يستدعي عبر الواجهة
                    ┌────────▼────────┐
                    │ Repository Layer│
                    │ (postgres impl) │
                    │                 │
                    │ • pgx queries   │
                    │ • sqlc generated│
                    │ • error         │
                    │   translation   │
                    └────────┬────────┘
                             │ يستخدم
                    ┌────────▼────────┐
                    │   pgxpool       │
                    │ (connection     │
                    │  pool)          │
                    │                 │
                    │ • OpenTelemetry │
                    │ • Health checks │
                    │ • Pool stats    │
                    └────────┬────────┘
                             │
                    ┌────────▼────────┐
                    │  PostgreSQL     │
                    │                 │
                    │ • pg_stat_stmts │
                    │ • pg_audit      │
                    │ • SCRAM-SHA-256 │
                    │ • SSL/TLS       │
                    └─────────────────┘
```

---

## المصادر والمراجع

### مصادر رسمية

1. **Go Official Docs** — [Accessing relational databases](https://go.dev/doc/database/)
2. **database/sql package** — [pkg.go.dev/database/sql](https://pkg.go.dev/database/sql)
3. **Effective Go** — [go.dev/doc/effective_go](https://go.dev/doc/effective_go)

### مكتبات أساسية

1. **pgx v5** — [github.com/jackc/pgx](https://github.com/jackc/pgx)
2. **sqlc** — [sqlc.dev](https://sqlc.dev/)
3. **golang-migrate** — [github.com/golang-migrate/migrate](https://github.com/golang-migrate/migrate)
4. **goose** — [github.com/pressly/goose](https://github.com/pressly/goose)
5. **pgxlisten** — [pkg.go.dev/github.com/jackc/pgxlisten](https://pkg.go.dev/github.com/jackc/pgxlisten)

### مراقبة

1. **otelpgx** — [github.com/pgx-contrib/pgxotel](https://github.com/pgx-contrib/pgxotel)
2. **OpenTelemetry Go** — [opentelemetry.io/docs/languages/go](https://opentelemetry.io/docs/languages/go/)

### اختبارات

 1. **Testcontainers for Go** — [golang.testcontainers.org](https://golang.testcontainers.org/)

### PostgreSQL

 1. **PostgreSQL Documentation** — [postgresql.org/docs](https://www.postgresql.org/docs/)
 2. **pg_stat_statements** — [postgresql.org/docs/current/pgstatstatements.html](https://www.postgresql.org/docs/current/pgstatstatements.html)
 3. **LISTEN/NOTIFY** — [postgresql.org/docs/current/sql-notify.html](https://www.postgresql.org/docs/current/sql-notify.html)

### مقالات ومراجع تقنية

 1. **Alex Edwards** — [Configuring sql.DB for Better Performance](https://alexedwards.net/blog/configuring-sqldb)
 2. **go-database-sql.org** — [Tutorial and Reference](http://go-database-sql.org/)
 3. **Three Dots Labs** — [Repository Pattern in Go](https://threedots.tech/)
