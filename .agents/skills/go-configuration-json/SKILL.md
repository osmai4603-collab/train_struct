---
name: go-service-configuration
description: "Production-ready configuration engineering for Go HTTP services. Covers layered configuration resolution (defaults < file < env < CLI < overrides), fail-fast validation with multi-error aggregation, secret security and redaction (***), custom duration/size parsing, atomic file persistence (0600 permissions), and declarative schema introspection for self-documenting services."
---

# مهارة هندسة تكوين خدمات Go (Go Service Configuration Skill)

تحدد هذه المهارة المعمارية القياسية والمتبعة لإنشاء وإدارة أنظمة التكوين والإعدادات (Configuration Engineering) لخدمات Go HTTP في بيئات الإنتاج الفعلية. ترتكز هذه المعمارية على نظام يعتمد على نمط المحركات المتخصصة (Engine Pattern)؛ لضمان المرونة، والأمان العالي، والتحقق الاستباقي الشامل (Fail-Fast)، وحماية البيانات الحساسة والأسرار من التسرب، ودعم بيئات النشر المتعددة (التطوير المحلي، الخوادم السحابية، الحاويات Docker، وKubernetes).

---

## بنية الملفات ومواقع البناء في المشروع (Project File Layout)

تم بناء وتنظيم حزمة التكوين `config` في مشروعنا بدقة داخل المسار `internal/platform/config/`، وتتوزع وظائف النظام عبر الملفات التالية:

```text
train_struct/
├── config/
│   └── config.json                  # ملف التكوين الثابت الافتراضي (JSON Store)
├── .env.example                     # نموذج المتغيرات البيئية المستخرج آلياً
├── internal/
│   └── platform/
│       └── config/
│           ├── engine.go            # المحرك المركزي (Orchestrator) وإدارة الحالة الذرية (atomic.Pointer)
│           ├── pipeline_engine.go   # محرك خط أنابيب التحميل متعدد الطبقات (5 طبقات)
│           ├── validation_engine.go # محرك الفحص الاستباقي الشامل وتجميع الأخطاء (Multi-Error Report)
│           ├── security_engine.go   # محرك حجب الأسرار وتشفير الروابط (Redaction & Masking)
│           ├── persistence_engine.go# محرك الحفظ الذري للملفات بصلاحيات مشددة (0600)
│           ├── schema_engine.go     # محرك استخراج التوثيق الذاتي عبر Struct Tags وإنشاء .env.example
│           ├── types.go             # الأنواع المخصصة المقاومة للأخطاء (Duration, ByteSize, SecretString)
│           ├── configuration.go     # هياكل البيانات ونماذج الإعدادات والقيم الافتراضية
│           ├── options.go           # نمط الخيارات الوظيفية (Functional Options Pattern)
│           └── engine_test.go       # حزمة اختبارات الوحدة الشاملة مع فحص التزامن (-race)
```

---

## المبادئ الستة لهندسة التكوين في بيئات الإنتاج

1. **خط أنابيب التدرج متعدد الطبقات (Layered Resolution Pipeline)**:
   لا يتم سحب الإعدادات من مصدر وحيد مطلقاً، بل تُدمج طبقات متعددة بترتيب أسبقية حتمي وصارم:
   $$\text{Defaults} < \text{JSON File} < \text{Environment Variables} < \text{CLI Flags} < \text{Runtime Overrides}$$

2. **التحقق الاستباقي الشامل وتجميع الأخطاء (Fail-Fast Multi-Error Validation)**:
   يُحظر تشغيل الخدمة إذا كان هناك أي خلل في الإعدادات. يتم فحص كل القيود المنطقية دفعة واحدة في مرحلة التهيئة الأولى (Phase 1)، وتُجمع كافة الأخطاء في تقرير تشخيصي موحد بدلاً من إيقاف التشغيل عند أول خطأ فقط.

3. **حماية الأسرار والبيانات الحساسة (Strict Secret Security & Redaction)**:
   كلمات المرور، ومفاتيح التوقيع، وبيانات الاتصال يجب ألا تظهر كنصوص واضحة في السجلات (Logs) أو الـ stdout أو ردود JSON التشخيصية. تُحجب الحقول تلقائياً (`***`) وتُحدد صلاحيات الملفات بنظام POSIX الصارم (`0600`).

4. **التحليل المرن للأنواع المخصصة (Resilient Type Parsing)**:
   يكتب مشغلو الأنظمة المُدد الزمنية بصيغ نصية مثل `"5s"` أو `"10m"`، وأحجام الذاكرة بصيغ مثل `"512MB"`. يجب أن يدعم النظام قراءة وتحليل كل من النصوص المفهومة بشرياً والأرقام الخام دون حدوث أخطاء فك الترميز.

5. **الحفظ الذري للملفات (Atomic File Persistence)**:
   عند كتابة التكوين على القرص، يُمنع الكتابة المباشرة في نفس الملف. يتم إنشاء ملف مؤقت، ومزامنة ذاكرة التخزين المؤقت (`Sync()`)، ثم استبدال الملف الأصلي بعملية إعادة تسمية ذرية (`os.Rename`) لحمايته من التلف عند انقطاع التيار المفاجئ.

6. **التوثيق الذاتي التصريحي (Self-Documenting Declarative Schema)**:
   تعلن كل خاصية عن مفتاح البيئة الخاص بها، والقيمة الافتراضية، ووصفها التوضيحي باستخدام وسوم Go Struct Tags (`env:`, `default:`, `desc:`)، مما يتيح توليد ملفات `.env.example` آلياً من الكود المصدري مباشرة.

---

## المعمارية: نمط التصميم القائم على المحركات (Engine-Based Pattern)

تم تصميم نظام التكوين كنظام محرك تركيبي يتألف من محرك رئيسي منسق (`Engine`) وخمسة محركات فرعية متخصصة ومستقلة تؤدي كل منها وظيفة محددة بعناية:

```text
┌──────────────────────────────────────────────────────────────────────────┐
│                    Engine (المحرك المركزي والمنسق العام)                  │
│    atomic.Pointer[Configuration] ── قراءة متزامنة فائقة السرعة بدون أقفال │
│    NewEngine(opts ...Option)    ── تركيب المحرك عبر نمط الخيارات الوظيفية │
├──────────────────────────────────────────────────────────────────────────┤
│                                                                          │
│  ┌─────────────────────┐  ┌─────────────────────┐  ┌──────────────────┐ │
│  │  PipelineEngine     │  │  ValidationEngine   │  │  SecurityEngine  │ │
│  │  ────────────────── │  │  ────────────────── │  │  ──────────────  │ │
│  │  الطبقة 1: الافتراضيات│ │  فحص نطاق المنافذ │  │  حجب الأسرار    │ │
│  │  الطبقة 2: ملف JSON │  │  اتساق المُدد المهلة│  │  حجب عناوين URL  │ │
│  │  الطبقة 3: متغيرات  │  │  سلامة روابط الـ DB │  │  SecretString    │ │
│  │  الطبقة 4: معلمات CLI│ │  قوة مفاتيح JWT     │  │  معالجة %2A%2A%2A│ │
│  │  الطبقة 5: التجاوزات│  │  حدود بركة الاتصال  │  │                  │ │
│  └─────────────────────┘  └─────────────────────┘  └──────────────────┘ │
│                                                                          │
│  ┌─────────────────────┐  ┌─────────────────────┐                       │
│  │  PersistenceEngine  │  │  SchemaEngine        │                       │
│  │  ────────────────── │  │  ────────────────── │                       │
│  │  كتابة ذرية مؤقتة   │  │  فحص Struct Tags    │                       │
│  │  صلاحيات 0600 مشددة │  │  توليد .env.example │                       │
│  │  Temp + Sync + Rename│ │  استخراج FieldSpec   │                       │
│  └─────────────────────┘  └─────────────────────┘                       │
└──────────────────────────────────────────────────────────────────────────┘
```

### واجهة برمجة المحرك (Engine API Surface)

```go
//a إنشاء وتهيئة المحرك المركزي عبر الخيارات الوظيفية
engine := config.NewEngine(
    config.WithConfigFile("config/config.json"),
    config.WithCLIArgs(os.Args[1:]),
    config.WithRuntimeOverrides(func(cfg *config.Configuration) {
        cfg.Server.Port = "0" // تعيين منفذ عشوائي مؤقت لبيئة الاختبارات
    }),
)

cfg, err := engine.Load()      // تحميل الطبقات + التحقق الشامل + تخزين ذري
cfg = engine.Current()          // قراءة ذرية بدون أقفال (Lock-Free Hot Path)
cfg, err = engine.Reload()      // إعادة تحميل وتطبيق الإعدادات دون إعادة تشغيل الخدمة
safe := engine.Redact(cfg)      // حجب الأسرار والبيانات الحساسة لطباعة السجلات
err = engine.Save("config/config.json", cfg) // حفظ ذري آمن على القرص
template := engine.GenerateEnvTemplate()     // توليد محتوى .env.example تلقائياً

// دالة تسهيلية على مستوى الحزمة للاستخدام السريع
cfg, err := config.Load(config.WithConfigFile("config/config.json"))
```

### نمط الخيارات الوظيفية (Functional Options Pattern)

```go
type Option func(*Engine)

WithConfigFile(path string) Option       // تحديد مسار ملف التكوين JSON للطبقة الثانية
WithCLIArgs(args []string) Option        // تمرير وسائط سطر الأوامر للطبقة الرابعة
WithEnvLookup(fn) Option                 // حقن دالة قراءة المتغيرات البيئية (لأغراض الاختبار)
WithDisableEnv() Option                  // تعطيل طبقة المتغيرات البيئية كلياً
WithRuntimeOverrides(fn) Option          // تمرير دوال التجاوز البرمجي للطبقة الخامسة
WithValidationRule(rule) Option          // تسجيل قواعد فحص وتحقق مخصصة للمحرك
```

---

## خط أنابيب التحميل وتدرج الأسبقية (The Loading Pipeline)

```text
┌──────────────────────────────────────────────────────────────────────────┐
│                       1. القيم الافتراضية المضمنة                        │
│           قيم افتراضية آمنة مجمعة مباشرة داخل الكود الثنائي              │
└────────────────────────────────────┬─────────────────────────────────────┘
                                     ▼
┌──────────────────────────────────────────────────────────────────────────┐
│                      2. ملف التكوين الثابت (JSON)                        │
│      config/config.json ── دمج الحقول المحددة صراحة فوق الافتراضيات      │
└────────────────────────────────────┬─────────────────────────────────────┘
                                     ▼
┌──────────────────────────────────────────────────────────────────────────┐
│                      3. المتغيرات البيئية (Env Layer)                    │
│      المفاتيح القياسية (PORT) + الأسماء البديلة (PGPORT) + ملفات *_FILE  │
└────────────────────────────────────┬─────────────────────────────────────┘
                                     ▼
┌──────────────────────────────────────────────────────────────────────────┐
│                      4. معلمات سطر الأوامر (CLI Flags)                   │
│      الخيارات المحددة صراحة (--port 8080, --host 0.0.0.0)                 │
└────────────────────────────────────┬─────────────────────────────────────┘
                                     ▼
┌──────────────────────────────────────────────────────────────────────────┐
│                      5. التجاوزات البرمجية أثناء التشغيل                 │
│      دوال التعديل البرمجي (Test Hooks و Ephemeral Network Adjustments)   │
└────────────────────────────────────┬─────────────────────────────────────┘
                                     ▼
┌──────────────────────────────────────────────────────────────────────────┐
│                    محرك الفحص والتحقق الاستباقي الشامل                   │
│      التأكد من المنافذ (1-65535)، وجود الأسرار، واتساق المُدد الزمنية    │
└────────────────────────────────────┬─────────────────────────────────────┘
                                     ▼
                          إعدادات جاهزة ومحققة
                    (محفوظة داخل atomic.Pointer[Configuration])
```

### قواعد أسبقية التحميل (Resolution Precedence Rules)

- **القيم الافتراضية (Defaults)**: قيم عمل أساسية وآمنة مدمجة داخل الكود (مثل: `Port: "8080"`, `ReadTimeout: 5s`). يتم تعريفها في دالة `DefaultConfiguration()`.
- **ملف التكوين (JSON File)**: ملف تخزين ثابت على القرص (`config/config.json`). يتم فك التشفير فوق النسخة الافتراضية، وبالتالي الحقول الموجودة في الملف فقط هي التي يتم استبدالها بينما تظل الحقول غير المذكورة محتفظة بقيمها الافتراضية.
- **المتغيرات البيئية (Environment Variables)**: خيارات بيئة التشغيل والحاويات. تدعم المفاتيح الرسمية، والأسماء البديلة لمحركات قواعد البيانات (مثل دعم `PGPORT` كبديل لـ `DB_PORT`)، والملفات المحقونة كأسرار (`*_FILE`).
- **معلمات سطر الأوامر (CLI Flags)**: خيارات تمرر للمشغل عند الإقلاع (مثل `--port`، `--host`). تتمتع بأسبقية تتجاوز ملفات التكوين والبيئة، ويتم تطبيق المعلمات الممررة صراحة فقط باستخدام `flag.Visit`.
- **التجاوزات البرمجية (Runtime Overrides)**: تُمرر برمجياً عبر دالة `WithRuntimeOverrides(func(cfg *Configuration))`، وتُعد الأعلى أسبقية، ومفيدة جداً في اختبارات التكامل والتوجيه الديناميكي للخدمات.

---

## محرك التحقق الاستباقي الشامل (Fail-Fast Multi-Error Validation)

يمنع النظام إقلاع الخدمة إطلاقاً في حال وجود خلل في التكوين، ويتم الفحص في المرحلة الأولى (Phase 1) قبل إنشاء أي اتصالات شبكية أو فتح اتصالات بقواعد البيانات:

```go
type ValidationError struct {
    Field   string `json:"field"`
    Message string `json:"message"`
}

type ValidationReport struct {
    Errors []ValidationError `json:"errors"`
}

func (r *ValidationReport) Add(field, msg string) {
    r.Errors = append(r.Errors, ValidationError{Field: field, Message: msg})
}

func (r *ValidationReport) HasErrors() bool {
    return len(r.Errors) > 0
}

func (r *ValidationReport) Error() string {
    // تنسيق كافة الأخطاء في تقرير تشخيصي تركيبي متعدد الأسطر
    // مثال: "configuration validation failed (3 errors detected):\n  - server.port : ..."
}
```

### القواعد الفاحصة المضمنة (Built-in Invariants)

- **نطاق المنافذ**: التحقق من أن المنفذ يقع بين `1 <= Port <= 65535` لمنفذ الخادم ومنفذ قاعدة البيانات.
- **اتصال قاعدة البيانات**: التحقق من اختيار محرك معتمد (`postgres` أو `memory`)، وعدم ترك اسم المضيف، واسم المستخدم، واسم قاعدة البيانات فارغة عند اختيار `postgres` بدون رابط اتصال كامل.
- **صيغة رابط الاتصال (Database URL)**: إذا تم تزويد `DatabaseURL`، يجب أن يبدأ ببروتوكول صالح (`postgres://` أو `postgresql://`).
- **المُدد الزمنية (Timeouts)**: التأكد أن `ReadTimeout > 0`، و `WriteTimeout > 0`، و `ShutdownTimeout > 0`.
- **اتساق مهلة الإيقاف مع التصريف**: التحقق الإلزامي من أن `ShutdownTimeout > DrainDuration` لضمان إتاحة وقت كافٍ لتفريغ الطلبات قبل قطع الاتصال.
- **بركة الاتصالات (Connection Pool)**: التحقق من أن `MaxOpenConns > 0`، و `MaxIdleConns >= 0`، وأن `MaxIdleConns <= MaxOpenConns`.
- **الأسرار الإلزامية**: التحقق من أن طول مفتاح توقيع JWT لا يقل عن 32 حرفاً (`>= 32`).
- **صلاحية الرموز (Token TTL)**: التأكد من أن `TokenTTL > 0`.

---

## محرك الأمان وحجب الأسرار (Secret Security & Redaction)

### 1. نمط الحجب عبر `Redacted()`

يُمنع طباعة هياكل الإعدادات مباشرة في السجلات عبر `fmt.Sprintf("%+v", cfg)` أو عبر مكتبات التدوين المهيكل:

```go
func (c *Configuration) Redacted() *Configuration {
    engine := NewSecurityEngine()
    return engine.Redact(c)
}

// يقوم SecurityEngine.Redact باستنساخ عميق للبيانات مع استبدال:
// - Database.Password ← "***"
// - Database.DatabaseURL ← MaskURL (user:*** محفوظ)
// - Auth.JWTSecret ← "***"
```

### 2. النوع المخصص `SecretString`

للحقول النصية فائقة الحساسية، يوفر النظام نوعاً مخصصاً يقوم بحجب نفسه تلقائياً عند الطباعة أو التحويل إلى JSON:

```go
type SecretString string

func (s SecretString) String() string {
    if s == "" {
        return ""
    }
    return "***"
}

func (s SecretString) MarshalJSON() ([]byte, error) {
    if s == "" {
        return []byte(`""`), nil
    }
    return []byte(`"***"`), nil
}

func (s SecretString) Expose() string {
    return string(s) // الوصول الصريح للنص الأصلي عند الحاجة للعمليات الحقيقية فقط
}
```

### 3. تشفير وحجب روابط قواعد البيانات (URL Credential Masking)

```go
//a تقوم دالة MaskURL بتنقية وتأمين روابط الاتصال بقاعدة البيانات:
// المدخل: "postgres://user:secretpass@host:5432/db"
// المخرج: "postgres://user:***@host:5432/db"
// كما تقوم بمعالجة وحل مشكلة الترميز %2A%2A%2A واستبدالها بنجوم صريحة ***
func (s *SecurityEngine) MaskURL(rawURL string) string
```

### 4. أذونات وصلاحيات الملفات الصارمة

أي ملف إعدادات يحتوي على بيانات اعتماد يتم تخزينه على القرص يجب أن يلتزم بصلاحيات POSIX الصارمة:

```go
const ConfigFilePerm = os.FileMode(0o600) // قراءة وكتابة للمستخدم المالك فقط
const ConfigDirPerm  = os.FileMode(0o700) // فتح وتصفح المجلد للمستخدم المالك فقط
```

### 5. دعم نمط الأسرار من ملفات الحاويات (*_FILE Pattern)

في بيئات Docker وKubernetes، تُحقن الأسرار عادةً كملفات في وحدات التخزين المؤقتة (Secret Volumes). يدعم محرك الأنابيب البحث التلقائي عن اللاحقة `*_FILE`:

- `DB_PASSWORD_FILE` ← قراءة كلمة المرور من المسار المحدد وتعيينها في `Database.Password`.
- `DATABASE_URL_FILE` ← قراءة رابط الاتصال وتعيينه في `Database.DatabaseURL`.
- `JWT_SECRET_FILE` ← قراءة المفتاح وتعيينه في `Auth.JWTSecret`.

---

## الأنواع المخصصة المقاومة للأخطاء والتحليل المرن

### 1. تحليل المُدد الزمنية المرن (`Duration`)

يدعم التحليل من النصوص المقروءة بشرياً (`"5s"`، `"10m"`) ومن الأرقام المباشرة كأجزاء من الثانية:

```go
type Duration time.Duration

func (d *Duration) UnmarshalJSON(b []byte) error {
    var v interface{}
    if err := json.Unmarshal(b, &v); err != nil {
        return err
    }
    switch val := v.(type) {
    case float64:
        *d = Duration(time.Duration(val * float64(time.Second)))
        return nil
    case string:
        parsed, err := time.ParseDuration(val)
        if err != nil {
            return err
        }
        *d = Duration(parsed)
        return nil
    default:
        return errors.New("invalid duration format")
    }
}
```

### 2. تحليل أحجام الذاكرة والملفات (`ByteSize`)

يدعم تحويل وحدات السعة المفهومة بشرياً (`"512MB"`، `"1GB"`، `"64KB"`) إلى بايتات رقمية صحيحة:

```go
type ByteSize int64

const (
    Byte     ByteSize = 1
    Kilobyte          = 1024 * Byte
    Megabyte          = 1024 * Kilobyte
    Gigabyte          = 1024 * Megabyte
    Terabyte          = 1024 * Gigabyte
)

func ParseByteSize(s string) (ByteSize, error) {
    // دعم الوحدات: B, KB/KIB, MB/MIB, GB/GIB, TB/TIB
}
```

---

## محرك الحفظ الذري للملفات (Atomic Persistence Engine)

الكتابة المباشرة لملفات الإعدادات عبر `os.WriteFile` قد تؤدي إلى تدمير أو تفريغ الملف إذا انقطع النظام أثناء الكتابة. يتبع `PersistenceEngine` الخطوات المعيارية التالية:

1. إنشاء ملف مؤقت في نفس المجلد الأصلي باسم: `.config.json.tmp.*`.
2. ضبط أذونات الملف المشددة فوراً إلى `0600`.
3. كتابة التكوين المنسق بصيغة JSON.
4. استدعاء `tmpFile.Sync()` لإجبار نظام التشغيل على إفراغ البيانات من الذاكرة إلى وسائط التخزين الفعلية.
5. إغلاق الملف المؤقت بشكل سليم.
6. استبدال الملف الأصلي عبر إعادة التسمية الذرية في نظام التشغيل (`os.Rename`).
7. تنظيف وإزالة الملف المؤقت تلقائياً في حالة حدوث أي تعثر قبل إتمام النقل.

---

## محرك المخطط والتوثيق الذاتي (Schema Engine)

يقوم `SchemaEngine` بفحص وسوم الهياكل (Struct Tags) أثناء التشغيل باستخدام الانعكاس (Reflection):

```go
type FieldSpec struct {
    EnvKey       string `json:"env_key"`
    DefaultValue string `json:"default_value"`
    Description  string `json:"description"`
}

// الاستخدام البرمجي:
specs := engine.ExtractSpecs(config.DefaultConfiguration())
template := engine.GenerateEnvTemplate()
engine.WriteEnvExample(".env.example")
```

وسوم الهياكل المستخدمة في التوثيق:

```go
Host string `json:"host" env:"HOST" default:"0.0.0.0" desc:"HTTP server binding host interface"`
Port string `json:"port" env:"PORT" default:"8080" desc:"HTTP server TCP listen port"`
```

---

## إدارة الحالة والتحديث الحي فائق السرعة (Atomic Hot-Reload)

يعتمد المحرك الرئيسي على `sync/atomic.Pointer[Configuration]` لضمان قراءة التكوين بسرعة هائلة في مسار الطلبات الحرج (Hot Path) بدون التسبب في أي تنافس على الأقفال:

```go
type Engine struct {
    // ... المحركات الفرعية ...
    current atomic.Pointer[Configuration]
}

// Current() ── قراءة ذرية بدون أقفال (Lock-free)، آمنة تماماً في معالجة طلبات HTTP
// Reload()  ── إعادة استقراء كافة الطبقات، وإعادة الفحص، والتبديل الذري للمؤشر
// Load()    ── التحميل الأولي والتحقق والتخزين الذري
```

---

## هيكل التكوين المعتمد في المشروع (Configuration Structure)

```go
type Configuration struct {
    Server   ServerSettings   `json:"server"`
    Database DatabaseSettings `json:"database"`
    Auth     AuthSettings     `json:"auth"`
}

type ServerSettings struct {
    Host            string   `json:"host" env:"HOST" default:"0.0.0.0"`
    Port            string   `json:"port" env:"PORT" default:"8080"`
    ReadTimeout     Duration `json:"read_timeout" env:"READ_TIMEOUT" default:"5s"`
    WriteTimeout    Duration `json:"write_timeout" env:"WRITE_TIMEOUT" default:"10s"`
    ShutdownTimeout Duration `json:"shutdown_timeout" env:"SHUTDOWN_TIMEOUT" default:"15s"`
    DrainDuration   Duration `json:"drain_duration" env:"DRAIN_DURATION" default:"5s"`
    MaxHeaderBytes  ByteSize `json:"max_header_bytes" env:"MAX_HEADER_BYTES" default:"1MB"`
    MaxBodySize     ByteSize `json:"max_body_size" env:"MAX_BODY_SIZE" default:"10MB"`
}

type DatabaseSettings struct {
    Driver       string `json:"driver" env:"DB_DRIVER" default:"postgres"`
    Host         string `json:"host" env:"DB_HOST" default:"localhost"`
    Port         string `json:"port" env:"DB_PORT" default:"5432"`
    Name         string `json:"name" env:"DB_NAME" default:"train_db"`
    User         string `json:"user" env:"DB_USER" default:"train_user"`
    Password     string `json:"password" env:"DB_PASSWORD" default:""`
    DatabaseURL  string `json:"database_url" env:"DATABASE_URL" default:""`
    MaxOpenConns int    `json:"max_open_conns" env:"DB_MAX_OPEN_CONNS" default:"25"`
    MaxIdleConns int    `json:"max_idle_conns" env:"DB_MAX_IDLE_CONNS" default:"25"`
}

type AuthSettings struct {
    JWTSecret string   `json:"jwt_secret" env:"JWT_SECRET" default:"..."`
    TokenTTL  Duration `json:"token_ttl" env:"JWT_TOKEN_TTL" default:"24h"`
}
```

---

## الاعتماديات والتكاملات بين المهارات (Cross-Skill Dependencies & Integrations)

تشكّل مهارة التكوين مصدر القيم الأول لكافة مهارات المنصة: تُغذّي قيمها الزمنية وضوابط قاعدة البيانات ونطاقات المنافذ بقية المهارات، بينما تعتمد بدورها على مهارة الأخطاء لتجميع تقرير التحقق الموحد:

```text
go-errors   ─► go-service-configuration (تجميع أخطاء التحقق في تقرير موحد)
go-service-configuration ─► go-server-lifecycle (مهل زمنية، عناوين، مدة التصريف، فحص Fail-Fast)
go-service-configuration ─► go-postgres (إعدادات قاعدة البيانات وحدود بركة الاتصالات مع الحجب)
go-service-configuration ─► go-logger-slog  (تسجيل آمن للتكوين عبر Redacted)
go-service-configuration ─► go-context (ميزانيات المهل الزمنية المشتقة من القيم)
```

### المهارات التي تعتمد عليها هذه المهارة (Downstream Dependencies)

| المهارة | نوع الاعتماد | نقاط التكامل الرئيسية |
| :--- | :--- | :--- |
| `go-errors` | نمط تجميع أخطاء التحقق في تقرير تشخيصي موحد (بأسلوب `errors.Join` متعدد الأخطاء) | `ValidationReport` يجمع كل مشاكل الإعدادات بدل إيقاف التشغيل عند أول خطأ |

### التكامل مع المهارات الأخرى (Upstream Integrations)

| المهارة | نوع التكامل | نقاط التكامل الرئيسية |
| :--- | :--- | :--- |
| `go-server-lifecycle` | كل قيم المراحل الثماني تُقرأ من التكوين المحمّل مسبقاً | `ServerSettings.Host/Port`, `ReadTimeout`, `WriteTimeout`, `ShutdownTimeout`, `DrainDuration`, `MaxHeaderBytes` |
| `go-postgres` | إعدادات قاعدة البيانات وحدود المجمع تُمرر لتوليف `pgxpool` | `DatabaseSettings.*`, `MaxOpenConns`, `MaxIdleConns`, ونمط الأسرار `*_FILE` |
| `go-logger-slog` | طباعة التكوين بأمان دون كشف الأسرار | `engine.Redact(cfg)` / `SecretString.MarshalJSON` |
| `go-context` | قيم المهل تزوّد حسابات ميزانية الوقت في طبقة الاستخدام | `ShutdownTimeout`, `ReadTimeout` كمرجع لـ `RequireMinimumBudget` |

---

## الأنماط المضادة التي يجب تجنبها (Anti-Patterns)

| النمط المضاد (Anti-Pattern) | لماذا يفشل في بيئات الإنتاج؟ | الحل المعماري السليم |
| :--- | :--- | :--- |
| الاعتماد المنفرد على `os.Getenv` | غير مرن ولا يتيح قيماً افتراضية آمنة أو ملفات JSON | خط أنابيب متعدد الطبقات: Defaults < File < Env < CLI |
| إيقاف التشغيل عند أول خطأ فقط | يجبر مشغل النظام على تكرار المحاولة N مرات لكشف N أخطاء | جامع أخطاء استباقي يوضح كافة المشاكل في تقرير موحد |
| طباعة الأسرار بنصوص صريحة في السجلات | تسريب كلمات المرور والتوكنات لأدوات تجميع السجلات | تطبيق `Redacted()` وحجب الروابط وكلمات المرور (`***`) |
| الكتابة المباشرة عبر `os.WriteFile` | انقطاع التيار أو انهيار النظام يؤدي لملف فارغ أو تالف | الكتابة الذرية لملف مؤقت ثم المزامنة `Sync()` والاستبدال الذري |
| الأذونات المتساهلة للملفات | تمكين المستخدمين الآخرين على نفس السيرفر من قراءة الأسرار | فرض الصلاحيات المشددة `0600` (للمالك فقط) |
| الربط الصارم بإطارات عمل خارجية | تبعية كود التكوين لمكتبات طرف ثالث غير معيارية | الاعتماد على هياكل Go القياسية مع الوسوم المخصصة |
| الحالة العامة القابلة للتعديل غير الآمن | تسابق البيانات (Race Conditions) بين خيوط المعالجة | استخدام `atomic.Pointer[Configuration]` للوصول الخالي من الأقفال |

---

## قائمة التحقق والاختبار (Verification Checklist)

```text
[ ] ربط المحركات الفرعية بنجاح: Pipeline, Validation, Security, Persistence, Schema
[ ] دمج الإعدادات بترتيب الأسبقية الصحيح: الافتراضيات < ملف JSON < البيئة < CLI
[ ] الفحص الاستباقي الشامل يكتشف الأخطاء ويعيد تقريراً تشخيصياً موحداً
[ ] إمكانية تسجيل قواعد تحقق مخصصة عبر WithValidationRule()
[ ] حجب كلمات المرور وروابط الاتصال والمفاتيح برمز *** في السجلات
[ ] كتابة ملفات التكوين بصلاحيات 0600 باستخدام التبديل الذري للملف المؤقت
[ ] دعم المُدد الزمنية كنصوص ("5s", "10m") وكأرقام بالثواني
[ ] دعم أحجام البيانات كنصوص ("1MB", "512KB") وكرقم بايتات خام
[ ] دعم الأسماء البديلة للبيئة (مثل PGHOST و PGPORT لقاعدة البيانات)
[ ] دعم قراءة الأسرار من ملفات الحاويات (نمط *_FILE لبيئات Kubernetes و Docker)
[ ] استخدام atomic.Pointer للقراءة بدون أقفال والتحديث الحي الآمن أثناء التشغيل
[ ] توليد نموذج .env.example مباشرة من وسوم الهياكل البرمجية
[ ] اجتياز كافة اختبارات الوحدة مع فحص التزامن: go test -v -race ./...
```

---

## مراجع التوثيق التفصيلية (Detailed Documentation References)

للاطلاع على أدق التفاصيل الهندسية، والسلوكيات، والخصائص لكل مكون ومحرك فرعي، راجع الملفات التوثيقية المرفقة بالمهارة:

- [دليل المكونات والمحركات المرجعي الشامل](docs/components_reference_guide.md)
- [معمارية التكوين متعدد الطبقات](docs/01_layered_configuration.md)
- [دليل التحقق الاستباقي وتجميع الأخطاء](docs/02_fail_fast_validation.md)
- [دليل أمان الأسرار وحجب البيانات الحساسة](docs/03_secret_security_and_redaction.md)
- [دليل الأنواع المخصصة والتحليل المرن](docs/04_custom_types_and_parsing.md)
- [دليل التوثيق الذاتي واستخراج المواصفات](docs/05_spec_and_self_documentation.md)
- [مصفوفة هرمية الأسبقية وقواعد التدرج](references/priority_hierarchy_matrix.md)
- [قائمة المعايير القياسية لمهندسي البرمجيات](references/config_standards_checklist.md)
