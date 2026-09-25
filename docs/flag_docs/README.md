# 🧭 الدليل الموسوعي الشامل لحزمة `flag` في لغة Go

مرحباً بك في التوثيق الهندسي الشامل والتحليلي لحزمة `flag` من مكتبة لغة Go القياسية (`/usr/local/go/src/flag`).

تم إعداد هذا المرجع الموسوعي ليغطي كافة جوانب الحزمة: من الجذور التاريخية وفلسفة التصميم، إلى التشريح الدقيق للواجهات والأنواع الملموسة، وآلة الحالة الداخلية للتحليل، مع توثيق لكل دالة، ونماذج عملية لبناء أدوات سطر أوامر احترافية بنظام الأوامر الفرعية (Subcommands).

---

## 🗺️ خريطة ملفات التوثيق والهيكلية

تم تقسيم هذا التوثيق الموسوعي إلى سبعة ملفات متخصصة ومترابطة داخل مجلد `docs/flag_docs/`:

```
docs/flag_docs/
├── README.md                                          # الفهرس العام وخريطة التوثيق الموسوعي (الملف الحالي)
├── 01_philosophy_and_architecture.md                  # الفلسفة المعمارية ومبررات الوجود والمشاكل الجوهرية الست
├── 02_interfaces_and_type_system.md                   # نظام الأنواع والواجهات (Value, Getter, boolFlag) والأنواع الـ 11
├── 03_flagset_and_core_structures.md                  # الهياكل الجوهرية (FlagSet, Flag, ErrorHandling) والحالة العامة
├── 04_parsing_pipeline_and_state_machine.md           # خوارزمية التحليل وقواعد الصياغة وآلة الحالة وحماية undef
├── 05_api_reference.md                                # المرجع الشامل لكافة الدوال والتوابع والتواقيع البرمجية
├── 06_subcommands_and_advanced_patterns.md            # الأنماط المتقدمة وهندسة الأوامر الفرعية والأنواع المخصصة والاختبار
└── 07_best_practices_limitations_and_comparisons.md   # أفضل الممارسات والمحاذير الأمنية والمقارنة مع Cobra و pflag
```

---

## 📚 ملخص محتويات المجلد وروابط الوثائق

### 1. [01. الفلسفة المعمارية ومبررات وجود حزمة `flag`](file:///home/osm/StudioProjects/train_struct/docs/flag_docs/01_philosophy_and_architecture.md)
- **السياق التاريخي:** الانتقال من إرث Unix و Plan 9 إلى Go، ومبررات رفض تعقيدات POSIX و GNU getopt.
- **المشاكل الجوهرية الست التي تحلها الحزمة:**
  1. معضلة تحويل النصوص وإدارة الأنماط المباشرة (Direct Type-Safe Binding).
  2. فخ توسيع الأسماء في قذائف الأنظمة (The Shell Globbing Asterisk Trap).
  3. استقلالية الأوامر وتعدد السياقات عبر `FlagSet`.
  4. التوثيق الذاتي المحاذى جدولياً وتوليد رسائل المساعدة التلقائية.
  5. قابلية التوسيع المفتوحة عبر واجهة `flag.Value`.
  6. كشف أخطاء ترتيب التهيئة واستدعاءات `init()` (Issue 57411).

---

### 2. [02. نظام الأنواع والواجهات (Interfaces & Type System)](file:///home/osm/StudioProjects/train_struct/docs/flag_docs/02_interfaces_and_type_system.md)
- **الواجهات الأساسية:**
  - `flag.Value`: العقد البرمجي الأساسي (`String()`, `Set()`) وقاعدة الـ Nil Receiver.
  - `flag.Getter`: سبب الوجود وتعهد التوافقية العكسية الصارم مع Go 1.
  - `boolFlag`: الواجهة الخفية التي تمنح الرايات المنطقية القدرة على العمل بدون معامِلات.
- **تشريح الأنواع الملموسة الـ 11:**
  `boolValue`, `intValue`, `int64Value`, `uintValue`, `uint64Value`, `stringValue`, `float64Value`, `durationValue`, `textValue` (مع واجهات encoding القياسية), `funcValue`, و `boolFuncValue`.
- دالة `numError` وتوحيد أخطاء التحويل وحدود السعة (`errParse` و `errRange`).

---

### 3. [03. الهياكل الجوهرية وحالة النظام (`FlagSet` & Core Structures)](file:///home/osm/StudioProjects/train_struct/docs/flag_docs/03_flagset_and_core_structures.md)
- **الهيكل `Flag`:** تمثيل الراية الفردية وتثبيت القيمة الافتراضية نصياً.
- **الهيكل `FlagSet`:** تشريح كافة الحقول الداخلية (`formal`, `actual`, `args`, `output`, `undef`).
- **الفارق المعماري بين `formal` (الرسمي) و `actual` (الواقعي).**
- **استراتيجيات الأخطاء الثلاث:** `ContinueOnError`, `ExitOnError`, `PanicOnError` ومعالجة `ErrHelp`.
- **الحالة العامة:** كائن `CommandLine` والدالة الحارسة `commandLineUsage`، وترتيب الرايات معجمياً عبر `sortFlags`.

---

### 4. [04. خوارزمية التحليل وآلة الحالة (Parsing Pipeline & State Machine)](file:///home/osm/StudioProjects/train_struct/docs/flag_docs/04_parsing_pipeline_and_state_machine.md)
- **قواعد الصياغة الخمس:** تطابق `-` و `--`، واستخدام الفاصل القاطع `--`، وسلوك الشرطة الفردية `-`.
- **التشريح السطري لدالة `parseOne()`:** دورة حياة قراءة الراية واستخراج علامة `=` ومعالجة الرايات المنطقية.
- **حلقة `Parse()` ومسار اتخاذ القرار.**
- **ميكانيكية الحماية الاستباقية ضد أخطاء التهيئة (Issue 57411):** التتبع التلقائي عبر `runtime.Caller(2)` في حقل `undef`.

---

### 5. [05. الدليل المرجعي الشامل لكافة الدوال والتوابع (API Reference)](file:///home/osm/StudioProjects/train_struct/docs/flag_docs/05_api_reference.md)
- مرجع مصنف ودقيق لكل دالة وتابع في الحزمة:
  - دوال الإنشاء والتهيئة وتغيير المخرجات.
  - دوال التحليل والاستعلام عن الحالة والمعاملات الموضعية (`Args`, `Arg`, `NArg`, `NFlag`).
  - دوال التسجيل للأنواع الأولية (مقارنة شاملة بين نمط المؤشر ونمط `Var`).
  - دوال التسجيل المتقدمة (`TextVar`, `Func`, `BoolFunc`, `Var`).
  - دوال المسح والتفتيش والتعديل البرمجي (`Lookup`, `Visit`, `VisitAll`, `Set`).
  - دوال المساعدة والتنصيص المائل (`PrintDefaults`, `UnquoteUsage`, `Usage`).

---

### 6. [06. الأنماط المتقدمة وهندسة الأوامر الفرعية (Subcommands & Advanced Patterns)](file:///home/osm/StudioProjects/train_struct/docs/flag_docs/06_subcommands_and_advanced_patterns.md)
- **بناء أدوات CLI بهرمية أوامر فرعية كاملة (مثل `git` أو `docker`) عبر `FlagSet` معزول.**
- **بناء أنواع مخصصة متقدمة عبر `flag.Value`:**
  - نمط الرايات المتكررة (Slice Flags).
  - نمط رايات الخرائط والقيم الزوجية (Key-Value / Map Flags).
  - نمط الخيارات المحصورة مسبقاً (Validated Enums).
- **التكامل مع `net/netip` وحزمة `encoding` عبر `TextVar`.**
- **استراتيجيات الاختبار المعياري (Unit Testing):** عزل المخرجات عبر `bytes.Buffer` وتجنب استدعاء `os.Exit(2)`.

---

### 7. [07. أفضل الممارسات، المحاذير الأمنية، ومقارنة البدائل (Best Practices & Comparisons)](file:///home/osm/StudioProjects/train_struct/docs/flag_docs/07_best_practices_limitations_and_comparisons.md)
- **هرمية التهيئة للتطبيقات الحديثة (12-Factor App):** الترتيب الصحيح للأسبقية واكتشاف الرايات المدخلة فعلياً عبر `Visit`.
- **تجنب تلويث النطاق العام للرايات في المكتبات المشتركة.**
- **المحاذير الأمنية الشديدة:** مخاطر تمرير كلمات المرور والرموز السرية كرايات في سطر الأوامر وبدائلها الآمنة.
- **القيود التصميمية لحزمة `flag` المعيارية.**
- **جدول المقارنة الموسع مع البدائل:** `flag` مقابل `pflag` و `cobra` و `urfave/cli` و `kong`.
- **مصفوفة اتخاذ القرار الهندسي:** متى تكتفي بالحزمة القياسية ومتى تنتقل لمكتبة خارجية.

---

## 🚀 نظرة سريعة: كيفية استخدام الحزمة في دقيقتين

### النمط الأول: الحصول على مؤشر مباشر (Pointer Pattern)
```go
package main

import (
	"flag"
	"fmt"
)

func main() {
	// تعريف الرايات مع قيم افتراضية ورسائل مساعدة
	wordPtr := flag.String("word", "hello", "a string word")
	numPtr := flag.Int("num", 42, "an integer number")
	boolPtr := flag.Bool("fork", false, "a boolean fork option")

	// إجراء عملية التحليل من os.Args[1:]
	flag.Parse()

	// الوصول للقيم عبر فك المؤشرات
	fmt.Println("word:", *wordPtr)
	fmt.Println("num:", *numPtr)
	fmt.Println("fork:", *boolPtr)
	fmt.Println("المعاملات المتبقية:", flag.Args())
}
```

### النمط الثاني: الربط بمتغيرات موجودة مسبقاً (Var Pattern)
```go
package main

import (
	"flag"
	"fmt"
)

func main() {
	var port int
	var env string

	flag.IntVar(&port, "port", 8080, "web server port")
	flag.StringVar(&env, "env", "development", "operating environment")

	flag.Parse()

	fmt.Printf("الخادم يعمل على المنفذ %d في بيئة %s\n", port, env)
}
```
