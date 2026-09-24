---
name: go-jwt
description: "Engineering and maintaining our project's JWT infrastructure (internal/infrastructure/jwt or internal/platform/jwt) based on global best practices, RFC 7519, RFC 7515 (JWS), RFC 7516 (JWE), RFC 7517 (JWKS), and RFC 8725 (BCP 225). Covers cryptographic algorithm selection (Ed25519, PS256, HMAC), automated thread-safe JWKS caching, zero-downtime key rotation, refresh token rotation (RTR) with reuse detection, DPoP and mTLS sender-constrained tokens, defense against algorithm confusion & kid injection, high-performance low-allocation middleware, type-safe context propagation, and slog masking."
---

# مهارة هندسة وإدارة رموز الويب الموثقة (`go-jwt`)

تحدد هذه المهارة المعمارية القياسية الهندسية لبناء، وتطوير، وصيانة **البنية التحتية لرموز الويب الموثقة (JWT / JOSE Infrastructure)** في مشاريع Go المتقدمة، استناداً إلى أرقى المعايير العالمية الرسمية الصادرة عن **IETF** (خاصة معيار **RFC 8725: BCP 225**)، وتوصيات التشفير للغة Go (`pkg.go.dev/crypto`)، ودليل أمان **OWASP** و **NIST SP 800-63B**.

تم تصميم هذه الحزمة لتكون الأساس الموحد والمحصن لكافة عمليات المصادقة والتفويض ونقل الادعاءات الآمنة في النظام الموزع، حيث تعزل التعقيدات التشفيرية، وتوفر إدارة محكمة وتلقائية لمجموعات المفاتيح (`JWKS`) مع كاش آمن التزامن، وتدعم التدوير الخالي من التوقف (`Zero-Downtime Rotation`)، وتطبق تدوير رموز التحديث مع كاشف إعادة الاستخدام (`RTR with Reuse Detection`)، وتوفر وسيط HTTP عالي الأداء مع حقن آمن في السياق (`context.Context`) وتعتيم صارم للرموز في السجلات (`slog.LogValuer`).

---

## بنية ملفات المهارة والحزمة في المشروع (`Skill & Package Layout`)

```text
train_struct/
├── internal/
│   └── infrastructure/
│       └── jwt/
│           ├── doc.go              # التوثيق المعماري الرسمي للحزمة وقواعد التبعيات
│           ├── types.go            # الادعاءات القياسية المخصصة (Custom Claims) والأنواع المشتركة
│           ├── validator.go        # محرك التدقيق والتحقق من التواقيع وصلاحية الادعاءات
│           ├── signer.go           # محرك التوقيع الرقمي (Ed25519/PS256) وتوليد الرموز
│           ├── jwks.go             # إدارة الكاش المحلي لـ JWKS والتحديث الدوري في الخلفية
│           ├── rotation.go         # محرك تدوير رموز التحديث (RTR) وكشف الاختراق
│           ├── revocation.go       # واجهة إدارة القوائم السوداء (Redis / Bloom Filter / Epoch)
│           ├── middleware.go       # وسيط net/http لاستخراج الرموز وحقنها في السياق
│           ├── sensitive.go        # تطبيق slog.LogValuer لمنع تسريب الرموز في السجلات
│           └── jwt_test.go         # اختبارات الأمان والتحقق والقياس المعياري (Benchmarks)
│
└── .agents/skills/go-jwt/          # الدليل المعماري التخصصي للمهارة
    ├── SKILL.md
    ├── docs/                       # الأدلة المعمارية والتطبيقية التفصيلية
    │   ├── 01_jose_architecture_and_token_taxonomy.md
    │   ├── 02_cryptographic_primitives_and_algorithms.md
    │   ├── 03_jwks_infrastructure_and_key_rotation.md
    │   ├── 04_token_lifecycle_and_revocation.md
    │   ├── 05_http_middleware_and_context_propagation.md
    │   ├── 06_security_mitigations_and_rfc8725.md
    │   ├── 07_testing_and_benchmarking.md
    │   └── 08_platform_skills_integration.md
    ├── examples/                   # كود Go نموذجي جاهز للاستخدام والتطبيق
    │   ├── jwks_validator.go
    │   ├── token_service.go
    │   ├── refresh_rotation.go
    │   ├── middleware.go
    │   ├── sensitive_token.go
    │   └── platform_integration.go
    └── references/                 # مصفوفات الفحص والأنماط المضادة
        ├── antipatterns_matrix.md
        └── production_checklist.md
```

---

## 1. التموضع المعماري داخل المشروع (Architectural Layering)

تنتمي حزمة الـ JWT إما إلى **طبقة المنصة الأساسية (Platform)** إن كانت مجرد مكتبة تشفير وتحقق نقية، أو إلى **طبقة البنية التحتية (Infrastructure)** عند اتصالها بشبكات خارجية (خادم JWKS) أو مخازن الذاكرة (Redis Revocation Cache):

```text
┌────────────────────────────────────────────────────────────────────────┐
│                        Transport / HTTP Handlers                       │
│                         (internal/httphandlers)                        │
│          تطبق jwt.Middleware لحماية المسارات واستخراج هوية المستخدم     │
└───────────────────────────────────┬────────────────────────────────────┘
                                    │ يمرر طلبات محقونة بالسياق
                                    ▼
┌────────────────────────────────────────────────────────────────────────┐
│                    Application / Use Cases / Services                  │
│                     (internal/usecases, internal/services)             │
│        تستخرج Claims عبر jwt.UserFromContext وتصدر رموز التحديث        │
└───────────────────────────────────┬────────────────────────────────────┘
                                    │ يعتمد على
                                    ▼
┌────────────────────────────────────────────────────────────────────────┐
│                  Infrastructure Layer (طبقة البنية التحتية)            │
│                  internal/infrastructure/jwt (أو platform/jwt)         │
│                                                                        │
│  - التحقق الصارم من التواقيع الرقمية والتشفير (JWS / JWE / JWKS)       │
│  - كاش محلي لـ JWKS مع تحديث دوري بالخلفية وآلية مقاومة للـ DDoS        │
│  - تدوير رموز التحديث مع كشف إعادة الاستخدام وإلغاء العائلات المخترقة   │
│  - إدارة القوائم السوداء للـ JTI عبر Redis مع انتهاء زمني ذاتي (TTL)   │
│  - وسيط HTTP منيع يعتمد على مفاتيح سياق غير مصدّرة (Unexported Key)    │
│  - تعتيم الرموز في السجلات تلقائياً عبر log/slog.LogValuer             │
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

1. **الالتزام المطلق بـ [RFC 8725 (BCP 225)](https://datatracker.ietf.org/doc/html/rfc8725):**
   - الخادم المتلقي هو صاحب الكلمة الحصرية في تحديد الخوارزمية عبر قائمة بيضاء صريحة (`jwt.WithValidMethods`).
   - المنع التام للرموز غير الموقعة (`alg: "none"`).
   - التحقق الإلزامي من كافة الادعاءات الزمنية (`exp`, `nbf`, `iat`) والتعريفية (`iss`, `aud`).
2. **المفاضلة التشفيرية الحديثة:**
   - **Ed25519 / EdDSA** كخيار قياسي أول للمشاريع الحديثة (توقيع حتمي يمنع تسريب المفاتيح، ومناعة ضد هجمات القنوات الجانبية، ومفاتيح مدمجة 32 بايت).
   - **PS256 (RSASSA-PSS)** كبديل موثوق إذا كان النظام ملزماً بدعم معيار RSA.
   - تجنب **RS256 القديم** و **HS256** في الأنظمة الموزعة لتجنب تسريب المفاتيح المشتركة بين الميكروسيرفيسز.
3. **أعمار الرموز والتصميم الهجين:**
   - قصر عمر رموز الوصول (Access Tokens) ليكون بين **5 إلى 15 دقيقة**.
   - استخدام رموز تحديث (Refresh Tokens) مع تدوير إلزامي عند كل استهلاك (**Refresh Token Rotation - RTR**) مع إلغاء العائلة فورياً عند كشف تكرار الاستخدام.
4. **أمان النقل والتخزين:**
   - حظر تخزين الرموز في `localStorage` بالمتصفحات، وفرض ملفات الارتباط ذات البادئة **`__Host-`** وخاصية `HttpOnly` و `SameSite=Strict`.
   - استخدام الترويسة القياسية `Authorization: Bearer <token>` في واجهات الـ REST API وتطبيقات الهاتف.
5. **الأداء ومكافحة استهلاك الذاكرة في Go:**
   - وسيط مصادقة بنمط Zero-Allocation وتفادي استخدام الانعكاس (`reflect`) غير الضروري.
   - استخدام نوع سياق غير مصدّر `type contextKey struct{}` لمنع تصادم المفاتيح بين الحزم البرمجية.
   - منع تسريب الرموز في السجلات الموزعة (Datadog / ELK) عبر تغليف الرمز بواجهة `slog.LogValuer`.

---

## 3. تصنيف أنواع ومستويات الـ JWT في المهارة

تغطي هذه المهارة كافة الأنواع والمستويات التشفيرية والمعمارية:

### المستويات التشفيرية (Cryptographic Tiers)

1. **Unsecured Tokens (`alg: none`):** ممنوعة ومرفوضة قطعياً في أي بيئة إنتاجية.
2. **Signed Tokens (JWS):** توقيع رقمي لضمان النزاهة والهوية (المعيار الشائع).
3. **Encrypted Tokens (JWE):** تشفير كامل للحمولة لضمان السرية والخصوصية للبيانات الحساسة (PII).
4. **Nested Tokens (JWS داخل JWE):** توقيع البيانات أولاً ثم تشفيرها بالكامل (Sign-then-Encrypt) لأعلى مستويات الحماية.

### المستويات المعمارية ودورات الحياة (Architectural Roles)

1. **رموز الهوية (ID Tokens):** لتعريف المستخدم لدى الواجهة الأمامية (OIDC) — يُحظر استخدامها كتفويض لواجهات الـ API.
2. **رموز الوصول (Access Tokens - RFC 9068):** رموز قصيرة الأجل (5-15 دقيقة) مصممة لحماية الموارد.
3. **رموز التحديث (Refresh Tokens):** رموز طويلة الأجل تخضع لنظام التدوير الإلزامي (RTR) وعائلات الجلسات.
4. **الرموز المقيدة بحائزها (Sender-Constrained Tokens):**
   - **DPoP (RFC 9449):** ربط الرمز ببصمة المفتاح العام للمتصفح/الموبايل لمنع استغلال الرمز حتى لو سُرق.
   - **mTLS (RFC 8705):** ربط الرمز بشهادة الـ X.509 بين الخوادم السحابية.
5. **الرموز الأحادية المؤقتة (Ephemeral Action Tokens):** لتفعيل الحسابات، واستعادة كلمة المرور، والروابط السحرية، مرتبطة بحقبة زمنية (User Epoch) و `jti` وحيد الاستهلاك.
6. **رموز الاتصال بين الخدمات (M2M / Service-to-Service):** معتمدة على OAuth2 Client Credentials أو SPIFFE/SPIRE JWT-SVIDs.

### المراحل الست لدورة حياة الرمز (The 6 Production Lifecycle Stages)

تتبع المهارة أحدث المعايير الدولية (**IETF**, **NIST SP 800-63B**, **OAuth 2.0 Security BCP**) عبر 6 مراحل متكاملة:

1. **التوليد والإصدار (Minting & Issuance):** توقيع أزواج الرموز (Ed25519/PS256) مع تقييد رمز الوصول بـ 5-15 دقيقة، وتضمين حقل النوع الصريح `"typ": "at+jwt"`، وربط بصمات المفاتيح (DPoP / mTLS).
2. **النقل والحفظ الآمن (Transport & Storage):** تشفير HTTPS حصري، وحظر التخزين في `localStorage`، واستخدام ملفات ارتباط ببادئة `__Host-` وخاصية `HttpOnly` و `SameSite=Strict`.
3. **التحقق والاستهلاك (Validation & Consumption):** فحص التواقيع محلياً عبر كاش JWKS في الذاكرة، والتأكد من مطابقة الجمهور `aud`، وسماحية توقيت (Clock Skew) لا تتجاوز 60 ثانية، وفحص القوائم السوداء.
4. **التجديد والتدوير المستمر (Renewal & RTR with Grace Period):** التجديد الاستباقي (عند 75-80% من عمر الرمز)، وتدوير رموز التحديث أحادية الاستخدام، مع تطبيق **مهلة السماح للسباق الشبكي (Grace Period من 10-30 ثانية)** لمنع إنذارات الاختراق الخاطئة عند تكرار طلبات الموبايل المتزامنة، ونسف عائلة الجلسة فور تجاوز المهلة.
5. **الإلغاء والإنهاء المبكر (Revocation & Early Termination):** الإلغاء الفردي لـ `jti` عبر Redis مع انتهاء زمني ذاتي (TTL)، والإلغاء الشامل عبر حقبة المستخدم `user.token_valid_after` في $O(1)$، وإلغاء العائلات المخترقة.
6. **التقادم والكنس التلقائي (Eviction & Garbage Collection):** انتهاء صلاحية رموز الوصول دون استهلاك موارد، والحذف التلقائي لمفاتيح Redis المنتهية، وكنس سجلات الجلسات التاريخية المنتهية في PostgreSQL عبر عمال الخلفية (Background Workers).

---

## 4. كيفية استخدام وتطبيق المهارة في المشروع

عند تكليف الوكيل ببناء أو فحص أو تعديل نظام المصادقة والـ JWT في المشروع، يجب اتباع الخطوات الهندسية التالية:

1. **مراجعة الأدلة المعمارية التفصيلية:** استشارة الملفات في `docs/` لتحديد الخوارزمية، وآلية تدوير المفاتيح، وبنية الكاش المناسبة.
2. **تطبيق الكود النموذجي:** الاستفادة من الملفات الجاهزة في `examples/` لبناء كود خالٍ من الثغرات ومتوافق مع `net/http` و `context.Context` و `slog`.
3. **مراجعة مصفوفة الأنماط المضادة:** فحص الكود بالرجوع إلى `references/antipatterns_matrix.md` لضمان عدم ارتكاب أي من الأخطاء الكارثية العشرة.
4. **تدقيق الجاهزية للإنتاج:** استيفاء كافة بنود `references/production_checklist.md` قبل دمج الكود.

---

## 5. مصفوفة التكامل والاعتماديات مع مهارات المنظومة (Platform Skills Integration)

ترتبط مهارة `go-jwt` بعقود معمارية حية مع المهارات الست الأساسية للمشروع:

| المهارة الأساسية | الحزمة في المشروع (`Package`) | العقد الهندسي للتكامل (`Integration Contract`) |
| :--- | :--- | :--- |
| **`go-context`** | `internal/platform/context` | إثراء `RequestMetadata` بـ `UserID` و `TenantID` عبر `platformctx.WithUserID` و `platformctx.WithTenantID` فور نجاح فحص الرمز. |
| **`go-errors`** | `internal/platform/errors` | ترجمة أخطاء مكتبات التشفير إلى `platformerrors.Unauthorized` (401) و `platformerrors.Forbidden` (403) و `platformerrors.Invalid` (400). |
| **`go-logger-slog`** | `internal/platform/logger` | حماية وتعتيم الرموز الحساسة عبر `platformlogger.Sensitive[T]` و `SecretString` وربط السجلات بالسياق عبر `platformlogger.FromContext`. |
| **`go-postgres`** | `internal/infrastructure/postgres` | تنفيذ مستودع جلسات تدوير الرموز (RTR Session Repository) باستخدام `postgres.DBTX` وعزل أخطاء DB عبر `postgres.TranslateError`. |
| **`go-server-lifecycle`** | `internal/infrastructure/server` | التهيئة الاستباقية لكاش JWKS أثناء الإقلاع، وحصر وسيط المصادقة في خادم الـ Public API مع عزل مسارات الإدارة و `/livez` و `/readyz`. |
| **`go-configuration-json`** | `internal/platform/config` | تحميل كائن `JWTConfig` والتحقق الصارم من صحته مع إخفاء المفاتيح التشفيرية السرية بـ `***` في سجلات الإعدادات. |

> للمزيد من التفاصيل والكود النموذجي، راجع الوثيقة المعمارية: [docs/08_platform_skills_integration.md](docs/08_platform_skills_integration.md) والمثال التطبيقي المباشر: [examples/platform_integration.go](examples/platform_integration.go).
