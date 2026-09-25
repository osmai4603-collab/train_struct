# 07. الميزات المتقدمة والأمان والأداء الداخلي في `net/http`

تتميز حزمة `net/http` بأنها خادم وعميل إنتاجي لا يحتاج بالضرورة إلى حزم وسيطة خارجية لحمايته أو تسريع أدائه. يغطي هذا الفصل أحدث الإضافات الأمنية المدمجة لحماية النظم من الهجمات العابرة للمواقع (CSRF)، وخوادم تقديم الملفات الثابتة فائقة السرعة، والمعالجات المساعدة القياسية، وكيفية استغلال تجمعات الذاكرة المؤقتة (`sync.Pool`)، مع مصفوفة الأنماط المضادة الشائعة في بيئات الإنتاج.

---

## 🛡️ 1. نظام الحماية المدمج من CSRF: `CrossOriginProtection` (Go 1.25+ / Go 1.27)

أضافت Go مؤخراً مكوناً أمنياً ثورياً في صلب المكتبة القياسية داخل ملف [`/usr/local/go/src/net/http/csrf.go`](file:///usr/local/go/src/net/http/csrf.go) لحماية خوادم الويب من هجمات تزوير الطلبات العابرة للمواقع (Cross-Site Request Forgery - CSRF) دون الحاجة لمولدات الرموز المميزة القديمة (Synchronizer Tokens):

### كيف يعمل التحقق الأمني الحديث؟
تعتمد الحزمة على ترويسة المتصفحات القياسية الحديثة **`Sec-Fetch-Site`** (المدعومة في كافة المتصفحات منذ عام 2023) بالإضافة لمقارنة نطاق ترويسة **`Origin`** مع ترويسة **`Host`**:

1. **الطرائق الآمنة (Safe Methods):** طلبات `GET` و `HEAD` و `OPTIONS` مسموح بها دائماً دون قيود.
2. **الطرائق المسببة لتعديل البيانات (State-Changing Methods):** في طلبات `POST` و `PUT` و `DELETE` و `PATCH`:
   - إذا كانت قيمة `Sec-Fetch-Site` تساوي `same-origin` أو `none`، يُسمح بالطلب فوراً.
   - إذا كانت قيمتها تشير إلى موقع خارجي، يتم فحص ما إذا كان النطاق مدرجاً في قائمة النطاقات الموثوقة (`Trusted Origins`) أو الأنماط المستثناة (`Bypass Patterns`).
   - إذا فشل التحقق، يتم إجهاض الطلب فوراً برمز `403 Forbidden`.

```text
                                طلب وارد (HTTP Request)
                                          │
                         هل الطريقة GET / HEAD / OPTIONS؟
                                    ┌─────┴─────┐
                             [نعم]  │           │ [لا: POST/PUT/DELETE]
                                    ▼           ▼
                               قبول فوري    فحص Sec-Fetch-Site و Origin
                                                │
                                ┌───────────────┴───────────────┐
                          [مطابق لنفس النطاق]             [عابر للمواقع Cross-Site]
                                │                               │
                                ▼                               ▼
                            قبول فوري              هل النطاق موثوق أو مستثنى؟
                                                                ┌─────┴─────┐
                                                         [نعم]  │           │ [لا]
                                                                ▼           ▼
                                                             قبول     رفض 403 Forbidden
```

### استخدام `CrossOriginProtection` كوسيط (Middleware)
```go
func main() {
    mux := http.NewServeMux()
    mux.HandleFunc("POST /transfer", handleTransfer)

    // إنشاء نظام الحماية العابرة للمواقع
    csrfGuard := http.NewCrossOriginProtection()

    // السماح بنطاقات موثوقة محددة (مثل تطبيقات الجوال أو الواجهات المنفصلة)
    csrfGuard.AddTrustedOrigin("https://app.trusted-partner.com")

    // استثناء مسار معين (مثل Webhook خارجي من بوابة Stripe)
    csrfGuard.AddInsecureBypassPattern("POST /webhooks/stripe")

    // تفعيل الحماية على كامل الموجه
    protectedHandler := csrfGuard.Handler(mux)

    http.ListenAndServe(":8080", protectedHandler)
}
```

---

## 📁 2. معمارية خادم الملفات الثابتة (`fs.go`)

يحتوي ملف [`/usr/local/go/src/net/http/fs.go`](file:///usr/local/go/src/net/http/fs.go) على واحدة من أذكى خوارزميات تقديم الملفات، والتي تدعم بروتوكول HTTP بكامل تعقيداته تلقائياً:

### الدوال الأساسية لتقديم الملفات
- **`http.FileServer(root FileSystem) Handler`:** ينشئ معالجاً يقدم الملفات من مسار القرص الصلب.
- **`http.FileServerFS(root fs.FS) Handler`:** يقدم الملفات من نظام ملفات تجريدي (مثل ملفات الواجهة المدمجة داخل البرنامج عبر `//go:embed`).
- **`http.ServeFile(w ResponseWriter, r *Request, name string)`:** تقدم ملفاً واحداً محدداً استجابة لطلب.
- **`http.ServeContent(w ResponseWriter, req *Request, name string, modtime time.Time, content io.ReadSeeker)`:** المحرك الأساسي الذي يقدم دفقاً قابلاً للبحث (`io.ReadSeeker`).

### القدرات المدمجة تلقائياً في خادم الملفات
1. **الاستئناف والتحميل الجزئي (HTTP 206 Partial Content):** يتعامل الخادم تلقائياً مع ترويسة `Range: bytes=100-2000`، مما يتيح تشغيل الفيديو والتقديم والتأخير فيه بسلاسة ودعم برامج التحميل المجزأ.
2. **التخزين المؤقت الذكي (Caching Validation):**
   - يدعم ترويسات التحقق المشروط: `If-Modified-Since` و `If-Unmodified-Since`.
   - إذا لم يتغير الملف، يعيد الخادم فوراً رمز `304 Not Modified` بجسم فارغ لتوفير استهلاك الشبكة تماماً.
3. **الحماية التلقائية من Directory Traversal:** تطهر Go المسارات تلقائياً وترفض أي محاولات لاستخدام `../` للخروج خارج المجلد المصرح به.
4. **دعم `StripPrefix`:** لتشغيل خادم الملفات تحت مسار فرعي:
```go
// تقديم محتويات المجلد المحلي ./static تحت المسار /assets/
fs := http.FileServer(http.Dir("./static"))
http.Handle("/assets/", http.StripPrefix("/assets/", fs))
```

---

## 🧰 3. المعالجات المساعدة القياسية (Standard Utility Handlers)

توفر الحزمة معالجات جاهزة ومغلفة لتنفيذ أنماط معمارية شائعة:

### 1. معالج المهل الزمنية: `http.TimeoutHandler`
```go
func TimeoutHandler(h Handler, dt time.Duration, msg string) Handler
```
- **الوظيفة:** يغلف أي معالج بمهلة زمنية صارمة (`dt`).
- **كيف يعمل؟** يشغل المعالج في Goroutine منفصلة، مع مخزن مؤقت لكتابة الرد. فإذا استغرق المعالج وقتاً أطول من `dt`، يقاطعه الخادم ويرسل للعميل فوراً رمز `503 Service Unavailable` مع الرسالة المحددة، مع حماية الخادم من التسريب.

### 2. معالج الحجم الأقصى للطلب: `http.MaxBytesHandler`
```go
func MaxBytesHandler(h Handler, n int64) Handler
```
- يغلف جسم كل طلب وارد بـ `MaxBytesReader` لضمان ألا يتجاوز الحجم الإجمالي للجسم المرفوع `n` بايت، مع إعادة رمز `413 Request Entity Too Large` عند التجاوز.

### 3. معالجات إعادة التوجيه والصفحات غير الموجودة
- **`http.RedirectHandler(url string, code int) Handler`:** معالج جاهز لإعادة توجيه كافة الطلبات الواردة إلى رابط محدد (مثل تحويل HTTP إلى HTTPS برمز 301).
- **`http.NotFoundHandler() Handler`:** معالج قياسي يعيد صفحة الخطأ `404 page not found`.
- **`http.AllowQuerySemicolons(h Handler) Handler`:** يحمي من ثغرات تهريب الاستعلامات (Query Smuggling) عبر تطهير الفواصل المنقوطة `;` في روابط الاستعلام.

---

## ⚡ 4. التشريح الداخلي للأداء الفائق وتجمعات الذاكرة (`sync.Pool`)

لتحقيق سرعات معالجة استثنائية تصل إلى مئات الآلاف من الطلبات في الثانية دون إثقال جامع القمامة (Garbage Collector)، تعتمد حزمة `net/http` داخلياً على نمط إعادة استخدام الكائنات عبر `sync.Pool`:

```go
var (
    bufioReaderPool   sync.Pool
    bufioWriterPool   sync.Pool
    copyBufPool       sync.Pool
)
```

1. **إعادة تدوير المخازن المؤقتة (`bufio.Reader` و `bufio.Writer`):**
   - بدلاً من تخصيص مخزن مؤقت بسعة 4KB لكل اتصال عند قراءة الترويسات وكتابة الردود، تسترجع الحزمة مخزناً جاهزاً من الـ Pool.
   - عند اكتمال الاتصال أو إغلاقه، يُعاد المخزن إلى التجمع لإعادة استخدامه في الاتصال القادم (Zero-Allocation Hot Path).
2. **تخزين نصوص الأعداد وحالات HTTP:**
   - الحزمة تحتفظ بمصفوفة ثابتة من نصوص حالات الـ HTTP (`StatusText`) لمنع أي تخصيصات نصية متكررة أثناء توليد أسطر الردود.

---

## ⚠️ 5. مصفوفة الأنماط المضادة في بيئات الإنتاج (Production Anti-Patterns Matrix)

| # | النمط المضاد (Anti-Pattern) | الكارثة المترتبة عليه | الحل الهندسي الموصى به |
| :---: | :--- | :--- | :--- |
| **1** | إهمال استدعاء `resp.Body.Close()` في العميل | تسريب مقابس TCP، بقاء الـ Goroutines معلقة، ونفاد واصفات الملفات في النظام (Socket Exhaustion). | استدعاء `defer resp.Body.Close()` فوراً بعد التحقق من خلو الطلب من الخطأ (`if err != nil`). |
| **2** | استخدام `http.Get(...)` أو `http.DefaultClient` في الإنتاج | الكائن الافتراضي لا يملك مهلة زمنية (`Timeout: 0`)؛ أي بطء خارجي يوقف خادمك كلياً. | إنشاء `&http.Client{Timeout: 10 * time.Second}` بميزانيات زمنية دقيقة دائماً. |
| **3** | تشغيل الخادم عبر `http.ListenAndServe(":8080", mux)` دون مهل | الخادم عرضة لهجمات Slowloris DoS التافهة التي تشغل اتصالاته وتوقفه عن العمل. | استخدام هيكل `&http.Server{}` مع ضبط إلزامي لـ `ReadHeaderTimeout` و `WriteTimeout`. |
| **4** | كتابة الترويسات بعد `w.WriteHeader(...)` أو `w.Write(...)` | تجاهل الترويسات المضافة بصمت وعدم إرسالها للعميل لأن الترويسات ترسل أولاً. | استدعاء `w.Header().Set(...)` **قبل** أي عملية كتابة للرد أو تحديد للحالة. |
| **5** | إغلاق `resp.Body` دون قراءة محتواه حتى الـ EOF | عجز مجمع النقل (`Transport`) عن إعادة استخدام الاتصال في مجمع Keep-Alive واضطراره لقتله. | تفريغ الجسم بسرعة عبر `io.Copy(io.Discard, resp.Body)` قبل الإغلاق. |
| **6** | تعديل كائن `*http.Request` مباشرة داخل الـ Middlewares | حدوث سباق بيانات (Data Race) عند تفرع الروتينات أو تشغيل المعالجات بالتوازي. | استخدام `r.Clone(ctx)` أو `r.WithContext(ctx)` لإنشاء نسخة نظيفة ومحمية. |
