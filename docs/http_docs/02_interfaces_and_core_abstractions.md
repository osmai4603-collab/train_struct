# 02. الواجهات والتجريدات الأساسية في حزمة `net/http`

تعتمد حزمة `net/http` على فلسفة لغة Go الجوهرية: **"الواجهات الصغيرة القوية تصنع برمجيات قابلة للتركيب والتوسيع"**. بفضل هذه الواجهات، يمكنك استبدال خادم الويب، أو كتابة طبقات وسيطة (Middlewares)، أو محاكاة الاتصالات في الاختبارات، دون أي تعديل على منطق العمل الأساسي.

يستعرض هذا الملف كافة الواجهات العامة والأنواع التجريدية الملحقة بها، مع توضيح أدوارها وعلاقاتها الهندسية.

---

## 🧭 خريطة الواجهات والتجريدات في الحزمة

```text
                               ┌───────────────────────────┐
                               │     net/http Interfaces   │
                               └─────────────┬─────────────┘
                                             │
      ┌─────────────────────┬────────────────┼────────────────────┬────────────────────┐
      ▼                     ▼                ▼                    ▼                    ▼
[معالجة الطلبات]     [كتابة الردود]   [تنفيذ طلبات العميل]  [قدرات الخادم المتقدمة] [نظام الملفات والكوكيز]
 • Handler             • ResponseWriter • RoundTripper        • Flusher            • FileSystem & File
 • HandlerFunc (نوع)                                          • Hijacker           • CookieJar
                                                              • Pusher
                                                              • CloseNotifier (مهمل)
                                                              • ResponseController (Go 1.20+)
```

---

## 1. واجهة معالجة الطلبات: `http.Handler`

### التعريف البرمجي

```go
type Handler interface {
    ServeHTTP(ResponseWriter, *Request)
}
```

### الأهمية المعمارية

واجهة `Handler` هي المحرك المركزي لكل استجابة في خادم Go. أي كائن ينفذ دالة `ServeHTTP` يمكن تمريره كمستقبل للطلبات إلى خادم HTTP، أو تسجيله داخل موجه الطلبات (`http.ServeMux`).

### الميزات التصميمية

- **البساطة المتناهية:** معاملان فقط: أحدهما للكتابة والآخر للقراءة.
- **التوافق التام مع نمط الوسائط (Middleware Pattern):** يمكن لأي معالج أن يغلف معالجاً آخر وينفذ قبله أو بعده منطقاً معيناً (مثل تسجيل السجلات، التحقق من الصلاحيات، القياسات المترية):

```go
func LoggingMiddleware(next http.Handler) http.Handler {
    return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
        start := time.Now()
        next.ServeHTTP(w, r) // تمرير التنفيذ للمعالج التالي
        log.Printf("%s %s completed in %v", r.Method, r.URL.Path, time.Since(start))
    })
}
```

---

## 2. مهايئ الدوال: `http.HandlerFunc`

### التعريف البرمجي

```go
type HandlerFunc func(ResponseWriter, *Request)

func (f HandlerFunc) ServeHTTP(w ResponseWriter, r *Request) {
    f(w, r)
}
```

### الأهمية المعمارية

`HandlerFunc` هو نوع دالة (Function Type) ومهايئ تكييفي (Adapter). يسمح هذا النمط الرائع بتحويل أي دالة عادية مطابقة للتوقيع `func(ResponseWriter, *Request)` إلى كائن يحقق واجهة `http.Handler` تلقائياً دون الحاجة لتعريف `struct` جديد.

### مثال الاستخدام

```go
func homeHandler(w http.ResponseWriter, r *http.Request) {
    w.Write([]byte("مرحباً بك في الصفحة الرئيسية"))
}

// تحويل الدالة العادية مباشرة إلى Handler
var h http.Handler = http.HandlerFunc(homeHandler)
```

---

## 3. واجهة كتابة الردود: `http.ResponseWriter`

### التعريف البرمجي

```go
type ResponseWriter interface {
    Header() Header
    Write([]byte) (int, error)
    WriteHeader(statusCode int)
}
```

### تفصيل الدوال وقواعد الحالة (State Machine Rules)

| الدالة | التوقيع | الوظيفة وقواعد التنفيذ الصارمة |
| :--- | :--- | :--- |
| **`Header`** | `Header() Header` | تعيد خريطة الترويسات التي سيتم إرسالها للعميل. **يجب تعديل الترويسات قبل استدعاء `WriteHeader` أو `Write`.** |
| **`WriteHeader`** | `WriteHeader(statusCode int)` | ترسل سطر حالة HTTP (مثل 200 أو 404 أو 500) مع كافة الترويسات للعميل. **لا يمكن استدعاؤها إلا مرة واحدة فقط لكل طلب.** إذا استدعيت مرة أخرى، تسجل الحزمة تحذيراً وتتجاهل الاستدعاء. |
| **`Write`** | `Write([]byte) (int, error)` | تكتب كتلة من جسم الرد للعميل. إذا لم يتم استدعاء `WriteHeader` صراحة من قبل، ستقوم `Write` تلقائياً باستدعاء `WriteHeader(http.StatusOK)` واستشعار نوع المحتوى (`Content-Type`) تلقائياً. |

> [!WARNING]
> **قاعدة الترتيب الزمني لكتابة الرد:**
> أي محاولة لإضافة أو تعديل ترويسة عبر `w.Header().Set(...)` **بعد** استدعاء `w.WriteHeader(...)` أو بعد أول استدعاء لـ `w.Write(...)` ستكون عديمة الجدوى ولن تُرسل إلى العميل، لأن ترويسات HTTP يجب إرسالها عبر المقبس الشبكي أولاً قبل تدفق جسم الرد!

---

## 4. واجهة تنفيذ طلبات العميل: `http.RoundTripper`

### التعريف البرمجي

```go
type RoundTripper interface {
    RoundTrip(*Request) (*Response, error)
}
```

### الأهمية المعمارية

`RoundTripper` هي الواجهة التجريدية التي تمثل "رحلة الذهاب والإياب" لطلب HTTP واحد من منظور العميل:

- تأخذ كائن `*Request` وترسله عبر الشبكة.
- تعيد كائن `*Response` الناتج أو خطأ شبكي `error`.

### القواعد الصارمة لعقد الواجهة (Interface Contract)

1. **عدم تعديل الطلب:** يُحظر على تنفيذ `RoundTrip` تعديل حقول كائن `Request` الممرر إليه (باستثناء قراءة واستهلاك جسم الطلب `Body`).
2. **التنفيذ التزامني الآمن (Thread-Safety):** يجب أن يكون التنفيذ آمناً تماماً للاستخدام المتزامن بواسطة مئات الـ Goroutines.
3. **عدم معالجة التوجيهات أو الكوكيز:** لا تتعامل `RoundTripper` مع تفاصيل الطبقات العليا مثل تتبع إعادة التوجيه (Redirects)، أو المصادقة (Authentication)، أو حفظ الكوكيز في الـ Jar؛ هذه المهام تقع على عاتق كائن `http.Client` الذي يلتف حول `RoundTripper`.

### التنفيذ الافتراضي: `http.DefaultTransport`

التنفيذ الملموس الأكثر شهرة لهذه الواجهة في Go هو الهيكل `*http.Transport`، وهو المسؤول عن فتح اتصالات TCP و TLS، وإدارة مجمع الاتصالات الخاملة (Connection Pooling).

---

## 5. واجهات التحكم المتقدم في اتصال الخادم

تزود الحزمة كائن `ResponseWriter` الداخلي بقدرات اختيارية إضافية عبر واجهات متخصصة:

### أ) واجهة التفريغ الفوري: `http.Flusher`

```go
type Flusher interface {
    Flush()
}
```

- **الوظيفة:** تدفع البيانات المكتوبة في المخزن المؤقت (Buffer) فوراً عبر المقبس الشبكي إلى العميل دون انتظار امتلاء المخزن أو اكتمال المعالج.
- **حالات الاستخدام الحيوية:**
  1. تدفق الأحداث من طرف الخادم (Server-Sent Events - SSE).
  2. بث استجابات الذكاء الاصطناعي التوليدي التفاعلية (Streaming LLM Token Outputs).
  3. نقل البيانات الضخمة أو مقاطع الفيديو الحية.

---

### ب) واجهة اختطاف الاتصال: `http.Hijacker`

```go
type Hijacker interface {
    Hijack() (net.Conn, *bufio.ReadWriter, error)
}
```

- **الوظيفة:** تتيح للمعالج السيطرة التامة والمباشرة على مقبس الـ TCP الأساسي (`net.Conn`) وتجريد خادم HTTP القياسي من إدارته.
- **حالات الاستخدام الحيوية:**
  1. ترقية الاتصال إلى بروتوكول الويب سوكت (WebSocket Handshake).
  2. إنشاء أنفاق الاتصال الآمنة (HTTP CONNECT Proxies).
  3. التبديل إلى بروتوكولات مخصصة عبر نفس المنفذ.

---

### ج) واجهة الدفع من طرف الخادم: `http.Pusher` (HTTP/2 Server Push)

```go
type Pusher interface {
    Push(target string, opts *PushOptions) error
}
```

- **الوظيفة:** ترسل موارد إضافية (مثل ملفات CSS أو JavaScript) إلى متصفح العميل قبل أن يطلبها صراحة، مستفيدة من إمكانيات بروتوكول HTTP/2.

---

### د) واجهة الإشعار بالإغلاق: `http.CloseNotifier` (مهملة - Deprecated)

```go
type CloseNotifier interface {
    CloseNotify() <-chan bool
}
```

- **ملاحظة تاريخية:** تم استبدالها كلياً منذ إصدار Go 1.7 لصالح `r.Context().Done()`. لا ينبغي استخدام هذه الواجهة في الشيفرات البرمجية الحديثة مطلقاً.

---

## 6. كائن التحكم الشامل الحديث: `http.ResponseController` (Go 1.20+)

### معضلة التغليف القديمة (The Middleware Wrapper Problem)

في الإصدارات السابقة لـ Go 1.20، للوصول إلى `Flusher` أو `Hijacker`، كان المطور يلجأ للتحقق من النوع (Type Assertion):

```go
flusher, ok := w.(http.Flusher)
if ok {
    flusher.Flush()
}
```

**الكارثة الهندسية:** إذا استخدم تطبيقك طبقة وسيطة (Middleware) تغلف `w` بهيكل مخصص (مثل تسجيل حالة الرد أو قياس الحجم)، فإن عملية الـ Type Assertion تفشل وتنهار الميزة، ما لم تقم بتنفيذ كافة هذه الواجهات داخل كل مغلف وسيط يدويًا!

### الحل الثوري عبر `ResponseController`

قدمت Go 1.20 كائن `http.ResponseController` الذي يبحث شلالياً ويفكك التغليف (Unwrapping) تلقائياً عبر دعم دالة `Unwrap() http.ResponseWriter`:

```go
rc := http.NewResponseController(w)

// 1. التفريغ الفوري بأمان
err := rc.Flush()

// 2. ضبط مهلة قراءة إضافية ديناميكياً
err = rc.SetReadDeadline(time.Now().Add(10 * time.Second))

// 3. ضبط مهلة كتابة إضافية
err = rc.SetWriteDeadline(time.Now().Add(30 * time.Second))

// 4. اختطاف المقبس
netConn, bufrw, err := rc.Hijack()

// 5. تمكين الاتصال المزدوج الكامل في HTTP/1.1 (Full Duplex)
err = rc.EnableFullDuplex()
```

إذا كان الخادم أو الوسيط لا يدعم ميزة معينة، تعيد الدالة الخطأ المعياري الموحد `http.ErrNotSupported`.

---

## 7. واجهات أنظمة الملفات: `http.FileSystem` و `http.File`

### التعريف البرمجي

```go
type FileSystem interface {
    Open(name string) (File, error)
}

type File interface {
    io.Closer
    io.Reader
    io.Seeker
    Readdir(count int) ([]fs.FileInfo, error)
    Stat() (fs.FileInfo, error)
}
```

### الأهمية وتكامل `io/fs` (Go 1.16+)

- نوع `http.Dir` هو تطبيق ملموس لواجهة `FileSystem` يعتمد على القرص الصلب لنظام التشغيل (`type Dir string`).
- للربط بين نظام الملفات المدمج الحديث في Go (`io/fs.FS` المستخدم مع `//go:embed`) ونظام ملفات HTTP، توفر الحزمة الدالة المحولة:

```go
func FS(fsys fs.FS) FileSystem
```

يتيح ذلك تقديم الملفات الثابتة المخزنة داخل الذاكرة الثنائية مباشرة للمتصفحات بأعلى كفاءة ممكنة.

---

## 8. واجهة تخزين الكوكيز: `http.CookieJar`

### التعريف البرمجي

```go
type CookieJar interface {
    SetCookies(u *url.URL, cookies []*Cookie)
    Cookies(u *url.URL) []*Cookie
}
```

### الأهمية المعمارية

تستخدم هذه الواجهة في كائن العميل `http.Client` لإدارة جلسات المستخدم والكوكيز عبر الطلبات المتعاقبة:

- `SetCookies`: تستقبل الكوكيز القادمة في ترويسة `Set-Cookie` من الخادم وتخزنها بناءً على النطاق والمسار وقواعد الأمان.
- `Cookies`: تسترجع الكوكيز المناسبة لإرفاقها في ترويسة `Cookie` للطلب الصادر إلى الرابط `u`.

الحزمة توفر تنفيذاً جاهزاً ومطابقاً لمعايير RFC الحديثة في الحزمة الفرعية: `net/http/cookiejar`.

---

## 📌 ملخص الواجهات

| الواجهة / النوع | الدور الرئيسي | متى تُستخدم؟ |
| :--- | :--- | :--- |
| **`Handler`** | استقبال الطلب وتوليد الرد | في كافة خوادم Go وموجهات الطلبات والوسائط. |
| **`HandlerFunc`** | تحويل الدوال العادية إلى `Handler` | لكتابة معالجات رشيقة دون إنشاء `structs`. |
| **`ResponseWriter`** | وسيط كتابة الردود والترويسات | لبناء الاستجابة المرسلة للعميل. |
| **`RoundTripper`** | تجريد طبقة النقل الشبكي للعميل | لبناء بوابات اتصال مخصصة، واعتراض الطلبات وإعادة المحاولة. |
| **`ResponseController`** | التحكم المتقدم والموحد في استجابة الخادم | للتفريغ (Flush)، والاختطاف (Hijack)، وضبط المهل الحية. |
| **`FileSystem`** | تجريد قراءة الملفات الثابتة | لتقديم الأصول الثابتة (Static Assets) أو المدمجة (Embedded FS). |
| **`CookieJar`** | إدارة وتخزين كوكيز العميل | للحفاظ على جلسات تسجيل الدخول في برامج العميل والأتمتة. |
