# 01. البنية المعمارية وخيارات الـ Handlers (Architecture & Handlers)

> **المصادر المرجعية:** Go Official Blog: *Structured Logging with slog* (Jonathan Amsterdam), Go Design Proposal #56345, pkg.go.dev/log/slog.

---

## 1. مشكلة فوضى السجلات وحلها المعماري

في الأنظمة البرمجية الموزعة وخدمات Go الكبيرة، تعاني الفرق من ظاهرة "فوضى السجلات" (Log Anarchy):

- تعتمد كل مكتبة خارجية مستوردة على حزمة تسجيل مختلفة (`logrus`, `zap`, `zerolog`, `go-kit/log`, `log`).
- ينتج عن ذلك تضارب في وجهات الإخراج، وتنافر في صيغة التواريخ والبيانات المهيكلة، وصعوبة بالغة في توحيد المراقبة والتجميع (Log Ingestion & Aggregation).

جاءت حزمة `log/slog` في **Go 1.21** لتقديم حل جذري يعتمد على **الفصل التام بين الواجهة والتنفيذ (Frontend/Backend Decoupling)**.

---

## 2. معمارية Frontend مقابل Backend

```text
┌─────────────────────────────────────────────────────────┐
│                    Application Code                     │
│         logger.InfoContext(ctx, "order created", ...)   │
└────────────────────────────┬────────────────────────────┘
                             │
                    ┌────────▼────────┐
                    │   slog.Logger   │  ◄── Frontend (API)
                    │    (الواجهة)     │
                    └────────┬────────┘
                             │
                    ┌────────▼────────┐
                    │  slog.Handler   │  ◄── Backend (Interface)
                    │    (التنفيذ)    │
                    └────────┬────────┘
                             │
       ┌─────────────────────┼─────────────────────┐
       ▼                     ▼                     ▼
 slog.TextHandler      slog.JSONHandler       Custom Handler
 (التطوير المحلي)     (الإنتاج / الحاويات)   (Redaction / Zap / etc)
```

### دور الـ Frontend (`*slog.Logger`)

- يمثل نقطة الاستدعاء المباشرة من كود التطبيق.
- يوفر دوال التسجيل المريحة (`DebugContext`, `InfoContext`, `WarnContext`, `ErrorContext`, `LogAttrs`).
- ينشئ حدث السجل في صورة بنية تدعى `slog.Record`.

### دور الـ Backend (`slog.Handler`)

- واجهة Go تتلقى الـ `slog.Record` وتقرر كيفية تنسيقه وإلى أين يتم توجيهه.
- تتحكم في مستويات التسجيل ومطابقة الشروط قبل الاستهلاك الفعلي للموارد.
- قابلة للاستبدال كلياً أو التغليف (Middleware Chaining) دون الحاجة للمس كود التطبيق.

---

## 3. المكونات الأساسية في الحزمة

| المكون | النوع في Go | الوصف والمسؤولية |
| :--- | :--- | :--- |
| **Logger** | `struct *slog.Logger` | الواجهة الأمامية للمطورين لاستدعاء دوال التسجيل |
| **Record** | `struct slog.Record` | كائن يحتوي على بيانات الحدث الواحد (الوقت، المستوى، الرسالة، مؤشر البرنامج `PC`، والسمات) |
| **Handler** | `interface slog.Handler` | الواجهة الخلفية المسؤولة عن تنسيق وإرسال السجلات |
| **Attr** | `struct slog.Attr` | زوج مفتاح-قيمة مهيكل عالي الكفاءة (`Key string`, `Value slog.Value`) |
| **Level** | `type slog.Level` | مستوى الحدث (`LevelDebug = -4`, `LevelInfo = 0`, `LevelWarn = 4`, `LevelError = 8`) |

---

## 4. مقارنة المعالجات المدمجة (Built-in Handlers)

### 1. `slog.JSONHandler` (معيار بيئات الإنتاج)

- **الصيغة:** كائن JSON أحادي السطر لكل سجل (NDJSON - Newline Delimited JSON).
- **الاستخدام الموصى به:** بيئات الإنتاج، حاويات Docker، مجموعات Kubernetes، ومنصات تجميع السجلات مثل Datadog, Elasticsearch, Grafana Loki, Google Cloud Logging.
- **المزايا:** فك وفلترة آلية فورية وسريعة بدون Regex، متوافق مع كافة أدوات المراقبة.

### 2. `slog.TextHandler` (معيار التطوير المحلي)

- **الصيغة:** مفتاح=قيمة مقروءة بشرياً (`time=... level=INFO msg="..." key=value`).
- **الاستخدام الموصى به:** طرفية التطوير المحلي وأجهزة المهندسين ومخرجات الاختبارات السريعة.
- **المزايا:** قراءة بصرية سهلة دون ضوضاء أقواس الـ JSON.

---

## 5. خيارات المعالج (`slog.HandlerOptions`)

```go
opts := &slog.HandlerOptions{
    // 1. تحديد أدنى مستوى للتسجيل
    Level: slog.LevelInfo,

    // 2. تضمين اسم ملف المصدر ورقم السطر (مفيد للتشخيص في التطوير)
    AddSource: false, // في الإنتاج يُفضل تعطيله لتقليل استهلاك وحدة المعالجة المركزية إلا للضرورة

    // 3. تعديل أو استبدال السمات قبل كتابتها
    ReplaceAttr: func(groups []string, a slog.Attr) slog.Attr {
        // توحيد مفتاح الوقت إلى timestamp
        if a.Key == slog.TimeKey {
            return slog.Attr{Key: "timestamp", Value: a.Value}
        }
        return a
    },
}
```
