# أفضل الممارسات العالمية للتعامل مع الأخطاء (Error Handling) في مشاريع Go الكبيرة

> **تاريخ البحث:** 2026-09-21  
> **المصادر الرسمية والمراجع المعتمدة:** Go Official Documentation, Go Official Blog, Effective Go, Go Proposal #56345 & #53435, Google Go Style Guide, Uber Go Style Guide, Dave Cheney, Ben Johnson (Upspin & Failure Domain Pattern)

---

## جدول المحتويات

1. [الفلسفة التأسيسية: "الأخطاء قيم" (Errors are Values)](#1-الفلسفة-التأسيسية-الأخطاء-قيم-errors-are-values)
2. [التطور المعياري لنظام الأخطاء في Go](#2-التطور-المعياري-لنظام-الأخطاء-في-go)
3. [أنواع واستراتيجيات الأخطاء (Taxonomy of Errors)](#3-أنواع-واستراتيجيات-الأخطاء-taxonomy-of-errors)
4. [ميكانيكا التغليف وفحص السلسلة (Wrapping & Inspection)](#4-ميكانيكا-التغليف-وفحص-السلسلة-wrapping--inspection)
5. [تجميع الأخطاء المتعددة: `errors.Join` (Go 1.20+)](#5-تجميع-الأخطاء-المتعددة-errorsjoin-go-120)
6. [المعمارية متعددة الطبقات وعزل نطاق الفشل (Layered Architecture & Failure Domains)](#6-المعمارية-متعددة-الطبقات-وعزل-نطاق-الفشل-layered-architecture--failure-domains)
7. [نمط Ben Johnson المعماري: إدارة الأخطاء في المشاريع الكبيرة](#7-نمط-ben-johnson-المعماري-إدارة-الأخطاء-في-المشاريع-الكبيرة)
8. [التعامل مع الأخطاء في العمليات المتزامنة والموارد (Concurrency & Defer Cleanup)](#8-التعامل-مع-الأخطاء-في-العمليات-المتزامنة-والموارد-concurrency--defer-cleanup)
9. [حدود الذعر والاسترداد (Panic and Recover Boundary)](#9-حدود-الذعر-والاسترداد-panic-and-recover-boundary)
10. [الأمان وحماية البيانات في رسائل الأخطاء](#10-الأمان-وحماية-البيانات-في-رسائل-الأخطاء)
11. [الأنماط المضادة الشائعة (Anti-Patterns)](#11-الأنماط-المضادة-الشائعة-anti-patterns)
12. [قائمة مراجعة الإنتاج للمشاريع الكبيرة (Production Checklist)](#12-قائمة-مراجعة-الإنتاج-للمشاريع-الكبيرة-production-checklist)
13. [المصادر والمراجع الرسمية](#13-المصادر-والمراجع-الرسمية)

---

## 1. الفلسفة التأسيسية: "الأخطاء قيم" (Errors are Values)

تختلف لغة Go جوهرياً عن اللغات التي تعتمد على الاستثناءات (Exceptions) مثل Java و C# و Python. الخطأ في Go ليس تدفق تحكم خفي يقفز فوق مكدس الاستدعاءات (Stack Unwinding)، بل هو **قيمة عادية من الدرجة الأولى (First-Class Value)**.

> **المصدر الرسمي:** مقال Rob Pike الشهير في مدونة Go الرسمية: [Errors are values](https://go.dev/blog/errors-are-values)

### المبادئ الثلاثة الحاكمة

1. **لا تدفق تحكم خفي (No Hidden Control Flow):**  
   في اللغات القائمة على الاستثناءات، لا يمكنك معرفة ما إذا كانت الدالة قد تنهار أو ما هي أنواع الاستثناءات التي قد تقذفها دون قراءة تفاصيل كودها أو توثيقها. في Go، التوقيع (Signature) يوضح صراحة ما إذا كانت الدالة قد تفشل أم لا:

   ```go
   // الدالة تعلن بصراحة أنها قد تفشل وتلزم المستدعي بالتعامل مع النتيجة
   func FindUser(ctx context.Context, id string) (*User, error)
   ```

2. **الفحص الفوري والحاسم (Fail Fast):**  
   يجب التعامل مع الخطأ مباشرة في نفس نقطة حدوثه أو إرجاعه فوراً للطبقة الأعلى مع سياقه. تأجيل فحص الأخطاء يؤدي إلى حالات عدم اتساق (Inconsistent State).

3. **واجهة `error` المدمجة:**  
   في Go، الواجهة `error` هي أبسط واجهة ممكنة، معرّفة مسبقاً في الـ `builtin`:

   ```go
   type error interface {
       Error() string
   }
   ```

   أي نوع بيانات (struct, string, int) يطبق دالة `Error() string` يصبح خطأً شرعياً تلقائياً، مما يمنح اللغة مرونة هائلة دون الحاجة لأي تسلسلات هرمية معقدة من الوراثة.

---

## 2. التطور المعياري لنظام الأخطاء في Go

مر التعامل مع الأخطاء في Go بثلاث محطات مفصلية تاريخية ورسمية يجب على كل مهندس برمجيات استيعابها:

```text
┌─────────────────────────┐    ┌─────────────────────────┐    ┌─────────────────────────┐
│   Go 1.0 ── Go 1.12     │    │   Go 1.13 (سبتمبر 2019) │    │   Go 1.20 (فبراير 2023) │
├─────────────────────────┤    ├─────────────────────────┤    ├─────────────────────────┤
│ • errors.New            │    │ • التغليف الرسمي عبر %w   │    │ • errors.Join           │
│ • fmt.Errorf("%v")      │    │ • errors.Is             │    │ • Unwrap() []error      │
│ • انتشار pkg/errors     │    │ • errors.As             │    │ • دعم شجرة أخطاء متعددة │
│   بسبب غياب التغليف     │    │ • واجهة Unwrap() error  │    │   في Is و As            │
└─────────────────────────┘    └─────────────────────────┘    └─────────────────────────┘
```

### 1. مرحلة Go 1.0 إلى Go 1.12 (عصر `pkg/errors`)

كانت الطريقة المتاحة لإنشاء خطأ هي `errors.New` أو `fmt.Errorf`. عند كتابة:

```go
return fmt.Errorf("read config failed: %v", err)
```

كان الخطأ الأصلي يتحول إلى مجرد نص (String formatting) ويفقد هويته البرمجية ونوعه الأصلي. هذا دفع المجتمع لابتكار مكتبة `github.com/pkg/errors` بواسطة Dave Cheney لإضافة ميزات الـ Wrapping والـ Stack Traces.

### 2. نقطة التحول: Go 1.13

تبنى الفريق الرسمي في Go مقترح التغليف المعياري رسمياً، مما جعل مكتبة `pkg/errors` مهملة (Deprecated/Archived):

- تقديم الرمز `%w` داخل دالة `fmt.Errorf`.
- دوال الفحص الرسمية: `errors.Is` و `errors.As`.
- واجهة فك التغليف: `Unwrap() error`.

### 3. مرحلة النضج: Go 1.20+ (Multi-Error Wrapping)

- إضافة `errors.Join` لجمع عدة أخطاء في قيمة واحدة دون مكتبات خارجية.
- دعم واجهة `Unwrap() []error` للتعامل مع شجرة أخطاء (Error Trees).

---

## 3. أنواع واستراتيجيات الأخطاء (Taxonomy of Errors)

في المشاريع الكبيرة، لا يمكن التعامل مع جميع الأخطاء بنفس الطريقة. صنّف خبراء Go (مثل Dave Cheney في مؤتمر GopherCon) استراتيجيات الأخطاء إلى 4 مستويات:

```text
                 أنواع استراتيجيات الأخطاء في Go
       ┌─────────────────────────┼─────────────────────────┐
       ▼                         ▼                         ▼
┌───────────────┐       ┌─────────────────┐       ┌─────────────────┐
│Sentinel Errors│       │Custom Type Error│       │ Opaque Errors   │
│(قيم الحارس)   │       │(الهياكل المخصصة)│       │(الأخطاء المعتمة)│
├───────────────┤       ├─────────────────┤       ├─────────────────┤
│var ErrNotFound│       │type PathError   │       │IsTimeout(err)   │
│مقارنة هوية    │       │حمل بيانات إضافية │       │فحص سلوك لا نوع  │
└───────────────┘       └─────────────────┘       └─────────────────┘
```

### أ) أخطاء الحارس (Sentinel Errors)

هي متغيرات عامة ثابتة في حزمة معينة تعبر عن حالة خطأ متوقعة محددة لا تتغير تفاصيلها:

```go
package repository

import "errors"

// التسمية القياسية: تبدأ دائماً بـ Err
var (
    ErrNotFound     = errors.New("record not found")
    ErrConflict     = errors.New("record already exists")
    ErrUnauthorized = errors.New("unauthorized access")
)
```

#### متى تستخدمها؟

- عندما يحتاج المستدعي اتخاذ قرار تفريعي محدد (مثال: إذا لم يجد السجل ينشئ سجلاً جديداً).
- الأخطاء القياسية المعرفة في المكتبة القياسية مثل `io.EOF`, `sql.ErrNoRows`.

#### العيوب والتحذيرات

- **الاقتران القوي (Tight Coupling):** تجبر المستدعي على استيراد الحزمة الخاصة بك للتحقق من المتغير.
- **ثبات النص:** لا يمكنها حمل تفاصيل ديناميكية (مثل: أي سجل تحديداً لم يتم العثور عليه).

---

### ب) هياكل الأخطاء المخصصة (Custom Error Types)

عندما يحتاج المستدعي إلى **بيانات مهيكلة إضافية** عن الفشل لاتخاذ قرارات برمجية دقيقة:

```go
package validator

import (
    "fmt"
    "strings"
)

// FieldViolation يمثل تفاصيل حقل معين غير صالح
type FieldViolation struct {
    Field       string `json:"field"`
    Description string `json:"description"`
}

// ValidationError خطأ مهيكل لحالات فشل التحقق من المدخلات
type ValidationError struct {
    Entity     string
    Violations []FieldViolation
}

func (e *ValidationError) Error() string {
    msgs := make([]string, len(e.Violations))
    for i, v := range e.Violations {
        msgs[i] = fmt.Sprintf("%s: %s", v.Field, v.Description)
    }
    return fmt.Sprintf("validation failed for %s: [%s]", e.Entity, strings.Join(msgs, ", "))
}
```

---

### ج) الأخطاء المعتمة وفحص السلوك (Opaque Errors & Behavior Assertion)

تعتبر هذه الاستراتيجية من أكثر الاستراتيجيات مرونة في تقليل الاقتران البرمجي (Decoupling) في المشاريع الكبيرة بحسب **Dave Cheney**:

بدلاً من فحص النوع الصريح للخطأ، يتحقق الكود مما إذا كان الخطأ يدعم **سلوكاً معيناً** (Behavioral Interface):

```go
package client

import "errors"

// واجهة داخلية للتحقق من إمكانية إعادة المحاولة
type retryable interface {
    Retryable() bool
}

// IsRetryable تفحص السلوك دون الحاجة لمعرفة نوع الخطأ الحقيقي
func IsRetryable(err error) bool {
    var r retryable
    if errors.As(err, &r) {
        return r.Retryable()
    }
    return false
}

// نوع خطأ ينفذ هذا السلوك
type NetworkTimeoutError struct {
    Host string
}

func (e *NetworkTimeoutError) Error() string {
    return fmt.Sprintf("connection timeout to %s", e.Host)
}

func (e *NetworkTimeoutError) Retryable() bool {
    return true // هذا الخطأ مؤقت ويقبل إعادة المحاولة
}
```

---

## 4. ميكانيكا التغليف وفحص السلسلة (Wrapping & Inspection)

### متى نستخدم `%w` مقابل `%v`؟ (قرار معماري استراتيجي)

> **المصدر الرسمي:** مقال مدونة Go: [Working with Errors in Go 1.13](https://go.dev/blog/go1.13-errors)

ليس كل خطأ يجب تغليفه بـ `%w`. استخدام `%w` ينشئ عقداً عاماً (Public API Contract) بين الحزمة ومستخدميها:

| المحدد | المعنى البرمجي | متى يُستخدم؟ |
| :------- | :--------------- | :------------- |
| **`%w`** | **Wrap:** الخطأ المغلف جزء من الـ API العام. يمكن للمستدعي كشفه وفحصه باستخدام `errors.Is` و `errors.As`. | عندما تريد السماح للمستدعي بفحص السبب الجذري والتفاعل معه. |
| **`%v`** | **Mask (إخفاء):** الخطأ يتحول إلى نص عادي فقط. ينقطع حبل فك التغليف (`Unwrap`). | عندما يكون الخطأ تفصيلاً داخلياً (Implementation Detail) مثل نوع قاعدة البيانات المستخدمة ولا تريد تسريبه خارج الحزمة. |

```go
// 1. استخدام %w — المستدعي يستطيع اكتشاف أن السبب هو ErrNotFound
func GetUser(id string) (*User, error) {
    user, err := repo.Find(id)
    if err != nil {
        return nil, fmt.Errorf("service.GetUser id %s: %w", id, err)
    }
    return user, nil
}

// 2. استخدام %v — إخفاء تفاصيل التخزين ومنع الاقتران بمكتبة معينة
func SaveToken(token string) error {
    if err := redisClient.Set(token).Err(); err != nil {
        // المستدعي لا يحتاج معرفة أننا نستخدم Redis داخلياً
        return fmt.Errorf("token cache failure: %v", err)
    }
    return nil
}
```

---

### الفحص الصحيح: `errors.Is` مقابل `==`

في Go الحديثة، **يُحظر تماماً استخدام `==` لمقارنة الأخطاء** في بيئة الإنتاج ما لم تكن متأكداً 100% أن الخطأ لم ولن يُغلف أبداً:

```go
// ❌ خطأ فادح: يفشل إذا تم تغليف الخطأ في أي طبقة فرعية بواسطة %w
if err == sql.ErrNoRows { 
    // لن يُنفذ هذا الشرط إذا كان الخطأ: fmt.Errorf("query failed: %w", sql.ErrNoRows)
}

// ✅ الأسلوب العالمي الصحيح: يتتبع السلسلة بالكامل حتى السبب الجذري
if errors.Is(err, sql.ErrNoRows) {
    // ينجح دائماً طالما أن الخطأ أو أي خطأ مغلف بداخله هو sql.ErrNoRows
}
```

#### تخصيص `Is(target error) bool` داخل الهياكل

يمكنك تخصيص منطق الفحص داخل الـ Custom Error عن طريق تنفيذ دالة `Is`:

```go
type QueryError struct {
    Query string
    Err   error
}

func (e *QueryError) Error() string {
    return fmt.Sprintf("query %q: %v", e.Query, e.Err)
}

// تخصيص Is للتحقق مما إذا كان الخطأ الداخلي يطابق الهدف
func (e *QueryError) Is(target error) bool {
    return errors.Is(e.Err, target)
}
```

---

### استخراج النوع بواسطة `errors.As`

تُستخدم `errors.As` للبحث في سلسلة التغليف عن أول خطأ يطابق نوعاً معيناً واستخراج قيمته:

```go
// ⚠️ انتبه لمستوى المؤشر (Pointer Level):
var netErr net.Error

// نمرر مؤشر لمتغير الواجهة أو مؤشر للـ struct
if errors.As(err, &netErr) {
    if netErr.Timeout() {
        // معالجة حالة انتهاء الوقت
    }
}
```

```go
// مثال لاستخراج Custom Struct:
var valErr *validator.ValidationError
if errors.As(err, &valErr) {
    // يمكننا الآن الوصول إلى valErr.Violations بأمان تام
    for _, violation := range valErr.Violations {
        log.Printf("Field %s: %s", violation.Field, violation.Description)
    }
}
```

---

## 5. تجميع الأخطاء المتعددة: `errors.Join` (Go 1.20+)

> **المصدر الرسمي:** مقترح Go Proposal [#53435](https://go.dev/issue/53435)

في كثير من سيناريوهات المشاريع الكبيرة، تواجه عمليات قد ينتج عنها أكثر من خطأ واحد (مثل: إغلاق عدة اتصالات، تنفيذ مهام متوازية، أو جمع كافة أخطاء التحقق من البيانات). قبل Go 1.20 كان المطورون يضطرون لاستخدام مكتبات خارجية مثل `uber-go/multierr`. الآن حزمة `errors` الرسمية توفر `errors.Join`.

```go
func closeResources(r1 io.Closer, r2 io.Closer) error {
    var errs []error

    if err := r1.Close(); err != nil {
        errs = append(errs, fmt.Errorf("closing r1: %w", err))
    }
    if err := r2.Close(); err != nil {
        errs = append(errs, fmt.Errorf("closing r2: %w", err))
    }

    // إذا كانت جميع القيم nil، ترجع الدالة nil تلقائياً
    return errors.Join(errs...)
}
```

### كيف تعمل `errors.Join` مع `errors.Is` و `errors.As`؟

الخطأ الناتج عن `errors.Join` يطبق واجهة `Unwrap() []error`. لذلك فإن `errors.Is` و `errors.As` تقومان **بفحص شجرة الأخطاء بالكامل**:

```go
err1 := sql.ErrNoRows
err2 := os.ErrPermission
jointErr := errors.Join(err1, err2)

// كلاهما سيرجع true!
errors.Is(jointErr, sql.ErrNoRows)    // true
errors.Is(jointErr, os.ErrPermission) // true
```

---

## 6. المعمارية متعددة الطبقات وعزل نطاق الفشل (Layered Architecture & Failure Domains)

في البنى المعمارية النظيفة (Clean Architecture / Hexagonal Architecture)، تُعد إدارة الأخطاء عبر الحدود (Boundaries) من أهم مقاييس جودة الكود.

```text
┌─────────────────────────────────────────────────────────────┐
│ 1. Presentation Layer (HTTP / gRPC Handlers)                │
│    • نقطة الحسم: تحويل خطأ النطاق إلى HTTP Status Code     │
│    • حجب البيانات الحساسة وتسجيل الخطأ التقني في الـ Log     │
└──────────────────────────────▲──────────────────────────────┘
                               │ Domain Errors
┌──────────────────────────────┴──────────────────────────────┐
│ 2. Application / Service Layer (Use Cases)                   │
│    • تنسيق العمليات وإضافة السياق الإجرائي (Operation Context)│
│    • fmt.Errorf("checkout order %s: %w", orderID, err)      │
└──────────────────────────────▲──────────────────────────────┘
                               │ Domain / Standard Errors
┌──────────────────────────────┴──────────────────────────────┐
│ 3. Domain Layer (Business Logic)                            │
│    • أخطاء نطاق نقية: ErrInsufficientFunds, ErrOrderPaid   │
│    • معزولة تماماً عن أي تبعيات خارجية                       │
└──────────────────────────────▲──────────────────────────────┘
                               │ Translated Errors
┌──────────────────────────────┴──────────────────────────────┐
│ 4. Infrastructure / Repository Layer (DB, Redis, Third-Party)│
│    • ترجمة أخطاء PostgreSQL / Mongo إلى أخطاء نطاق         │
│    • منع تسريب تفاصيل التخزين للطبقات العليا (No SQL leaks) │
└─────────────────────────────────────────────────────────────┘
```

### قاعدة ترجمة الأخطاء في طبقة التخزين (Repository Error Translation)

**لا تسمح أبداً لأخطاء التخزين الخام بالمرور للطبقات العليا.** إذا غيرت قاعدة البيانات لاحقاً من PostgreSQL إلى DynamoDB، يجب ألا يتأثر كود الخدمات إطلاقاً:

```go
package postgres

import (
    "database/sql"
    "errors"
    "fmt"
    "myproject/internal/domain"
)

type UserRepository struct {
    db *sql.DB
}

func (r *UserRepository) GetByID(ctx context.Context, id string) (*domain.User, error) {
    row := r.db.QueryRowContext(ctx, "SELECT id, name FROM users WHERE id = $1", id)
    
    var user domain.User
    if err := row.Scan(&user.ID, &user.Name); err != nil {
        if errors.Is(err, sql.ErrNoRows) {
            // ✅ ترجمة الخطأ إلى خطأ نطاق معروف ومستقل عن الـ SQL
            return nil, fmt.Errorf("user %s: %w", id, domain.ErrUserNotFound)
        }
        // الأخطاء التقنية البحتة تُغلف مع سياقها
        return nil, fmt.Errorf("database query failed for user %s: %w", id, err)
    }

    return &user, nil
}
```

---

## 7. نمط Ben Johnson المعماري: إدارة الأخطاء في المشاريع الكبيرة

> **المصدر المعتمد عالمياً:** مقال Ben Johnson: [Failure is your Domain](https://middlemost.com/failure-is-your-domain/) (مطور BoltDB ومصمم بنية Upspin في Google مع Rob Pike).

يقوم هذا النمط على حقيقة جوهرية: **لكل خطأ 3 مستهلكين مختلفين تماماً**:

1. **النظام والمنطق البرمجي (The Application):** يحتاج كوداً برمجياً ثابتاً لاتخاذ قرار (مثل: `NOT_FOUND`, `CONFLICT`, `UNAUTHORIZED`).
2. **المستخدم النهائي (The End User):** يحتاج رسالة إنسانية واضحة، مهذبة، وخالية تماماً من المصطلحات التقنية أو تفاصيل النظام الداخلي.
3. **مهندس التشغيل (The Operator):** يحتاج المسار المنطقي الدقيق للعملية (`Logical Stack Trace`)، مع معرفة الخطأ الأصلي لتصحيحه.

### تطبيق النمط المعياري للإنتاج

```go
package apperror

import (
    "bytes"
    "fmt"
)

// أكواد الأخطاء القياسية المعرّفة للتطبيق
const (
    CodeInternal     = "INTERNAL"
    CodeNotFound     = "NOT_FOUND"
    CodeConflict     = "CONFLICT"
    CodeInvalid      = "INVALID"
    CodeUnauthorized = "UNAUTHORIZED"
)

// Error البنية الموحدة للأخطاء في المشاريع الكبيرة
type Error struct {
    // الكود البرمجي الذي يحدد نوع الفشل للتطبيق والـ HTTP Handler
    Code string

    // رسالة موجهة ومناسبة للمستخدم النهائي
    Message string

    // اسم العملية المنطقية التي حدث فيها الفشل (Logical Call Stack)
    Op string

    // الخطأ الداخلي المتداخل أو الأصلي
    Err error
}

// Error يطبع المسار المنطقي بالكامل للمهندسين واللوق
func (e *Error) Error() string {
    var b bytes.Buffer

    if e.Op != "" {
        fmt.Fprintf(&b, "%s: ", e.Op)
    }

    if e.Code != "" {
        fmt.Fprintf(&b, "<%s> ", e.Code)
    }

    if e.Message != "" {
        fmt.Fprintf(&b, "%s: ", e.Message)
    }

    if e.Err != nil {
        b.WriteString(e.Err.Error())
    }

    return b.String()
}

// Unwrap يتيح التوافق مع errors.Is و errors.As
func (e *Error) Unwrap() error {
    return e.Err
}

// دوال مساعدة لاستخراج الكود والرسالة من أي خطأ (حتى لو كان مغلفاً)
func ErrorCode(err error) string {
    if err == nil {
        return ""
    }
    var appErr *Error
    if ok := errors.As(err, &appErr); ok && appErr.Code != "" {
        return appErr.Code
    }
    return CodeInternal
}

func ErrorMessage(err error) string {
    if err == nil {
        return ""
    }
    var appErr *Error
    if ok := errors.As(err, &appErr); ok && appErr.Message != "" {
        return appErr.Message
    }
    return "An internal server error occurred. Please try again later."
}
```

### كيف يترجم هذا النمط في طبقة الـ HTTP Middleware؟

```go
func HandleHTTPError(w http.ResponseWriter, r *http.Request, err error, logger *slog.Logger) {
    if err == nil {
        return
    }

    code := apperror.ErrorCode(err)
    userMessage := apperror.ErrorMessage(err)

    var status int
    switch code {
    case apperror.CodeNotFound:
        status = http.StatusNotFound
    case apperror.CodeInvalid:
        status = http.StatusBadRequest
    case apperror.CodeConflict:
        status = http.StatusConflict
    case apperror.CodeUnauthorized:
        status = http.StatusUnauthorized
    default:
        status = http.StatusInternalServerError
    }

    // للمشغلين: نسجل الخطأ كاملاً بمساره وعمقه
    if status >= 500 {
        logger.ErrorContext(r.Context(), "server failure", "error", err.Error(), "code", code)
    } else {
        logger.WarnContext(r.Context(), "client rejection", "error", err.Error(), "code", code)
    }

    // للمستخدم النهائي: نعيد فقط الكود والرسالة المهذبة
    w.Header().Set("Content-Type", "application/json")
    w.WriteHeader(status)
    json.NewEncoder(w).Encode(map[string]any{
        "code":    code,
        "message": userMessage,
    })
}
```

---

## 8. التعامل مع الأخطاء في العمليات المتزامنة والموارد (Concurrency & Defer Cleanup)

### أ) أخطاء الـ Goroutines و `errgroup.Group`

في بيئات الإنتاج، لا يجب إطلاق Goroutine بدون آلية لالتقاط أخطائها. الحزمة الرسمية المعتمدة هي `golang.org/x/sync/errgroup`:

```go
package service

import (
    "context"
    "fmt"
    "golang.org/x/sync/errgroup"
)

func ProcessBatch(ctx context.Context, items []string) error {
    // ينشئ context يلغى تلقائياً بمجرد فشل أي مهمة متزامنة
    g, ctx := errgroup.WithContext(ctx)

    for _, item := range items {
        item := item // تجنب مشكلات الإغلاق في الإصدارات الأقدم
        g.Go(func() error {
            if err := processSingleItem(ctx, item); err != nil {
                return fmt.Errorf("item %s processing failed: %w", item, err)
            }
            return nil
        })
    }

    // ينتظر اكتمال كافة الـ goroutines ويعيد أول خطأ غير فارغ
    if err := g.Wait(); err != nil {
        return fmt.Errorf("batch processing aborted: %w", err)
    }

    return nil
}
```

---

### ب) نمط التقاط أخطاء `defer` مع دمجها (`errors.Join`)

خطأ شهير جداً في Go: نسيان التقاط خطأ `file.Close()` أو `rows.Close()` في استدعاءات `defer`. استخدام القيم المعادة المسمّاة (Named Return Values) مع `errors.Join` يحل هذه المعضلة بنظافة:

```go
func WriteData(filename string, data []byte) (err error) {
    f, err := os.Create(filename)
    if err != nil {
        return fmt.Errorf("create file: %w", err)
    }

    // التقاط خطأ الإغلاق ودمجه مع أي خطأ قد يكون حدث أثناء الكتابة
    defer func() {
        if closeErr := f.Close(); closeErr != nil {
            err = errors.Join(err, fmt.Errorf("close file: %w", closeErr))
        }
    }()

    if _, writeErr := f.Write(data); writeErr != nil {
        return fmt.Errorf("write data: %w", writeErr)
    }

    return nil
}
```

---

## 9. حدود الذعر والاسترداد (Panic and Recover Boundary)

> **المصدر الرسمي:** وثيقة [Effective Go](https://go.dev/doc/effective_go#panic) و Google Go Style Guide.

القاعدة الذهبية في لغة Go: **لا تستخدم `panic` للتحكم في تدفق البرنامج العادي (Control Flow).**

### متى يُسمح باستخدام `panic`؟

1. **أثناء إقلاع التطبيق (Startup/Initialization):**  
   إذا كانت الإعدادات الأساسية غير صالحة أو فشل الاتصال بقاعدة البيانات الحيوية، فمن الأفضل إنهاء التطبيق فوراً (`Fail Fast`).
2. **انتهاك الفرضيات المستحيلة منطقياً (Programmer Invariant Violations):**  
   مؤشر فارغ غير متوقع ناتج عن خطأ برمجي بحت وليس مدخلات خارجية.

### نمط الـ Recover Boundary في الخوادم

يجب ألا يؤدي ذعر في طلب واحد إلى إسقاط خادم الويب بأكمله. يتم وضع Middleware عند حدود الخادم:

```go
func RecoveryMiddleware(next http.Handler, logger *slog.Logger) http.Handler {
    return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
        defer func() {
            if rvr := recover(); rvr != nil {
                // التقاط مكدس الاستدعاءات
                stack := debug.Stack()

                // تسجيل الذعر كخطأ كارثي مع التفاصيل
                logger.ErrorContext(r.Context(), "panic recovered in http handler",
                    slog.Any("panic", rvr),
                    slog.String("stack", string(stack)),
                    slog.String("path", r.URL.Path),
                )

                // إعادة استجابة آمنة للعميل دون كشف أي تفاصيل داخلية
                w.Header().Set("Content-Type", "application/json")
                w.WriteHeader(http.StatusInternalServerError)
                json.NewEncoder(w).Encode(map[string]string{
                    "error": "A critical internal error occurred",
                })
            }
        }()

        next.ServeHTTP(w, r)
    })
}
```

---

## 10. الأمان وحماية البيانات في رسائل الأخطاء

في المشاريع الكبيرة وتطبيقات المؤسسات، تعتبر رسائل الأخطاء إحدى أكبر ثغرات تسريب البيانات (Information Disclosure):

### 1. إخفاء تفاصيل البنية التحتية

- **ممنوع للمستخدم النهائي:**  
  `pq: password authentication failed for user "postgres"` أو `dial tcp 10.0.4.15:5432: connection refused`
- **البديل الموجه للعميل:**  
  `Unable to complete operation. Please contact support if the issue persists.`

### 2. تنقية البيانات الحساسة (PII & Credentials Redaction)

تأكد من عدم تمرير كلمات المرور، رموز JWT، أو أرقام البطاقات الائتمانية في معاملات `fmt.Errorf`:

```go
// ❌ خطأ أمني كارثي: تسريب كلمة المرور في اللوق وسلسلة الأخطاء
return fmt.Errorf("failed to authenticate user %s with password %s: %w", user, password, err)

// ✅ أسلوب آمن: الإشارة للهوية دون البيانات السرية
return fmt.Errorf("authentication failed for user %q: %w", user, err)
```

---

## 11. الأنماط المضادة الشائعة (Anti-Patterns)

تجنب الوقوع في هذه الممارسات الخاطئة الشائعة في مشاريع Go:

### 1. نمط "سجل وأرجع الخطأ" (The "Log and Return" Anti-Pattern)
>
> **تحذير صارم من Uber Go Style Guide و Dave Cheney:**

```go
// ❌ نمط مضاد: تسجيل الخطأ ثم إرجاعه في كل طبقة
func ReadConfig() (*Config, error) {
    data, err := os.ReadFile("config.json")
    if err != nil {
        log.Printf("failed to read file: %v", err) // 1. تسجيل هنا
        return nil, err
    }
    // ...
}

func InitApp() error {
    cfg, err := ReadConfig()
    if err != nil {
        log.Printf("init app failed: %v", err) // 2. تسجيل مكرر لنفس الخطأ
        return err
    }
    // ...
}
```

**المشكلة:** يمتلئ السجل بسطور مكررة لنفس الحادثة، مما يصعب تتبع المشكلة الحقيقية وتصنيف التنبيهات.  
**القاعدة:** **إما أن تعالج الخطأ وتسجله (Handle & Log)، أو تغلفه وترجعه (Wrap & Return)، ولا تفعل الاثنين معاً أبداً.**

---

### 2. فحص الأخطاء عبر السلاسل النصية (String Matching Anti-Pattern)

```go
// ❌ نمط هش وسهل الكسر
if strings.Contains(err.Error(), "connection refused") {
    // إذا تغير نص الخطأ في تحديث لاحق للمكتبة، سيتعطل نظامك دون أي تحذير برمجي!
}

// ✅ البديل الآمن: استخدام errors.Is أو فحص النوع / السلوك
var netErr net.Error
if errors.As(err, &netErr) && netErr.Timeout() {
    // معالجة معتمدة برمجياً ومستقرة
}
```

---

### 3. إهمال الأخطاء بواسطة المعرّف الفارغ (`_`)

```go
// ❌ إخفاء الفشل بصمت
_ = conn.Close()

// ✅ الأفضل: فحص الخطأ أو تسجيله على الأقل إذا كان غير حرج
if err := conn.Close(); err != nil {
    logger.Debug("failed to close connection", "error", err)
}
```

---

### 4. التنسيق غير القياسي لرسائل الأخطاء (Capitalization & Punctuation)

تنص وثيقة الـ Code Review Comments الرسمية لـ Go على:
> "Error strings should not be capitalized (unless beginning with proper nouns or acronyms) or end with punctuation."

```go
// ❌ مخالف للمعايير القياسية لـ Go
return errors.New("Database connection failed.") 

// النتيجة عند التغليف تبدو مشوهة:
// "server start: Database connection failed.: Connection refused."

// ✅ مطابق للمعايير القياسية لـ Go
return errors.New("database connection failed")
// النتيجة عند التغليف:
// "server start: database connection failed: connection refused"
```

---

## 12. قائمة مراجعة الإنتاج للمشاريع الكبيرة (Production Checklist)

قبل اعتماد كود الخدمة للإنتاج، تأكد من مراجعة البنود التالية:

- [ ] **الفلسفة:** هل تُعامل الأخطاء كقيم ويتم التحقق منها فوراً دون إهمال؟
- [ ] **الفحص:** هل يتم استخدام `errors.Is` بدلاً من `==` للتحقق من أخطاء الحارس؟
- [ ] **الاستخراج:** هل يتم استخدام `errors.As` بدلاً من الـ Type Assertion اليدوي (`err.(*MyType)`)؟
- [ ] **التغليف الدقيق:** هل تم استخدام `%w` فقط للأخطاء المصممة لتكون جزءاً من الـ API العام، و `%v` للأخطاء الداخلية التي يجب حجبها؟
- [ ] **التسمية:** هل تبدأ رسائل الأخطاء بحروف صغيرة (lowercase) ودون نقطة في النهاية؟
- [ ] **الازدواجية:** هل تم الالتزام بقاعدة عدم الجمع بين التسجيل والإرجاع ("Do not log and return")؟
- [ ] **عزل الطبقات:** هل تترجم طبقة التخزين (Repository) أخطاء الـ SQL والمحركات إلى أخطاء نطاق (Domain Errors)؟
- [ ] **العمليات المتزامنة:** هل يتم استخدام `errgroup` بدلاً من ترك الـ Goroutines تفشل بصمت في الخلفية؟
- [ ] **الموارد والتنظيف:** هل يتم التقاط أخطاء الـ `defer Close()` ودمجها عبر `errors.Join` في الدوال الحساسة؟
- [ ] **الذعر:** هل الخادم محمي بـ `Recovery Middleware` على أعلى مستوى لمنع انهيار العملية بالكامل؟
- [ ] **الأمان والخصوصية:** هل تم تنقية رسائل الأخطاء الموجهة للمستخدم النهائي من أي بيانات سرية أو تفاصيل تقنية للبنية التحتية؟

---

## 13. المصادر والمراجع الرسمية

1. **مدونة Go الرسمية:** [Working with Errors in Go 1.13](https://go.dev/blog/go1.13-errors) — Damian Gryski.
2. **مدونة Go الرسمية:** [Errors are values](https://go.dev/blog/errors-are-values) — Rob Pike.
3. **مدونة Go الرسمية:** [Error handling and Go](https://go.dev/blog/error-handling-and-go) — Andrew Gerrand.
4. **دليل Go الرسمي:** [Effective Go - Errors and Panics](https://go.dev/doc/effective_go#errors).
5. **إرشادات مراجعة الكود الرسمية:** [Go Code Review Comments: Error Strings](https://go.dev/wiki/CodeReviewComments#error-strings).
6. **المقترح الرسمي Go Proposal #53435:** [errors: add support for wrapping multiple errors (`errors.Join`)](https://go.dev/issue/53435).
7. **Google Go Style Guide:** [Google Go Style Decisions: Error Handling](https://google.github.io/styleguide/go/decisions.html#handle-errors).
8. **Uber Go Style Guide:** [Uber Go Guide - Errors Handling and Wrapping](https://github.com/uber-go/guide/blob/master/style.md#errors).
9. **Dave Cheney:** [Don't just check errors, handle them gracefully](https://dave.cheney.net/2016/04/27/dont-just-check-errors-handle-them-gracefully).
10. **Ben Johnson:** [Failure is your Domain](https://middlemost.com/failure-is-your-domain/) (مطور BoltDB وتطبيق معمارية Upspin للأخطاء).
