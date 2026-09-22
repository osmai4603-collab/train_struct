---
name: go-postgres
description: "Engineering and maintaining our project's PostgreSQL infrastructure (internal/infrastructure/postgres) based on global best practices. Covers pgxpool connection management, SQLSTATE error translation to platform errors, safe transaction execution with retry, health checks, schema migrations, and strict failure domain isolation."
---

# مهارة البنية التحتية الموحدة لـ PostgreSQL (`go-postgres`)

تحدد هذه المهارة المعمارية القياسية الهندسية لبناء، وتطوير، وصيانة **طبقة البنية التحتية لقاعدة بيانات PostgreSQL (`internal/infrastructure/postgres`)** في مشاريع Go المتقدمة، استناداً إلى أرقى الممارسات العالمية المعتمدة في كبرى الشركات التقنية وتوصيات الفريق المطور لـ `pgx/v5`.

تم تصميم هذه الحزمة لتكون الأساس الموحد والموثوق لكافة عمليات التخزين في المشروع، حيث تعزل تفاصيل بروتوكول قاعدة البيانات، وتترجم أخطاء SQLSTATE تلقائياً إلى أخطاء نطاق المنصة (`internal/platform/errors`)، وتوفر إدارة محكمة للمجمعات (`pgxpool`)، وتدعم المعاملات الذرية مع إعادة المحاولة الآلية (`Exponential Backoff with Full Jitter`)، وتدير ترحيل المخططات (`golang-migrate`) وفحص الجاهزية.

---

## بنية الحزمة في المشروع (`internal/infrastructure/postgres/`)

```text
train_struct/
├── internal/
│   └── infrastructure/
│       └── postgres/
│           ├── doc.go              # التوثيق المعماري الرسمي للحزمة وقواعد التبعيات
│           ├── pool.go             # إدارة وتوليف مجمع الاتصالات pgxpool.Pool
│           ├── errors.go           # عزل نطاق الفشل وترجمة SQLSTATE إلى أخطاء المنصة
│           ├── tx.go               # تنفيذ المعاملات الذرية بأمان وواجهة DBTX الموحدة
│           ├── retry.go            # آلية إعادة المحاولة مع Exponential Backoff و Full Jitter
│           ├── health.go           # فحص الجاهزية وجمع المقاييس اللحظية للمجمع
│           ├── migrate.go          # تشغيل وتراجع ترحيلات المخطط عبر golang-migrate و go:embed
│           └── postgres_test.go    # اختبارات الوحدة الشاملة وتأكيد التوافق
│
└── .agents/skills/go-postgres/     # الدليل المعماري التخصصي للمهارة
    ├── SKILL.md
    ├── docs/                       # الأدلة المعمارية والتطبيقية
    │   ├── 01_architecture_and_pool_management.md
    │   ├── 02_error_translation_and_failure_domains.md
    │   ├── 03_transactions_and_retry.md
    │   ├── 04_migrations_and_health.md
    │   └── 05_testing_strategies.md
    ├── examples/                   # كود Go نموذجي جاهز للاستخدام والتطبيق
    │   ├── pool_setup.go
    │   ├── repository_with_translation.go
    │   ├── transaction_usage.go
    │   ├── migration_embed.go
    │   └── health_check.go
    └── references/                 # مصفوفات الفحص والأنماط المضادة
        ├── antipatterns_matrix.md
        └── production_checklist.md
```

---

## 1. التموضع المعماري داخل المشروع

تنتمي هذه الحزمة إلى **طبقة البنية التحتية (Infrastructure Layer)** في أسفل التسلسل الهرمي للمشروع:

```text
┌────────────────────────────────────────────────────────────────────────┐
│                        Transport / HTTP Handlers                       │
│                         (internal/httphandlers)                        │
└───────────────────────────────────┬────────────────────────────────────┘
                                    │ يستدعي
                                    ▼
┌────────────────────────────────────────────────────────────────────────┐
│                    Application / Use Cases / Services                  │
│                     (internal/usecases, internal/services)             │
│   تنسق العمليات وتدير المعاملات الموزعة عبر postgres.ExecTxWithRetry   │
└───────────────────────────────────┬────────────────────────────────────┘
                                    │ يستدعي عبر واجهة المستودع (Repository)
                                    ▼
┌────────────────────────────────────────────────────────────────────────┐
│                        Domain Layer (Entities & Repos)                 │
│                        (internal/domain, repository interfaces)        │
│          لا تعرف أي شيء عن pgx أو PostgreSQL أو SQL نهائياً            │
└───────────────────────────────────▲────────────────────────────────────┘
                                    │ تطبق الواجهات
                                    │
┌───────────────────────────────────┴────────────────────────────────────┐
│                        Persistence / Repositories                      │
│                  (internal/storage/user, storage/auth)                 │
│   تستقبل postgres.DBTX وتترجم أخطاء قاعدة البيانات عبر TranslateError  │
└───────────────────────────────────┬────────────────────────────────────┘
                                    │ يعتمد على
                                    ▼
┌────────────────────────────────────────────────────────────────────────┐
│                   Infrastructure Layer (طبقة البنية التحتية)            │
│                   internal/infrastructure/postgres                     │
│                                                                        │
│  - إدارة مجمع الاتصالات pgxpool.Pool وتوليفه للإنتاج                   │
│  - عزل أخطاء بروتوكول قاعدة البيانات (pgconn.PgError / SQLSTATE)       │
│  - واجهة DBTX المرنة المتوافقة مع الـ Pool والمعاملات                  │
│  - خوارزمية التراجع الأسي مع التشويش الكامل (Full Jitter Backoff)       │
│  - ترحيل المخططات المضمنة go:embed وفحص الصحة التلقائي                 │
└───────────────────────────────────┬────────────────────────────────────┘
                                    │ يعتمد على
                                    ▼
┌────────────────────────────────────────────────────────────────────────┐
│                    Platform / Foundation Layer                         │
│                    internal/platform/{errors, config, context}         │
└────────────────────────────────────────────────────────────────────────┘
```

### قواعد التبعيات الصارمة

1. **لا استيراد عكسي أبداً**: يحظر على حزمة `postgres` استيراد أي طبقة أعلى (`domain`, `usecases`, `services`, `httphandlers`, `storage`).
2. **عزل تفاصيل السائق**: لا يجوز لأي طبقة أعلى من مستودعات التخزين (`storage`) التعامل مع أنواع السائق المباشرة مثل `*pgconn.PgError` أو استيراد `github.com/jackc/pgerrcode`.
3. **الاعتماد على واجهات التنفيذ**: تعتمد مستودعات البيانات على واجهة `postgres.DBTX` حتى يسهل تشغيل الاستعلامات الفردية والمعاملات المجمعة بنفس المنطق، كما يسهل إنشاء Mock للاختبارات.

---

## 2. جدول ترجمة أخطاء SQLSTATE إلى أخطاء المنصة

تقوم الدالة المركزية `postgres.TranslateError(op, err)` بتحويل أخطاء المحرك البرمجي إلى كائنات `*platformerr.Error`:

| رمز SQLSTATE | الاسم المعياري | كود المنصة | كود HTTP | التوصيف والمعالجة |
| :--- | :--- | :--- | :--- | :--- |
| `pgx.ErrNoRows` | لا توجد صفوف | `NOT_FOUND` | 404 | المورد المطلوب غير موجود في قاعدة البيانات |
| `23505` | UniqueViolation | `CONFLICT` | 409 | انتهاك قيد الفرادة (مفتاح مكرر مثل البريد أو اسم المستخدم) |
| `23503` | ForeignKeyViolation | `INVALID` | 400 | انتهاك المفتاح الأجنبي (مرجع لمعرف غير موجود) |
| `23502` | NotNullViolation | `INVALID` | 400 | عمود غير قابل للقيمة الفارغة تم تمرير null له |
| `23514` | CheckViolation | `INVALID` | 400 | انتهاك لقيد الفحص الشرطي في الجدول |
| `22003 / 22P02` | DataException | `INVALID` | 400 | قيمة خارج النطاق أو نوع بيانات نصي غير متوافق |
| `40001` | SerializationFailure | `UNAVAILABLE` | 503 | فشل تسلسل المعاملة التزامنية (**Transient - قابلة لإعادة المحاولة**) |
| `40P01` | DeadlockDetected | `UNAVAILABLE` | 503 | تم اكتشاف طريق مسدود بين معاملتين (**Transient - قابلة لإعادة المحاولة**) |
| `57P01..03` | Shutdown/CannotConnect | `UNAVAILABLE` | 503 | خادم قاعدة البيانات قيد الإيقاف أو ممتلئ الاتصالات |
| `42501` | InsufficientPrivilege | `FORBIDDEN` | 403 | المستخدم الموثق لا يملك صلاحية كافية في المحرك |
| `net.Error` | Network Error | `UNAVAILABLE` | 503 | انقطاع اتصال الشبكة بمخدم قاعدة البيانات |
| `context.Canceled` | Context Timeout | `TIMEOUT` | 504 | انتهاء المهلة أو إلغاء العميل لطلب HTTP |
| أي خطأ آخر | Unclassified | `INTERNAL` | 500 | خطأ غير متوقع في محرك التخزين (يُحجب عن العميل) |

---

## 3. المبادئ الحاكمة الأساسية (Core Tenets)

1. **التعامل المباشر مع `pgx/v5`**:
   تفضيل واجهات `pgx` المباشرة على محول `database/sql` لتحقيق أعلى أداء، ودعم ميزات التخزين المؤقت للاستعلامات التلقائي (Automatic Prepared Statements Caching)، واستخدام `CopyFrom` لعمليات الإدخال الدفعي الكبيرة.
2. **عزل نطاق الفشل (Failure Domain Isolation)**:
   لا يخرج أي خطأ من نوع `pgconn.PgError` إلى طبقة الـ Service أو الـ Handler. كل خطأ يجب أن يمر بـ `postgres.TranslateError(op, err)`.
3. **معاملات آمنة وغير قابلة للتسريب (Leak-Proof Transactions)**:
   تُنفذ المعاملات حصراً عبر `postgres.ExecTx` أو `postgres.ExecTxWithRetry` مع وجود `defer Rollback` الإلزامي و `recover` لتفادي ترك اتصالات معلقة أو إقفالات (locks) غير محررة.
4. **تجنب تجمهر الاتصالات (Thundering Herd Prevention)**:
   إعادة المحاولة في `postgres.WithRetry` تستخدم **Exponential Backoff مع Full Jitter** لمنع هجوم كل الحالات المتراجعة على الخادم في نفس اللحظة.
5. **توليف مدروس للمجمّع (Production-Ready Pool Tuning)**:
   تعيين `MaxConns` و `MinConns` و `MaxConnLifetime` و `MaxConnIdleTime` استناداً إلى معادلة سعة الخادم والأنوية المتاحة.
6. **ترحيل تلقائي للمخطط عبر الثنائي (Embedded Migrations)**:
   تضمين ملفات SQL للترحيل عبر `go:embed` وتنفيذها تلقائياً عند إقلاع الخدمة لضمان تزامن شفرة التطبيق مع مخطط التخزين.

---

## 4. الاعتماديات والتكاملات بين المهارات (Cross-Skill Dependencies & Integrations)

تحتل هذه المهارة حيّزاً محورياً في مجموعة مهارات المنصة: فهي المستهلك الأساسي لأخطاء المنصة، والسياق، وإعدادات قاعدة البيانات، والتسجيل المهيكل. يوضح الرسم أدناه اتجاهات الاعتماد:

```text
go-service-configuration ─► go-postgres (إعدادات بركة الاتصالات، رابط الاتصال، الحجب)
go-context                ─► go-postgres (تمرير ctx كأول معامل، ميزانية المهلة، الإلغاء)
go-errors                 ─► go-postgres (ترجمة SQLSTATE إلى أخطاء المنصة)
go-logger-slog            ─► go-postgres (تسجيل تهيئة المجمع وفحوصات الجاهزية)
go-postgres               ─► go-server-lifecycle (إغلاق المجمع عند الإيقاف، فحص الجاهزية)
```

### المهارات التي تعتمد عليها هذه المهارة (Downstream Dependencies)

| المهارة | نوع الاعتماد | نقاط التكامل الرئيسية |
| :--- | :--- | :--- |
| `go-service-configuration` | يوفر قيم `DatabaseSettings` وحدود بركة الاتصالات ورابط الاتصال مع حجب الأسرار | `Driver/Host/Port/Name/User/Password/DatabaseURL`, `MaxOpenConns`, `MinConns`, `MaxConnLifetime` |
| `go-errors` | ترجمة أخطاء SQLSTATE (`pgconn.PgError`) إلى أخطاء نطاق المنصة قبل تجاوز حدود الحزمة | `postgres.TranslateError(op, err)` ← `platformerr.NotFound/Conflict/Invalid/Unavailable/...` وفق خريطة الأكواد 404/409/400/503 |
| `go-context` | تمرير `context.Context` كأول معامل، وميزانية المهل الزمنية، والإلغاء الفوري للاستعلامات | واجهة `DBTX` المعتمدة على `ctx`, `platformctx.RequireMinimumBudget`, `QueryRowContext/ExecContext` |
| `go-logger-slog` | تسجيل تهيئة المجمع وفحوصات الجاهزية والمقاييس بلوغرات فرعية خاصة بمكوّن `postgres` | `logger.With(slog.String("component", "postgres"))`, تسجيل `pool.Stat()` ومقاييس التجمّع |

### التكامل مع المهارات الأخرى (Upstream Integrations)

| المهارة | نوع التكامل | نقاط التكامل الرئيسية |
| :--- | :--- | :--- |
| `go-server-lifecycle` | إدارة عمر المجمع بما ينسجم مع المراحل الثماني للخدمة | فتح الاتصال في Phase 1 (fail-fast)، دعم فحص الجاهزية في `/readyz`، إيقاف الـ workers ثم `dbPool.Close()` في Phase 8 (الترتيب العكسي) |
| طبقات التخزين (`internal/storage`) | المستودعات تستهلك `postgres.DBTX` وتترجم الأخطاء عبر `TranslateError` داخل حدود الـ Repository | [examples/repository_with_translation.go](./examples/repository_with_translation.go), واجهة `DBTX` للاستعلامات الفردية والمعاملات |

---

## 5. الفهرس والروابط للأدلة والأمثلة

### الوثائق المعمارية التفصيلية

- [`docs/01_architecture_and_pool_management.md`](file:///home/osm/StudioProjects/train_struct/.agents/skills/go-postgres/docs/01_architecture_and_pool_management.md) — معمارية الطبقة وإدارة وتوليف مجمع اتصالات pgxpool.
- [`docs/02_error_translation_and_failure_domains.md`](file:///home/osm/StudioProjects/train_struct/.agents/skills/go-postgres/docs/02_error_translation_and_failure_domains.md) — خريطة ترجمة أخطاء SQLSTATE ومبدأ عزل نطاق الفشل.
- [`docs/03_transactions_and_retry.md`](file:///home/osm/StudioProjects/train_struct/.agents/skills/go-postgres/docs/03_transactions_and_retry.md) — تنفيذ المعاملات الذرية، واجهة DBTX، وإعادة المحاولة مع التراجع الأسي والتشويش.
- [`docs/04_migrations_and_health.md`](file:///home/osm/StudioProjects/train_struct/.agents/skills/go-postgres/docs/04_migrations_and_health.md) — ترحيل المخططات المضمنة go:embed وفحص الصحة والمقاييس.
- [`docs/05_testing_strategies.md`](file:///home/osm/StudioProjects/train_struct/.agents/skills/go-postgres/docs/05_testing_strategies.md) — استراتيجيات الاختبار واختبارات التكامل باستخدام Testcontainers ومحاكاة واجهة DBTX.

### الأمثلة العملية البرمجية

- [`examples/pool_setup.go`](file:///home/osm/StudioProjects/train_struct/.agents/skills/go-postgres/examples/pool_setup.go) — إنشاء وتوليف مجمع الاتصالات من إعدادات المنصة وتسجيله عبر slog.
- [`examples/repository_with_translation.go`](file:///home/osm/StudioProjects/train_struct/.agents/skills/go-postgres/examples/repository_with_translation.go) — بناء مستودع تخزين نموذجي يعتمد على DBTX ويترجم كافة الأخطاء.
- [`examples/transaction_usage.go`](file:///home/osm/StudioProjects/train_struct/.agents/skills/go-postgres/examples/transaction_usage.go) — تنفيذ معاملات نقدية وحسابية ذرية مع إعادة المحاولة التلقائية.
- [`examples/migration_embed.go`](file:///home/osm/StudioProjects/train_struct/.agents/skills/go-postgres/examples/migration_embed.go) — تضمين وتشغيل ترحيلات SQL باستخدام embed.FS.
- [`examples/health_check.go`](file:///home/osm/StudioProjects/train_struct/.agents/skills/go-postgres/examples/health_check.go) — ربط فحص جاهزية قاعدة البيانات بنظام الصحة المركزي ونشر مقاييس المجمع.

### المراجع وقوائم التحقق

- [`references/antipatterns_matrix.md`](file:///home/osm/StudioProjects/train_struct/.agents/skills/go-postgres/references/antipatterns_matrix.md) — مصفوفة الأنماط المضادة الشائعة في مشاريع Go مع PostgreSQL وكيفية تجنبها.
- [`references/production_checklist.md`](file:///home/osm/StudioProjects/train_struct/.agents/skills/go-postgres/references/production_checklist.md) — قائمة فحص الجاهزية للإنتاج لطبقة التخزين.
