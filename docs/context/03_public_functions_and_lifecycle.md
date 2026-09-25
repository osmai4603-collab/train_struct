# 03. الدوال العامة وإدارة دورة حياة السياق في حزمة `context`

تُشكل الدوال العامة في حزمة `context` الواجهة البرمجية (Public API) التي يتعامل معها مطورو Go لبناء أشجار السياق، والتحكم في إشارات الإلغاء، وإدارة المهل الزمنية، وتخزين واسترجاع البيانات المقتصرة على الطلب.

---

## 🧭 1. دوال الجذور (Root Contexts)

تُمثل دوال الجذور نقطة الانطلاق الأساسية لأي شجرة سياق؛ حيث يُحظر تماماً تمرير `nil` في أي دالة تطلب `Context`.

### 1. `context.Background() Context`

```go
func Background() Context
```

- **الوصف الهندسي:** تعيد كائناً غير صفري خالي المحتوى من نوع `backgroundCtx`. هذا السياق لا يُلغى أبداً، ولا يحمل أي مهلة زمنية، ولا يحمل أي بيانات.
- **متى يُستخدم؟**
  1. في الدالة الرئيسية للتطبيق (`func main()`).
  2. في دوال التهيئة والتشغيل الأولي (`init()`).
  3. كجذر أعلى لمعالجة الطلبات الواردة في الخوادم (`HTTP Server`, `gRPC Server`, `Background Workers`).
  4. في بيئات الاختبارات التي تبدأ من الصفر (ما لم يُستخدم `t.Context()`).

```go
func main() {
    ctx := context.Background()
    if err := run(ctx); err != nil {
        log.Fatal(err)
    }
}
```

---

### 2. `context.TODO() Context`

```go
func TODO() Context
```

- **الوصف الهندسي:** تشترك دقيقاً مع `Background()` في السلوك الباطني (ترجع كائناً من نوع `todoCtx` يرث نفس `emptyCtx`)، ولكنها تختلف فلسفياً ودلالياً.
- **متى يُستخدم؟**
  - عندما تكون بصدد كتابة كود لم يكتمل بعد، أو عندما ترغب في توسيع دالة قديمة لتستقبل `Context` مستقبلاً ولكنك غير متأكد في الوقت الحالي من أي سياق يجب تمريره.
  - تُستخدم كإشارة واضحة لمحللات الكود الثابتة (`Linters`) ولزملائك في الفريق بأن هذا الموضع يحتاج إلى مراجعة وتمرير سياق حقيقي.

---

## 🛑 2. دوال الإلغاء اليدوي وتوثيق الأسباب (Cancellation Functions)

### 3. `context.WithCancel(parent Context) (ctx Context, cancel CancelFunc)`

```go
func WithCancel(parent Context) (ctx Context, cancel CancelFunc)
```

- **السلوك الداخلي:**
  1. تتحقق أولاً أن `parent != nil` (إذا كان `nil` تحدث حالة `panic("cannot create context from nil parent")`).
  2. تنشئ كائناً جديداً من نوع `cancelCtx`.
  3. تربط الابن بالأب عبر دالة `propagateCancel`: إذا كان الأب قابلاً للإلغاء، يُسجل الابن في خريطة أبناء الأب (`parent.children[child]`).
  4. تعيد السياق الجديد مع دالة `CancelFunc`.
- **حتمية استدعاء `cancel()`:**
  > [!IMPORTANT]
  > استدعاء دالة `cancel()` أمر إلزامي وحتمي عبر `defer cancel()` حتى لو انتهت الدالة بنجاح! عدم استدعائها يُبقي مؤشر الأب إلى الابن عالقاً في الذاكرة حتى يُلغى الأب بالكامل، مما يسبب تسريباً بطيئاً للذاكرة (Memory Leak). ترصد أداة `go vet` هذا الخطأ تلقائياً.

```go
func ProcessItems(ctx context.Context, items []Item) error {
    ctx, cancel := context.WithCancel(ctx)
    defer cancel() // يضمن تحرير الموارد فور خروج الدالة

    for _, item := range items {
        if err := saveItem(ctx, item); err != nil {
            return err // سيتم إلغاء العمليات المتبقية تلقائياً بواسطة defer
        }
    }
    return nil
}
```

---

### 4. `context.WithCancelCause` و `context.Cause` (Go 1.20+)

```go
func WithCancelCause(parent Context) (ctx Context, cancel CancelCauseFunc)
func Cause(c Context) error
```

- **المشكلة التي حلتها:** في الإصدارات القديمة، عند إلغاء السياق بـ `cancel()`، كانت `ctx.Err()` تعيد دائماً الخطأ العام والمبهم `context.Canceled`، دون أي وسيلة لمعرفة السبب الحقيقي للإلغاء (هل فشل الدفع؟ هل المستخدم أغلق الجلسة؟ هل هناك خطأ في التحقق؟).
- **السلوك:**
  - `CancelCauseFunc(cause error)`: تقبل كائن خطأ تفصيلي يُسجل في حقل `cause` داخل `cancelCtx`.
  - `Cause(ctx)`: تستخرج السبب الدقيق الذي أدى لإلغاء هذا السياق أو أول سياق أب أُلغي في السلسلة.
  - إذا تم استدعاء `cancel(nil)`، يُعتبر السبب افتراضياً هو `context.Canceled`.

```go
var ErrUserLoggedOut = errors.New("user explicitly logged out")

func handleUserSession(parent context.Context) {
    ctx, cancel := context.WithCancelCause(parent)

    go func() {
        // حدث أمني أدى لتسجيل الخروج
        cancel(ErrUserLoggedOut)
    }()

    <-ctx.Done()
    fmt.Println(ctx.Err())       // Output: context canceled (الخطأ العام القياسي)
    fmt.Println(context.Cause(ctx)) // Output: user explicitly logged out (السبب التشخيصي الدقيق!)
}
```

---

## ⏳ 3. دوال المواعيد النهائية والمهل الزمنية (Deadlines & Timeouts)

### 5. `context.WithDeadline(parent Context, d time.Time) (Context, CancelFunc)`

```go
func WithDeadline(parent Context, d time.Time) (Context, CancelFunc)
```

- **السلوك الداخلي:**
  1. تتحقق مما إذا كان سياق الأب يملك موعداً نهائياً بالفعل عبر `parent.Deadline()`.
  2. **قاعدة الموعد الأقرب:** إذا كان موعد الأب أقرب من الموعد الجديد `d`، تتجاهل الدالة الموعد الجديد وتعيد `WithCancel(parent)`، لأن الأب سيُلغى قبل وصول هذا الموعد!
  3. تنشئ كائن `timerCtx` يحدد مهلة عبر `time.AfterFunc`.
  4. إذا كان الموعد قد انقضى بالفعل (`time.Until(d) <= 0`)، تُلغى العملية فوراً بالخطأ `DeadlineExceeded`.

---

### 6. `context.WithTimeout(parent Context, timeout time.Duration) (Context, CancelFunc)`

```go
func WithTimeout(parent Context, timeout time.Duration) (Context, CancelFunc)
```

- **السلوك الداخلي:** مجرد اختصار برمجي أنيق (Syntactic Sugar) يستدعي `WithDeadline` مباشرة:

  ```go
  func WithTimeout(parent Context, timeout time.Duration) (Context, CancelFunc) {
      return WithDeadline(parent, time.Now().Add(timeout))
  }
  ```

#### مثال إنتاجي لحماية استدعاءات الشبكة وقواعد البيانات

```go
func FetchRemoteData(ctx context.Context, url string) ([]byte, error) {
    // تحديد مهلة قاطعة بـ 500 ملي ثانية
    ctx, cancel := context.WithTimeout(ctx, 500*time.Millisecond)
    defer cancel() // يحرر المؤقت الزمني فور انتهاء القراءة

    req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
    if err != nil {
        return nil, err
    }

    resp, err := http.DefaultClient.Do(req)
    if err != nil {
        if errors.Is(ctx.Err(), context.DeadlineExceeded) {
            return nil, fmt.Errorf("remote call timed out: %w", err)
        }
        return nil, err
    }
    defer resp.Body.Close()

    return io.ReadAll(resp.Body)
}
```

---

### 7. `WithDeadlineCause` و `WithTimeoutCause` (Go 1.21+)

```go
func WithDeadlineCause(parent Context, d time.Time, cause error) (Context, CancelFunc)
func WithTimeoutCause(parent Context, timeout time.Duration, cause error) (Context, CancelFunc)
```

- تمكنك من تسجيل خطأ مسبب يوضح سبب فرض هذه المهلة بالذات عند انتهائها.

---

## 📦 4. دوال تمرير البيانات (Values)

### 8. `context.WithValue(parent Context, key, val any) Context`

```go
func WithValue(parent Context, key, val any) Context
```

- **السلوك والتحققات الصارمة:**
  1. `parent == nil` ──► يطلق `panic("cannot create context from nil parent")`.
  2. `key == nil` ──► يطلق `panic("nil key")`.
  3. `!reflectlite.TypeOf(key).Comparable()` ──► يطلق `panic("key is not comparable")`. (المفتاح يجب أن يدعم المقارنة `==`؛ لذلك تُمنع الشرائح `slices` والخرائط `maps` والدوال كـ keys).
- **النمط الصحيح للمفاتيح:**
  تجنب استخدام أنواع النصوص العامة (`string`) مثل `"user_id"` لأن حزم الطرف الثالث قد تستخدم نفس النص فتتصادم البيانات وتُستبدل. بدلاً من ذلك، عرّف نوعاً غير مصدّر في حزمتك:

```go
package session

type contextKey struct{}

var userKey = contextKey{}

func WithUser(ctx context.Context, u *User) context.Context {
    return context.WithValue(ctx, userKey, u)
}

func FromContext(ctx context.Context) (*User, bool) {
    u, ok := ctx.Value(userKey).(*User)
    return u, ok
}
```

---

## ⚡ 5. الدوال المتقدمة الحديثة (Go 1.20 & Go 1.21)

### 9. `context.WithoutCancel(parent Context) Context` (Go 1.20+)

```go
func WithoutCancel(parent Context) Context
```

- **الفلسفة:** فصل دورة حياة سياق فرعي عن إشارات الإلغاء للأب، مع **الاحتفاظ بكافة البيانات المخزنة** (`Values`).
- **المخرجات:**
  - `ctx.Done() == nil`
  - `ctx.Deadline() == false`
  - `ctx.Err() == nil`
  - `ctx.Value(key)` ──► يبحث بنجاح في كافة بيانات الأب.

#### متى تُستخدم في الإنتاج؟

في خوادم الويب، إذا أردت إطلاق مهمة غير متزامنة لتسجيل تدقيق أمني في قاعدة البيانات بعد انتهاء الرد:

```go
func HandleOrder(w http.ResponseWriter, r *http.Request) {
    // معالجة الطلب...
    w.WriteHeader(http.StatusOK)

    // فصل السياق لتشغيل مهمة خلفية دون أن يقتلها خادم HTTP بعد انتهاء الطلب
    detachedCtx := context.WithoutCancel(r.Context())
    go func() {
        // تقييد المهمة الخلفية بمهلة مستقلة خاصة بها
        bgCtx, cancel := context.WithTimeout(detachedCtx, 5*time.Second)
        defer cancel()

        // يحتفظ بالمعلومات التشخيصية (RequestID, TenantID) من الطلب الأصلي
        auditService.RecordOrder(bgCtx, orderID)
    }()
}
```

---

### 10. `context.AfterFunc(ctx Context, f func()) (stop func() bool)` (Go 1.21+)

```go
func AfterFunc(ctx Context, f func()) (stop func() bool)
```

- **الفلسفة والمعمارية:**
  ترتيب استدعاء الدالة `f` داخل Goroutine منفصلة تلقائياً بمجرد إلغاء السياق `ctx` (أو استدعاؤها فوراً إذا كان السياق مُلغى بالفعل عند التسجيل).
- **دالة الإيقاف `stop() bool`:**
  - تعيد `true` إذا نجح إيقاف الدالة قبل أن تبدأ في العمل.
  - تعيد `false` إذا كانت الدالة قد بدأت بالفعل أو تم إيقافها مسبقاً.
- **البديل المتفوق لـ `select` في العمليات المعقدة:**
  قبل `AfterFunc`، كان المطورون يضطرون لإطلاق Goroutine مخصصة فقط للانتظار على `<-ctx.Done()` لمزامنة العمليات. دالة `AfterFunc` تختصر استهلاك الذاكرة وتتعامل مباشرة مع نظام أحداث السياق الداخلي.

#### مثال لربط إلغاء السياق بإغلاق مقبس شبكي (Socket Cancellation)

```go
func ReadFromConnWithContext(ctx context.Context, conn net.Conn) ([]byte, error) {
    // إغلاق الاتصال الشبكي فور إلغاء السياق لفك تعليق عملية القراءة فوراً
    stop := context.AfterFunc(ctx, func() {
        conn.Close()
    })
    defer stop() // إذا انتهت القراءة بنجاح، نلغي الربط لتفادي إغلاق الاتصال لاحقاً

    buf := make([]byte, 1024)
    n, err := conn.Read(buf)
    return buf[:n], err
}
```
