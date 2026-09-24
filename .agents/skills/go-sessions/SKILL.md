---
name: go-sessions
description: "Engineering and maintaining our project's Session infrastructure (internal/infrastructure/session or internal/platform/session) based on global best practices, RFC 6265bis, OWASP Session Management, NIST SP 800-63B, and Go 1.23+ CHIPS. Covers stateful session stores (Redis Sentinel/Cluster, PostgreSQL JSONB), client-side encrypted cookies, session ID hashing at rest (SHA-256), cryptographic CSPRNG generation (256-bit entropy), session fixation defense via regeneration, dual timeouts (idle vs absolute), write throttling to reduce cache pressure, concurrent session limiting & device tracking, synchronized CSRF defense, thread-safe context propagation, and slog masking."
---

# مهارة هندسة وإدارة الجلسات (`go-sessions`)

تحدد هذه المهارة المعمارية القياسية الهندسية لبناء، وتطوير، وحماية، وصيانة **البنية التحتية للجلسات (Session Management Infrastructure)** في مشاريع Go الموزعة الكبيرة، استناداً إلى أرقى المعايير العالمية الرسمية الصادرة عن **IETF** (معيار **[RFC 6265bis](https://datatracker.ietf.org/doc/html/draft-ietf-httpbis-rfc6265bis)** ومواصفة **[W3C CHIPS](https://github.com/privacycg/CHIPS)** المدعومة رسمياً في Go 1.23+)، وتوجيهات الأمان من **[OWASP Session Management Cheat Sheet](https://cheatsheetseries.owasp.org/cheatsheets/Session_Management_Cheat_Sheet.html)**، ومعايير الهوية الرقمية **[NIST SP 800-63B](https://pages.nist.gov/800-63-3/sp800-63b.html)**.

تم تصميم هذه المهارة لتكون الأساس الموحد والمحصن لكافة عمليات إدارة حالة المصادقة من جانب الخادم (Stateful Server-Side Sessions) والأنظمة الهجينة (BFF Pattern)، حيث توفر عزلاً تاماً لبيانات الجلسة، وتجزئة تشفيرية عند التخزين لمنع تسريب الجلسات النشطة، وتدعم التوسع الأفقي عبر مخازن الذاكرة الموزعة عالية الأداء (Redis Sentinel / Cluster) وقواعد البيانات العلائقية (PostgreSQL)، وتوفر وسيط HTTP عالي الأداء مع حقن آمن في السياق (`context.Context`)، وتجديد إلزامي للمعرف لمنع تثبيت الجلسات، وحماية متكاملة ضد هجمات CSRF.

---

## بنية ملفات المهارة والحزمة في المشروع (`Skill & Package Layout`)

```text
train_struct/
├── internal/
│   └── infrastructure/
│       └── session/
│           ├── doc.go                  # التوثيق المعماري الرسمي للحزمة وقواعد التبعيات
│           ├── types.go                # هياكل الجلسة، البيانات الوصفية، والواجهات المجردة (Store Interface)
│           ├── manager.go              # مدير الجلسات المركزي (SessionManager) وإدارة السياق
│           ├── cookie.go               # تكوين ملفات الارتباط الصارمة (__Host-, SameSite, CHIPS)
│           ├── crypto.go               # توليد المعرفات العشوائية الآمنة (CSPRNG 256-bit) وتجزئتها (SHA-256)
│           ├── middleware.go           # وسيط net/http للتحقق، تجديد وقت النشاط، وحقن البيانات في السياق
│           ├── csrf.go                 # وسيط الحماية المزدوجة ضد CSRF مع مقارنة الوقت الثابت
│           ├── redis_store.go          # محرك التخزين الموزع عالي الأداء عبر Redis go-redis/v9
│           ├── postgres_store.go       # محرك التخزين المتين عبر PostgreSQL JSONB
│           ├── throttle.go             # كبح كتابة وقت النشاط (Rolling Expiration Throttling)
│           └── session_test.go         # اختبارات التزامن (-race)، الأمان، والقياس المعياري (Benchmarks)
│
└── .agents/skills/go-sessions/         # الدليل المعماري التخصصي للمهارة
    ├── SKILL.md
    ├── docs/                           # الأدلة المعمارية والتطبيقية التفصيلية
    │   ├── 01_session_architecture_and_taxonomy.md
    │   ├── 02_cookie_security_and_rfc6265bis.md
    │   ├── 03_cryptographic_primitives_and_hashing_at_rest.md
    │   ├── 04_session_lifecycle_and_multidevice.md
    │   ├── 05_concurrency_and_write_throttling.md
    │   ├── 06_csrf_mitigation_and_session_pairing.md
    │   ├── 07_testing_and_benchmarking.md
    │   └── 08_platform_skills_integration.md
    ├── examples/                       # كود Go نموذجي جاهز للاستخدام والتطبيق
    │   ├── session_manager.go
    │   ├── redis_store.go
    │   ├── postgres_store.go
    │   ├── csrf_middleware.go
    │   ├── concurrency_throttling.go
    │   └── platform_integration.go
    └── references/                     # مصفوفات الفحص والأنماط المضادة
        ├── antipatterns_matrix.md
        └── production_checklist.md
```

---

## 1. التموضع المعماري داخل المشروع (Architectural Layering)

تنتمي حزمة الجلسات إلى **طبقة البنية التحتية (Infrastructure)** عند اتصالها بمخازن خارجية (Redis أو PostgreSQL)، أو إلى **طبقة المنصة الأساسية (Platform)** عند تجريد واجهات التخزين:

```text
┌────────────────────────────────────────────────────────────────────────┐
│                        Transport / HTTP Handlers                       │
│                         (internal/httphandlers)                        │
│   تطبق session.Middleware و session.CSRFMiddleware وتستخرج هوية المستخدم│
└───────────────────────────────────┬────────────────────────────────────┘
                                    │ يمرر طلبات محقونة بالسياق
                                    ▼
┌────────────────────────────────────────────────────────────────────────┐
│                    Application / Use Cases / Services                  │
│                     (internal/usecases, internal/services)             │
│        تستخرج بيانات الجلسة عبر session.FromContext وتدير الأجهزة       │
└───────────────────────────────────┬────────────────────────────────────┘
                                    │ يعتمد على
                                    ▼
┌────────────────────────────────────────────────────────────────────────┐
│                  Infrastructure Layer (طبقة البنية التحتية)            │
│                  internal/infrastructure/session                       │
│                                                                        │
│  - توليد الرموز العشوائية التشفيرية (256-bit Entropy via crypto/rand)  │
│  - تجزئة المعرف عند التخزين (SHA-256 Hashing at Rest) لمقاومة التسريب  │
│  - إدارة مخازن البيانات الموزعة (Redis Cluster / PostgreSQL JSONB)      │
│  - كبح كتابة وقت النشاط في الذاكرة (Write Throttling) لتقليل الضغط     │
│  - تتبع الجلسات المتعددة وفهرستها حسب المستخدم وطرد الأجهزة الزائدة   │
│  - زرع ملفات الارتباط المؤمنة ببادئة __Host- وخصائص RFC 6265bis         │
│  - التحقق المزدوج من رموز CSRF بمقارنة الوقت الثابت (Constant Time)    │
└───────────────────────────────────┬────────────────────────────────────┘
                                    │ يعتمد على
                                    ▼
┌────────────────────────────────────────────────────────────────────────┐
│                    Platform / Foundation Layer                         │
│                    internal/platform/{errors, context, logger}         │
└────────────────────────────────────────────────────────────────────────┘
```

---

## 2. المبادئ الهندسية الصارمة للمشاريع الكبيرة

1. **الالتزام الصارم بـ RFC 6265bis و W3C CHIPS:**
   - إلزامية البادئة الذهبية **`__Host-`** (تفرض `Secure=true`، مسار `/`، وحظر النطاقات الفرعية).
   - تفعيل `HttpOnly=true` لمنع سرقة الجلسة عبر XSS.
   - ضبط `SameSite=Lax` أو `SameSite=Strict`.
   - استخدام خاصية `Partitioned` في Go 1.23+ لمنع التتبع عبر النطاقات وسياقات الـ iframes.
2. **تجزئة المعرّف عند التخزين (Session ID Hashing at Rest):**
   - حظر تخزين معرّف الجلسة الخام في قاعدة البيانات أو Redis.
   - تخزين `SHA-256(RawToken)` كمعرّف للبحث في المتجر، وتسليم الرمز الخام للعميل فقط؛ لضمان عدم تمكن المهاجمين من انتحال الجلسات حتى في حال تسريب قاعدة البيانات بالكامل.
3. **منع هجوم تثبيت الجلسة (Session Fixation Defense):**
   - استدعاء `RegenerateToken()` فور تسجيل الدخول، أو ترقية الصلاحيات، أو تغيير كلمة المرور.
   - حذف المعرف القديم من التخزين ذرياً ومباشرة.
4. **سياسة الانتهاء المزدوجة (Dual Timeout Policy):**
   - **مهلة الخمول (Idle Timeout):** 15-30 دقيقة تنتهي إن لم ينشط المستخدم.
   - **المهلة المطلقة (Absolute Timeout):** 8-24 ساعة تنتهي عندها الجلسة حتماً وتجبر المستخدم على إعادة تسجيل الدخول مهما كان نشطاً.
5. **إدارة التزامن وكبح الكتابة (Write Throttling):**
   - معالجة طلبات Go المتزامنة في الـ Goroutines.
   - تحديث حقل `LastActiveAt` ووقت التمديد في Redis فقط إذا انقضت دقيقة كاملة على الأقل منذ آخر تحديث، لمنع إغراق Redis بالطلبات في الأنظمة ذات التردد العالي (High QPS).
6. **حوكمة الأجهزة والجلسات المتعددة (Multi-Device Governance):**
   - فهرسة الجلسات عبر مجموعة فرعية `user_sessions:<user_id>`.
   - فرض حد أقصى للجلسات المتزامنة (Max Concurrent Sessions) وحذف الأقدم (FIFO).
   - إتاحة ميزة "تسجيل الخروج من كافة الأجهزة الأخرى" بطلب واحد ذري.
7. **التكامل الإلزامي مع حماية CSRF:**
   - استخدام نمط الرمز المتزامن (Synchronizer Token Pattern) المخزن بالجلسة لجميع طلبات تعديل الحالة (POST/PUT/PATCH/DELETE)، والمقارنة باستخدام `crypto/subtle.ConstantTimeCompare`.

---

## 3. المراحل الست لدورة حياة الجلسة (The 6 Session Lifecycle Stages)

تتبع المهارة أحدث المعايير الدولية عبر 6 مراحل متكاملة:

1. **الإنشاء والسك (Creation & Token Minting):**
   - توليد 32 بايت من العشوائية التشفيرية (`crypto/rand`) كرمز خام.
   - حساب الهاش `SHA-256(RawToken)`.
   - توليد رمز CSRF عشوائي مستقل ومستمر مع الجلسة.
   - تسجيل الجلسة في المتجر مع ضبط `CreatedAt`, `LastActiveAt`, و `AbsoluteExp`.
2. **البث والزرع الآمن (Cookie Dispatching):**
   - إرسال الكوكي عبر الترويسة `Set-Cookie` بالبادئة `__Host-` وخاصيتي `HttpOnly` و `Secure`.
   - حساب عمر الكوكي بالثواني عبر `Max-Age` (الذي له الأسبقية على `Expires`).
3. **التحقق وتغذية السياق (Validation & Context Hydration):**
   - استخراج الرمز من الكوكي عبر الميدلوير.
   - البحث في المتجر باستخدام الهاش.
   - التأكد من عدم تجاوز المهلة المطلقة (`now.After(AbsoluteExp)`).
   - حقن كائن الجلسة في `context.Context` بمفتاح غير مصدّر لمنع تصادم المفاتيح.
4. **التجديد وكبح الكتابة (Activity Throttling & Sliding Expiration):**
   - فحص الفارق الزمني `now.Sub(LastActiveAt)`.
   - إذا تجاوز الفارق 60 ثانية: تحديث `LastActiveAt` وتمديد الـ TTL في Redis لمهلة الخمول، مع مراعاة ألا يتجاوز التمديد موعد الانتهاء المطلق.
5. **تجديد المعرف عند الترقية (Privilege Elevation & Regeneration):**
   - توليد زوج رموز جديد (RawToken و Hash جديدين).
   - نقل بيانات الجلسة للمعرف الجديد وحذف المعرف القديم ذرياً.
   - إعادة كتابة الكوكي للمتصفح بالرمز الجديد.
6. **التدمير والتنظيف (Destruction & Garbage Collection):**
   - محو الجلسة فور تسجيل الخروج من المتجر وإرسال كوكي بمحو فوري (`MaxAge: -1`).
   - تنظيف تلقائي عبر TTL في Redis، أو كنس دوري (Background Worker Sweeper) للجلسات المنتهية في PostgreSQL.

---

## 4. كيفية استخدام وتطبيق المهارة في المشروع

عند تكليف الوكيل ببناء أو فحص أو تعديل نظام الجلسات والمصادقة في المشروع، يجب اتباع الخطوات الهندسية التالية:

1. **مراجعة الأدلة المعمارية التفصيلية:** استشارة الملفات في `docs/` لتحديد نوع المتجر (Redis أو Postgres)، واستراتيجية الكوكيز، وضبط فترات الخمول.
2. **تطبيق الكود النموذجي:** الاستفادة من الملفات الجاهزة في `examples/` لبناء كود خالٍ من الثغرات ومتوافق مع `net/http` و `context.Context` و `slog`.
3. **مراجعة مصفوفة الأنماط المضادة:** فحص الكود بالرجوع إلى `references/antipatterns_matrix.md` لضمان عدم ارتكاب أي من الأخطاء الكارثية.
4. **تدقيق الجاهزية للإنتاج:** استيفاء كافة بنود `references/production_checklist.md` قبل دمج الكود.

---

## 5. مصفوفة التكامل والاعتماديات مع مهارات المنظومة (Platform Skills Integration)

ترتبط مهارة `go-sessions` بعقود معمارية حية مع المهارات الأساسية للمشروع:

| المهارة الأساسية | الحزمة في المشروع (`Package`) | العقد الهندسي للتكامل (`Integration Contract`) |
| :--- | :--- | :--- |
| **`go-context`** | `internal/platform/context` | إثراء `RequestMetadata` بـ `UserID` المستخرج من الجلسة عبر `platformctx.WithUserID`، وحقن الجلسة كاملة بمفتاح غير مصدّر. |
| **`go-errors`** | `internal/platform/errors` | ترجمة حالات الجلسة المنتهية أو غير الصالحة إلى `platformerrors.Unauthorized` (401)، وفشل التحقق من CSRF إلى `platformerrors.Forbidden` (403). |
| **`go-logger-slog`** | `internal/platform/logger` | حظر طباعة الرموز الخام في السجلات، واستخدام `SecretString` أو قناع للتعتيم، وتدوين أحداث تسجيل الدخول والخروج مع ربطها بـ `req_id`. |
| **`go-postgres`** | `internal/infrastructure/postgres` | تنفيذ مخزن الجلسات العلائقي المتين (`PostgresStore`) باستخدام `postgres.DBTX` وعزل أخطاء DB عبر `postgres.TranslateError`. |
| **`go-server-lifecycle`** | `internal/infrastructure/server` | التأكد من اتصال عنقود Redis أو قاعدة البيانات قبل إعلان الجاهزية `/readyz`، وإيقاف عمال كنس الجلسات بنعومة أثناء الـ Graceful Shutdown. |
| **`go-service-configuration`** | `internal/platform/config` | تحميل كائن `SessionConfig` والتحقق من صحة المهل الزمنية والنطاقات، وحماية أسرار تشفير الكوكيز من الظهور في سجلات الإعدادات. |

> للمزيد من التفاصيل والكود النموذجي، راجع الوثيقة المعمارية: [docs/08_platform_skills_integration.md](docs/08_platform_skills_integration.md) والمثال التطبيقي المباشر: [examples/platform_integration.go](examples/platform_integration.go).
