# 02. الواجهات والهياكل والأنواع الأساسية في حزمة `context`

تعتمد حزمة `context` في صميمها على تصميم مقتضب وبالغ الدقة؛ فبدلاً من الاعتماد على واجهات متضخمة أو هياكل معقدة، تتألف الحزمة من واجهة عامة واحدة، و3 واجهات داخلية خاصة، و7 هياكل ملموسة (Concrete Structs) تُشكل بمجموعها العمود الفقري لشجرة السياق.

---

## 🌟 1. الواجهات (Interfaces)

### أ. الواجهة العامة الرئيسية: `context.Context`

الكود المصدري الرسمي:

```go
type Context interface {
    Deadline() (deadline time.Time, ok bool)
    Done() <-chan struct{}
    Err() error
    Value(key any) any
}
```

تحتوي الواجهة على 4 دوال فقط، وتُمثل العقد الرسمي لنقل الإشارات والبيانات عبر حدود العمليات:

#### 1. `Deadline() (deadline time.Time, ok bool)`

- **الوظيفة:** تعيد الوقت المحدد (الموعد النهائي) الذي يجب عنده إلغاء كافة الأعمال المرتبطة بهذا السياق.
- **القيمة المرجعة:**
  - `ok == true`: إذا كان السياق مقيداً بموعد نهائي (تم إنشاؤه عبر `WithDeadline` أو `WithTimeout`).
  - `ok == false`: إذا لم يكن هناك موعد نهائي محدد (مثل `Background` أو `WithValue`).
- **الاستقرار والتكرار:** استدعاء الدالة بشكل متكرر ومستمر على نفس السياق يعيد دائماً نفس النتيجة بدقة.

#### 2. `Done() <-chan struct{}`

- **الوظيفة:** تعيد قناة (Channel) أحادية الاتجاه للقراءة فقط (`Receive-only`). يتم إغلاق هذه القناة عند إلغاء السياق (سواء بالاستدعاء الصريح لدالة الإلغاء، أو بانتهاء المهلة الزمنية، أو بإلغاء سياق الأب).
- **السلوك مع السياقات الأبدية:** قد تعيد `nil` إذا كان السياق غير قابل للإلغاء إطلاقاً (مثل `context.Background()` أو `context.TODO()` أو السياقات المشتقة عبر `WithoutCancel`). وبما أن القراءة من قناة `nil` داخل `select` تتعطل وتتجاهلها الـ Runtime، فإنها لا تُثار أبداً.
- **التصميم العبقري:** تُغلق القناة (`close(c.done)`) بدلاً من إرسال قيمة بداخلها. والسبب الهندسي في ذلك هو أن إغلاق القناة في Go يُوقظ **جميع** الـ Goroutines المستمعة للقناة دفعة واحدة وبشكل متزامن وبكفاءة $O(1)$ دون الحاجة لمعرفة عددها!

```go
//a الاستخدام النمطي في مراقبة الإلغاء
select {
case <-ctx.Done():
    return ctx.Err()
case data := <-inChan:
    process(data)
}
```

#### 3. `Err() error`

- **الوظيفة:** توضيح سبب إلغاء السياق.
- **القيمة المرجعة:**
  - تعيد `nil` إذا كانت القناة `Done` لم تُغلق بعد.
  - تعيد خطأ غير صفري (`non-nil error`) فور إغلاق القناة:
    1. `context.Canceled`: إذا أُلغي السياق يدوياً عبر استدعاء دالة `cancel()`.
    2. `context.DeadlineExceeded`: إذا أُلغي السياق بسبب انتهاء الموعد النهائي المحدد.
    3. أخطاء مخصصة: إذا أُلغي عبر `WithCancelCause` وتم تمرير سبب محدد.
- **الخاصية الثابتة (Idempotent):** بعد أن تصبح القيمة غير صفرية، تضمن الدالة إعادة نفس الخطأ دائماً في أي استدعاء لاحق.

#### 4. `Value(key any) any`

- **الوظيفة:** البحث عن واسترجاع القيمة المرتبطة بالمفتاح `key` المخزنة في السياق أو أي سياق أب له في الشجرة.
- **القيمة المرجعة:** تعيد الكائن المخزن، أو `nil` إذا لم يكن المفتاح موجوداً في الشجرة بالكامل.
- **طريقة العمل:** تبحث الدالة عمودياً من العقدة الحالية صعوداً نحو الجذر (Root-ward traversal).

---

### ب. الواجهات الداخلية غير المصدّرة (Internal Unexported Interfaces)

#### 1. واجهة `canceler`

```go
type canceler interface {
    cancel(removeFromParent bool, err, cause error)
    Done() <-chan struct{}
}
```

- **الغرض المعماري:** تُمثل العقد الداخلي لأي سياق يمكن إلغاؤه بشكل مباشر.
- **المطبقون:** يطبقها كل من `*cancelCtx` و `*timerCtx`.
- **الفائدة:** تمكن سياق الأب من الاحتفاظ بقائمة أبنائه القابلين للإلغاء في خريطة خاصة (`children map[canceler]struct{}`) وإشعارهم شلالياً فور إلغائه دون الحاجة لمعرفة نوعهم الملموس الدقيق.

#### 2. واجهة `afterFuncer`

```go
type afterFuncer interface {
    AfterFunc(func()) func() bool
}
```

- **الغرض المعماري:** أُضيفت في Go 1.21 للسماح لأي تطبيق مخصص لـ `Context` بتوفير ميكانيكية مخصصة لجدولة دوال الإلغاء الخلفية بكفاءة عالية، مما يتيح تكاملاً سلساً مع أنظمة الجدولة الخارجية.

#### 3. واجهة `stringer`

```go
type stringer interface {
    String() string
}
```

- **الغرض المعماري:** واجهة مطابقة لـ `fmt.Stringer`.
- **السر الهندسي الصادم:** تم تعريف هذه الواجهة محلياً داخل حزمة `context` لتفادي استيراد حزمة `fmt`! استيراد `fmt` يسحب معه جداول الحروف الضخمة وجداول الرموز (Unicode Tables) ومحركات الطباعة المعقدة، مما يزيد من حجم الملف الثنائي (Binary Size) ويبطئ زمن بدء التشغيل (Cold Start). لذلك تُعرف الحزمة الواجهة داخلياً للاستعلام عن أسماء السياقات عند التشخيص.

---

## 🧱 2. الهياكل والأنواع الملموسة (Concrete Structs & Types)

```text
 ┌────────────────────────────────────────────────────────────────────────┐
 │                      خريطة الهياكل الملموسة                            │
 ├────────────────────────────────────────────────────────────────────────┤
 │  emptyCtx            ◄── الجذر الصفري الخامل (Background / TODO)       │
 │  cancelCtx           ◄── محرك الإلغاء الشلالي والتحكم بالأبناء         │
 │  timerCtx            ◄── محرك المهل الزمنية والمؤقت الحقيقي (Timer)     │
 │  valueCtx            ◄── عقدة تخزين الزوج (Key-Value) وتمرير الشجرة     │
 │  withoutCancelCtx    ◄── غلاف عزل الإلغاء والاحتفاظ بالبيانات          │
 │  afterFuncCtx        ◄── مسجل استدعاء المهام الخلفية عند الإلغاء       │
 │  stopCtx             ◄── وسيط إلغاء تسجيل المهام الخلفية من الأب      │
 └────────────────────────────────────────────────────────────────────────┘
```

---

### 1. `emptyCtx`

```go
type emptyCtx struct{}

func (emptyCtx) Deadline() (deadline time.Time, ok bool) { return }
func (emptyCtx) Done() <-chan struct{}                   { return nil }
func (emptyCtx) Err() error                              { return nil }
func (emptyCtx) Value(key any) any                       { return nil }
```

- **الوصف:** هيكل فارغ لا يستهلك أي بايت في الذاكرة (Zero Allocation).
- **السلوك:** دواله الأربع خاملة بالكامل ولا تقوم بأي معالجة.
- **المشتقات:** يُبنى منه نوعان فرعيان يختلفان في دالة `String()` فقط للتمييز عند الطباعة:

  ```go
  type backgroundCtx struct{ emptyCtx }
  func (backgroundCtx) String() string { return "context.Background" }

  type todoCtx struct{ emptyCtx }
  func (todoCtx) String() string { return "context.TODO" }
  ```

---

### 2. `cancelCtx` (محرك الإلغاء الأساسي)

```go
type cancelCtx struct {
    Context

    mu       sync.Mutex            // يحمي الحقول التالية
    done     atomic.Value          // يحمل chan struct{}، يُنشأ بكسل ويُغلق بأول إلغاء
    children map[canceler]struct{} // يُضبط إلى nil بعد أول استدعاء للإلغاء
    err      atomic.Value          // يُسجل فيه الخطأ بعد أول إلغاء
    cause    error                 // يُسجل سبب الإلغاء المحدد
}
```

#### تحليل الحقول المعمارية

1. `Context`: السياق الأب المضمّن (`Embedded Parent Context`).
2. `mu sync.Mutex`: قفل مزامنة دقيق لحماية حالة الأبناء وقنوات الإلغاء وتفادي سباقات البيانات (`Data Races`).
3. `done atomic.Value`: استخدام العبقرية البرمجية بالتخصيص الكسول (`Lazy Allocation`). لا يتم إنشاء القناة إلا عند استدعاء `ctx.Done()`. إذا لم يستدعِ الكود القناة مطلقاً، فلن يتم استهلاك أي تخصيص لقنوات Go! كما أن القراءة الذرية عبر `atomic.Value` تمنح سرعة فائقة تفوق الأقفال العادية بخمسة أضعاف.
4. `children map[canceler]struct{}`: جدول تجزئة يحتفظ بجميع السياقات الفرعية المشتقة من هذا السياق. عند الإلغاء، يمر هذا السياق على جميع الأبناء ويطلب إلغاءهم شلالياً، ثم يفرغ الخريطة إلى `nil` لتحرير الذاكرة لجامع القمامة.
5. `err atomic.Value`: يحمل كائن الخطأ عند الإلغاء (`Canceled`).
6. `cause error`: يحمل سبب الإلغاء المفصل في حال استخدام `WithCancelCause`.

---

### 3. `timerCtx` (محرك المهل الزمنية)

```go
type timerCtx struct {
    cancelCtx
    timer *time.Timer // محمي بواسطة cancelCtx.mu

    deadline time.Time
}
```

- **الوصف:** يرث كافة قدرات `cancelCtx` عبر التضمين، ويضيف:
  1. `deadline time.Time`: الموعد النهائي المحدد بالأمر.
  2. `timer *time.Timer`: مؤقت المكتبة القياسية الفعلي الذي يستدعي دالة الإلغاء بعد انقضاء المهلة.
- **دورة الحياة:** عند استدعاء `cancel()`، يقوم أولاً بإيقاف المؤقت فوراً عبر `c.timer.Stop()` لتفريغ المؤقت من مجدول لغة Go (`Go Runtime Timer Heap`)، ثم يستدعي دالة الإلغاء الأصلية.

---

### 4. `valueCtx` (عقدة تخزين البيانات)

```go
type valueCtx struct {
    Context
    key, val any
}
```

- **الوصف:** هيكل بسيط جداً يحمل المفتاح `key` والقيمة `val` ومؤشراً نحو سياق الأب `Context`.
- **التصميم المتسلسل:** كل استدعاء لـ `WithValue` ينشئ عقدة واحدة جديدة تشير إلى الأب، لتشكل معاً قائمة مترابطة شجرية (Linked-List).

---

### 5. `withoutCancelCtx` (عازل الإلغاء - Go 1.20+)

```go
type withoutCancelCtx struct {
    c Context
}

func (withoutCancelCtx) Deadline() (deadline time.Time, ok bool) { return }
func (withoutCancelCtx) Done() <-chan struct{}                   { return nil }
func (withoutCancelCtx) Err() error                              { return nil }
func (c withoutCancelCtx) Value(key any) any                     { return value(c, key) }
```

- **الوصف:** يلتف حول سياق أب موجود، ويعيد صياغة السلوك:
  - يعطل الإلغاء تماماً (`Done` ترجع `nil`، و `Err` ترجع `nil`).
  - يعطل المهل الزمنية (`Deadline` ترجع `ok == false`).
  - يبقي استرجاع البيانات مفعلاً بالكامل (`Value` تواصل البحث في شجرة الأب).

---

### 6. `afterFuncCtx` و `stopCtx` (Go 1.21+)

```go
type afterFuncCtx struct {
    cancelCtx
    once sync.Once // يضمن إما تشغيل f أو إيقاف f
    f    func()
}
```

- **الوظيفة:** تسجيل دالة مخصصة `f` ليتم تشغيلها تلقائياً داخل Goroutine مستقلة بمجرد إلغاء السياق.
- **الحماية:** استخدام `sync.Once` يضمن قطيعة رياضية: إما أن تعمل الدالة مرة واحدة فقط، أو أن يتم إيقافها قبل أن تبدأ، مستحيلاً حدوث تشغيل مكرر أو سباق بيانات.

```go
type stopCtx struct {
    Context
    stop func() bool
}
```

- **الوظيفة:** سياق وسيط يلتف حول الأب عند تسجيل `AfterFunc` ليحمل دالة الإيقاف `stop` ويسمح بإلغاء التسجيل بسلاسة وتفريغ الموارد.

---

## ⚠️ 3. متغيرات الأخطاء الرسمية (Sentinel Errors)

### أ. `context.Canceled`

```go
var Canceled = errors.New("context canceled")
```

- الخطأ القياسي المعاد عند إلغاء السياق بالاستدعاء الصريح لدالة `cancel()`.

### ب. `context.DeadlineExceeded`

```go
var DeadlineExceeded error = deadlineExceededError{}

type deadlineExceededError struct{}

func (deadlineExceededError) Error() string   { return "context deadline exceeded" }
func (deadlineExceededError) Timeout() bool   { return true }
func (deadlineExceededError) Temporary() bool { return true }
```

- **الوصف:** خطأ متخصص مطبق لـ 3 واجهات:
  1. واجهة `error` القياسية.
  2. واجهة `net.Error` (التي تفحص `Timeout() == true` و `Temporary() == true`)، مما يسمح لمكتبات الشبكات والـ HTTP بالتعرف التلقائي على أن الخطأ ناجم عن انتهاء المهلة الزمنية والتعامل معه بمرونة.

---

## 🎛️ 4. أنواع دوال الإلغاء (Function Types)

```go
// دالة الإلغاء القياسية: خالية من المعاملات وآمنة للاستدعاء المتكرر
type CancelFunc func()

// دالة الإلغاء المسببة: تستقبل كائن خطأ لتوثيق سبب الإلغاء
type CancelCauseFunc func(cause error)
```

- كلتا الدالتين آمنة خيطياً بنسبة 100%، ويمكن استدعاؤهما من أي Goroutine بشكل متكرر دون خوف (استدعاؤها بعد المرة الأولى لا يُحدث أي تأثير - Idempotent).
