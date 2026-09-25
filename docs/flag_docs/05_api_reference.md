# 05. الدليل المرجعي الشامل لكافة الدوال والتوابع (API Reference)

يقدم هذا الدليل مرجعاً توثيقياً دقيقاً وشاملاً لكل دالة، تابع، نوع، ومتغير عام في حزمة `flag`، مصحوبة بالتواقيع البرمجية (Signatures)، التحليل السلوكي، الشروط المسبقة، والأمثلة العملية.

---

## 📑 فهرس الدوال والتوابع المصنفة

1. **الإنشاء والتهيئة وتغيير الحالة:** `NewFlagSet`, `Init`, `SetOutput`, `Output`, `Name`, `ErrorHandling`
2. **التحليل وحالة المعالجة:** `Parse`, `Parsed`
3. **الاستعلام عن المعاملات الموضعية:** `NFlag`, `NArg`, `Arg`, `Args`
4. **تعريف الرايات الأولية (النمط المزدوج):** `Bool/BoolVar`, `Int/IntVar`, `Int64/Int64Var`, `Uint/UintVar`, `Uint64/Uint64Var`, `Float64/Float64Var`, `String/StringVar`, `Duration/DurationVar`
5. **تعريف الرايات المتقدمة والدوال المخصصة:** `TextVar`, `Func`, `BoolFunc`, `Var`
6. **الاستكشاف والمسح والبحث:** `Lookup`, `Visit`, `VisitAll`
7. **التعديل البرمجي للقيم:** `Set`
8. **رسائل المساعدة والتوثيق التلقائي:** `PrintDefaults`, `UnquoteUsage`, `Usage`

---

## 1. الإنشاء والتهيئة (Initialization & Configuration)

### `NewFlagSet`
```go
func NewFlagSet(name string, errorHandling ErrorHandling) *FlagSet
```
- **الوصف:** ينشئ ويعيد مؤشراً جديداً لمجموعة رايات فارغة ومستقلة تماماً (`*FlagSet`).
- **المعاملات:**
  - `name`: اسم المجموعة (يظهر في ترويسات رسائل الخطأ والمساعدة: `Usage of <name>:`).
  - `errorHandling`: سياسة التعامل مع الأخطاء (`ContinueOnError`, `ExitOnError`, `PanicOnError`).
- **السلوك الداخلي:** يضبط حقل `Usage` تلقائياً على الدالة الافتراضية `f.defaultUsage`.

### `(f *FlagSet) Init`
```go
func (f *FlagSet) Init(name string, errorHandling ErrorHandling)
```
- **الوصف:** يقوم بإعادة تهيئة كائن `FlagSet` موجود مسبقاً باسم وسياسة أخطاء جديدة.
- **ملاحظة:** كائن `FlagSet{}` الصغير بدون تهيئة تكون سياسته الافتراضية `ContinueOnError` واسمه فارغ.

### `(f *FlagSet) SetOutput`
```go
func (f *FlagSet) SetOutput(output io.Writer)
```
- **الوصف:** يحدد الوجهة التي ستُطبع إليها رسائل المساعدة وأخطاء الصياغة.
- **القيمة الافتراضية:** إذا كانت القيمة `nil`، تعود الدالة تلقائياً إلى `os.Stderr`.
- **الاستخدام الشائع:** توجيه المخرجات إلى `bytes.Buffer` أثناء اختبارات الوحدات (Unit Testing)، أو إلى `io.Discard` لكتم الرسائل.

### `(f *FlagSet) Output`
```go
func (f *FlagSet) Output() io.Writer
```
- **الوصف:** يعيد مجرى الإخراج الحالي. إذا لم يُضبط المجرى أو كان `nil`، يُعاد `os.Stderr`.

### `(f *FlagSet) Name`
```go
func (f *FlagSet) Name() string
```
- **الوصف:** يعيد الاسم النصي المعين لمجموعة الرايات.

### `(f *FlagSet) ErrorHandling`
```go
func (f *FlagSet) ErrorHandling() ErrorHandling
```
- **الوصف:** يعيد سياسة معالجة الأخطاء الحالية لمجموعة الرايات.

---

## 2. التحليل وحالة المعالجة (Parsing Execution)

### `(f *FlagSet) Parse`
```go
func (f *FlagSet) Parse(arguments []string) error
```
- **الوصف:** يقوم بتحليل قائمة وسائط سطر الأوامر الممررة في `arguments` (التي لا ينبغي أن تتضمن اسم البرنامج نفسه).
- **الشروط:** يجب استدعاؤها بعد الانتهاء من تعريف كافة الرايات وقبل قراءة قيم المتغيرات.
- **المخرجات:** يُعيد `nil` في حال النجاح، أو خطأ التحليل (أو `ErrHelp`) إذا كانت السياسة `ContinueOnError`.

### `Parse` (العامة)
```go
func Parse()
```
- **الوصف:** الدالة العامة المناظرة التي تقوم بتحليل وسائط البرنامج الفعلي المستلمة من `os.Args[1:]` عبر الكائن العام `CommandLine`.
- **السلوك عند الخطأ:** تخرج من البرنامج فوراً بـ `os.Exit(2)` أو `os.Exit(0)` لأن سياسة `CommandLine` هي `ExitOnError`.

### `(f *FlagSet) Parsed` و `Parsed`
```go
func (f *FlagSet) Parsed() bool
func Parsed() bool
```
- **الوصف:** تُعيد `true` إذا تم استدعاء دالة `Parse` على المجموعة مسبقاً، و `false` خلاف ذلك.

---

## 3. المعاملات الموضعية والعدادات (Positional Args & Counts)

### `NFlag` و `(f *FlagSet) NFlag`
```go
func NFlag() int
func (f *FlagSet) NFlag() int
```
- **الوصف:** يُعيد عدد الرايات التي تم **تمريرها وضبطها فعلياً** في هذا التنفيذ (أي حجم خريطة `actual`). لا تشمل الرايات التي بقيت على قيمها الافتراضية دون إدخال.

### `NArg` و `(f *FlagSet) NArg`
```go
func NArg() int
func (f *FlagSet) NArg() int
```
- **الوصف:** يُعيد عدد المعاملات الموضعية المتبقية بعد اكتمال معالجة الرايات (`len(f.args)`).

### `Arg` و `(f *FlagSet) Arg`
```go
func Arg(i int) string
func (f *FlagSet) Arg(i int) string
```
- **الوصف:** يُعيد المعامل الموضعي رقم `i` (يبدأ الفهرس من `0`). إذا كان الفهرس خارج النطاق (`i < 0` أو `i >= NArg()`)، تُعيد الدالة نصاً فارغاً `""` بأمان تام دون panic.

### `Args` و `(f *FlagSet) Args`
```go
func Args() []string
func (f *FlagSet) Args() []string
```
- **الوصف:** يُعيد شريحة `[]string` تحوي كافة المعاملات الموضعية المتبقية بترتيبها.

---

## 4. دوال تعريف الرايات الأولية (Primitive Flags)

تتبع جميع الرايات الأولية النمط المعماري المزدوج:
1. **دالة تُعيد مؤشراً (`*T`):** تقوم بحجز الذاكرة داخلياً وإرجاع عنوان المتغير.
2. **دالة الربط بالمتغير (`TVar`):** تستقبل مؤشراً خارجياً أعده المبرمج وتخزن القيمة فيه مباشرة.

### جدول الدوال الأولية وتواقيعها:

| النوع المستهدف | دالة إرجاع المؤشر | دالة الربط بالمتغير القائم |
| :--- | :--- | :--- |
| **`bool`** | `Bool(name, val, usage) *bool` | `BoolVar(p, name, val, usage)` |
| **`int`** | `Int(name, val, usage) *int` | `IntVar(p, name, val, usage)` |
| **`int64`** | `Int64(name, val, usage) *int64` | `Int64Var(p, name, val, usage)` |
| **`uint`** | `Uint(name, val, usage) *uint` | `UintVar(p, name, val, usage)` |
| **`uint64`** | `Uint64(name, val, usage) *uint64`| `Uint64Var(p, name, val, usage)` |
| **`float64`** | `Float64(name, val, usage) *float64`| `Float64Var(p, name, val, usage)` |
| **`string`** | `String(name, val, usage) *string` | `StringVar(p, name, val, usage)` |
| **`time.Duration`**| `Duration(name, val, usage) *time.Duration` | `DurationVar(p, name, val, usage)` |

*(ملاحظة: تتوفر كافة هذه الدوال كدوال عامة على مستوى الحزمة، وكتوابع مطابقة على هيكل `*FlagSet`).*

---

## 5. دوال التعريف المتقدمة (Advanced & Dynamic Registrations)

### `TextVar` و `(f *FlagSet) TextVar` (Go 1.19+)
```go
func TextVar(p encoding.TextUnmarshaler, name string, value encoding.TextMarshaler, usage string)
func (f *FlagSet) TextVar(p encoding.TextUnmarshaler, name string, value encoding.TextMarshaler, usage string)
```
- **الوصف:** تربط راية بمتغير يحقق واجهة `encoding.TextUnmarshaler`.
- **المعاملات:**
  - `p`: مؤشر لمتغير يطبق `UnmarshalText([]byte) error`.
  - `value`: القيمة الافتراضية مطبقة لواجهة `encoding.TextMarshaler`.
- **الشروط:** يجب أن يكون نوع `value` مطابقاً تماماً لنوع المتغير المشار إليه في `p`، وإلا أطلقت الدالة هلعاً فورياً (`panic`).

```go
// مثال تطبيقي باستخدام net.IP
var host net.IP
flag.TextVar(&host, "host", net.IPv4(127, 0, 0, 1), "server listening IP")
```

---

### `Func` و `(f *FlagSet) Func` (Go 1.16+)
```go
func Func(name, usage string, fn func(string) error)
func (f *FlagSet) Func(name, usage string, fn func(string) error)
```
- **الوصف:** تُعرّف راية تقوم باستدعاء دالة رد النداء (Callback Closure) `fn` في كل مرة تظهر فيها الراية في سطر الأوامر.
- **معالجة الأخطاء:** إذا أعادت الدالة `fn` خطأ غير فارغ (`err != nil`)، يُعتبر ذلك فشلاً تحليلياً ويتم التعامل معه وفق سياسة الأخطاء.
- **الاستخدام الأمثل:** قراءة القيم المتكررة، أو إسناد إعدادات لعدة متغيرات معاً.

```go
var headers []string
flag.Func("H", "HTTP header to add", func(val string) error {
    if !strings.Contains(val, ":") {
        return errors.New("header must be in Key:Value format")
    }
    headers = append(headers, val)
    return nil
})
```

---

### `BoolFunc` و `(f *FlagSet) BoolFunc` (Go 1.21+)
```go
func BoolFunc(name, usage string, fn func(string) error)
func (f *FlagSet) BoolFunc(name, usage string, fn func(string) error)
```
- **الوصف:** مثل `Func` تماماً، ولكنها تُعامل كـ **راية منطقية** (تحقق واجهة `boolFlag`).
- **السلوك:** لا تتطلب تمرير قيمة بعدها؛ عند ذكر `-name` بمفردها، يتم استدعاء `fn("true")`. وإذا مُررت بصيغة `-name=false`، يتم استدعاء `fn("false")`.

---

### `Var` و `(f *FlagSet) Var`
```go
func Var(value Value, name string, usage string)
func (f *FlagSet) Var(value Value, name string, usage string)
```
- **الوصف:** الدالة المعمارية الأم لتعريف أي راية مخصصة تحقق واجهة `flag.Value`.
- **الشروط الصارمة:**
  - اسم الراية لا يجوز أن يبدأ بـ `-`.
  - اسم الراية لا يجوز أن يحتوي على `=`.
  - لا يجوز إعادة تعريف راية بنفس الاسم في نفس الـ `FlagSet`.
  - لا يجوز تعريف راية تم استدعاء `Set()` عليها مسبقاً قبل تعريفها (Issue 57411).

---

## 6. الاستكشاف والمسح والبحث (Introspection & Reflection)

### `Lookup` و `(f *FlagSet) Lookup`
```go
func Lookup(name string) *Flag
func (f *FlagSet) Lookup(name string) *Flag
```
- **الوصف:** يبحث في خريطة الرايات الرسمية (`formal`) عن الراية المسماة `name`.
- **المخرجات:** يُعيد مؤشر `*Flag` للراية في حال وجودها، أو `nil` إذا لم تكن مسجلة.

### `VisitAll` و `(f *FlagSet) VisitAll`
```go
func VisitAll(fn func(*Flag))
func (f *FlagSet) VisitAll(fn func(*Flag))
```
- **الوصف:** يمر على **كافة الرايات المسجلة رسمياً** (`formal`) بالترتيب الأبجدي المعجمي المعياري، ويستدعي الدالة `fn` لكل راية، حتى لو لم يتم تعيينها من سطر الأوامر.

### `Visit` و `(f *FlagSet) Visit`
```go
func Visit(fn func(*Flag))
func (f *FlagSet) Visit(fn func(*Flag))
```
- **الوصف:** يمر فقط على **الرايات التي تم تمريرها وضبطها فعلياً** في هذا التنفيذ (`actual`) بالترتيب الأبجدي، متجاهلاً الرايات التي لم يحددها المستخدم.

---

## 7. التعديل البرمجي للقيم: `Set`

### `Set` و `(f *FlagSet) Set`
```go
func Set(name, value string) error
func (f *FlagSet) Set(name, value string) error
```
- **الوصف:** يُعدل قيمة الراية المسماة `name` برمجياً بالقيمة النصية `value`.
- **الآثار الجانبية:**
  - يستدعي تابع `Value.Set(value)` للراية المعنية.
  - إذا نجح التعديل، يُضيف الراية إلى خريطة `actual` (وكأنها مُررت من سطر الأوامر).
  - إذا لم تكن الراية موجودة، يسجل مسار الملف والسطر في `undef[name]` ويعيد خطأً.

---

## 8. رسائل المساعدة والتوثيق (Help & Usage)

### `PrintDefaults` و `(f *FlagSet) PrintDefaults`
```go
func PrintDefaults()
func (f *FlagSet) PrintDefaults()
```
- **الوصف:** يطبع كافة الرايات المعرفة وقيمها الافتراضية ونصوصها الإرشادية إلى مجرى `Output()` الخاص بالمجموعة.
- **التنسيق:**
  - يتم استخراج أسماء المعاملات من الـ Backticks عبر `UnquoteUsage`.
  - الرايات البولينية المكونة من حرف واحد تُطبع في سطر واحد مضغوط.
  - الرايات الأخرى تُحاذى عبر علامات الجدولة (`\t`).
  - تُطبع القيمة الافتراضية بصيغة `(default ...)` فقط إذا كانت تختلف عن القيمة الصفرية لنوع الراية.

### `UnquoteUsage`
```go
func UnquoteUsage(flag *Flag) (name string, usage string)
```
- **الوصف:** تفحص نص `flag.Usage` بحثاً عن اسم محاط بـ Backticks (مثل `` `file` to parse ``).
- **المخرجات:**
  - إذا وجد تنصيص: تعيد الاسم المستخرج `name` والنص الوصفي منزوع التنصيص `usage`.
  - إذا لم تجد تنصيصاً: تعيد تخميناً تلقائياً ذكياً لنوع الراية (`"int"`, `"float"`, `"string"`, `"duration"` إلخ)، أو نصاً فارغاً `""` إذا كانت الراية منطقية.
