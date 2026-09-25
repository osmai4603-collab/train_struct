# 06. تدفق البيانات وكائنات الطلب والاستجابة والترويسات

يمثل كائنا `http.Request` و `http.Response` عمودي التخاطب بين الخادم والعميل في بروتوكول HTTP. في هذا الفصل، سنقوم بتشريح متعمق لكافة الحقول والأساليب والآليات الداخلية لقراءة وتعديل البيانات، وإدارة النماذج (Forms)، والتعامل مع الترويسات الموحدة (Canonical Headers)، وضبط الكوكيز وفق أحدث معايير الأمان، وخوارزمية استشعار نوع المحتوى (MIME Sniffing).

---

## 📥 1. تشريح هيكل الطلب: `http.Request`

يُعرّف الهيكل في ملف [`/usr/local/go/src/net/http/request.go`](file:///usr/local/go/src/net/http/request.go):

```go
type Request struct {
    Method           string
    URL              *url.URL
    Proto            string // "HTTP/1.1"
    ProtoMajor       int    // 1
    ProtoMinor       int    // 1
    Header           Header
    Body             io.ReadCloser
    GetBody          func() (io.ReadCloser, error)
    ContentLength    int64
    TransferEncoding []string
    Close            bool
    Host             string
    Form             url.Values
    PostForm         url.Values
    MultipartForm    *multipart.Form
    Trailer          Header
    RemoteAddr       string
    RequestURI       string
    TLS              *tls.ConnectionState
    Response         *Response
    ctx              context.Context
}
```

### الحقول الأساسية ومجموعاتها الوظيفية

| المجموعة | الحقول | الدور الهندسي والمعماري |
| :--- | :--- | :--- |
| **تعريف المورد** | `Method`, `URL`, `Host`, `RequestURI` | تحدد الفعل المراد تنفيذه ومسار المورد المطلوب. في الخادم، يمثل `URL.Path` المسار المنظف، بينما يحتوي `RequestURI` على السلسلة الخام كما أرسلها العميل. |
| **البروتوكول والترويسات** | `Proto`, `ProtoMajor`, `Header`, `Trailer` | إصدار البروتوكول وخريطة الترويسات والترويسات اللاحقة (Trailers المرسلة في نهاية التدفق المجزأ Chunked). |
| **جسم الطلب وتدفقه** | `Body`, `GetBody`, `ContentLength`, `TransferEncoding` | `Body` هو تدفق متسلسل للقراءة والإغلاق (`io.ReadCloser`). حقل `GetBody` هو دالة استدعاء خلفية تستخدمها طبقة العميل لإعادة توليد تدفق الجسم في حال تطلب الأمر إعادة المحاولة أو متابعة إعادة التوجيه. |
| **النماذج والبيانات المرفوعة** | `Form`, `PostForm`, `MultipartForm` | الخرائط التي تحتوي على المعلمات والبيانات بعد فك تشفيرها عبر دوال التحليل. |
| **البيانات الأمنية والشبكية** | `RemoteAddr`, `TLS`, `Close` | عنوان العميل ورقم المنفذ الوارد (`IP:Port`)، وتفاصيل جلسة التشفير في حال كان الاتصال عبر HTTPS. |

---

### دوال البناء وإدارة السياق (Constructors & Context)

#### 1. دالة البناء القياسية الحديثة
```go
func NewRequestWithContext(ctx context.Context, method, url string, body io.Reader) (*Request, error)
```
تنشئ كائن طلب جديد مرتبط بسياق محدد (`ctx`). إذا كان `body` ينفذ واجهة `io.ReadCloser` يُستخدم كما هو، وإلا يُغلف تلقائياً بـ `io.NopCloser`.

#### 2. النسخ السطحي مقابل النسخ العميق (`WithContext` مقابل `Clone`)
- **`r.WithContext(ctx context.Context) *Request`:** يقوم بإنشاء نسخة سطحية (Shallow Copy) سريعة من هيكل الطلب مع استبدال السياق الداخلي فقط.
- **`r.Clone(ctx context.Context) *Request`:** يقوم بإنشاء **نسخة عميقة بالكامل (Deep Copy)** تكرر كائنات `URL`، والترويسات `Header`، ومتغيرات التوجيه `matches` و `otherValues`. يُستخدم عند الحاجة إلى تعديل ترويسات الطلب دون التأثير على الروتين الأصلي.

---

### استخراج وتدقيق بيانات النماذج (Forms & File Uploads)

```text
                                r.ParseForm()
                                      │
              ┌───────────────────────┴───────────────────────┐
              ▼                                               ▼
         استعلام الرابط (URL Query)                 جسم الطلب (POST Body)
   application/x-www-form-urlencoded         application/x-www-form-urlencoded
              │                                               │
              └───────────────┬───────────────────────────────┘
                              ▼
                        r.Form (يجمعهما معاً)
                              ▲
                              │
                    r.PostForm (الجسم فقط)
```

1. **`r.ParseForm() error`:** تحلل معلمات الرابط (URL Query String) بالإضافة إلى جسم الطلب إذا كان من نوع `application/x-www-form-urlencoded`.
2. **`r.FormValue(key string) string`:** تستدعي `ParseForm` تلقائياً وتعيد القيمة الأولى للمفتاح (تبحث في الجسم أولاً ثم في استعلام الرابط).
3. **`r.PostFormValue(key string) string`:** تعيد القيمة من جسم الطلب حصراً، وتتجاهل معلمات الرابط.
4. **`r.ParseMultipartForm(maxMemory int64) error`:** مخصصة لرفع الملفات والبيانات الثنائية (`multipart/form-data`). تحتفظ بالبيانات في الذاكرة حتى حد `maxMemory` بايت، وما زاد عن ذلك يتم تخزينه تلقائياً في ملفات مؤقتة على القرص الصلب.
5. **`r.FormFile(key string) (multipart.File, *multipart.FileHeader, error)`:** اختصار فوري لاستخراج ملف مرفوع دون الحاجة للتعامل اليدوي مع الهياكل.

---

### حماية الخادم من فيضان الذاكرة: `http.MaxBytesReader`

```go
func MaxBytesReader(w ResponseWriter, r io.ReadCloser, n int64) io.ReadCloser
```

> [!IMPORTANT]
> **درع الأمان ضد هجمات حجب الخدمة (DoS via Memory Flooding):**
> إذا سمح خادمك برفع بيانات في الجسم دون تقييد، يمكن لمهاجم إرسال دفق بيانات بحجم 50GB عبر طلب واحد، مما يؤدي إلى استنزاف الذاكرة وانهيار النظام.
> دالة `MaxBytesReader` تغلف تدفق القراءة؛ فإذا تجاوز العميل حجم `n` بايت، تتوقف القراءة فوراً، وترمي خطأ `*http.MaxBytesError`، وترسل تلقائياً للعميل رمز الحالة `413 Request Entity Too Large` وتغلق الاتصال لمنع استنزاف الموارد!

---

## 📤 2. تشريح هيكل الاستجابة: `http.Response`

يُعرّف الهيكل في ملف [`/usr/local/go/src/net/http/response.go`](file:///usr/local/go/src/net/http/response.go):

```go
type Response struct {
    Status           string // e.g. "200 OK"
    StatusCode       int    // e.g. 200
    Proto            string // e.g. "HTTP/1.1"
    ProtoMajor       int
    ProtoMinor       int
    Header           Header
    Body             io.ReadCloser
    ContentLength    int64
    TransferEncoding []string
    Close            bool
    Uncompressed     bool
    Trailer          Header
    Request          *Request
    TLS              *tls.ConnectionState
}
```

### الأساليب المتاحة على كائن الاستجابة
- **`Cookies() []*Cookie`:** تحلل كافة ترويسات `Set-Cookie` الواردة في الرد وتعيدها كمصفوفة كائنات مهيكلة.
- **`Location() (*url.URL, error)`:** تستخرج الرابط الموجود في ترويسة `Location` وتعالجه نسبياً إلى رابط الطلب الأصلي (`r.Request.URL`).
- **`ProtoAtLeast(major, minor int) bool`:** تفحص ما إذا كان إصدار الرد مساوياً أو أحدث من الإصدار المحدد (مثل فحص دعم HTTP/2).

---

## 🏷️ 3. معمارية الترويسات: `http.Header` و `CanonicalHeaderKey`

### التعريف البرمجي
```go
type Header map[string][]string
```
تُعرّف الترويسات كخريطة مفتاحها نص وقيمتها شريحة نصوص، مما يتيح إسناد قيم متعددة لنفس المفتاح (مثل تكرار ترويسات `Set-Cookie` أو `Accept`).

### أساليب الترويسات
```go
func (h Header) Add(key, value string)   // تضيف قيمة جديدة دون حذف القيم السابقة
func (h Header) Set(key, value string)   // تستبدل كافة القيم السابقة بقيمة واحدة جديدة
func (h Header) Get(key string) string   // تعيد القيمة الأولى المرتبطة بالمفتاح (أو فارغة)
func (h Header) Values(key string) []string // تعيد كافة القيم المرتبطة بالمفتاح
func (h Header) Del(key string)          // تحذف الترويسة وجميع قيمها بالكامل
func (h Header) Clone() Header           // تنشئ نسخة عميقة ومستقلة تماماً من الخريطة
```

### توحيد صيغة المفاتيح: `CanonicalHeaderKey`
بروتوكول HTTP غير حساس لحالة الأحرف في أسماء الترويسات (Case-Insensitive). لضمان سرعة البحث وتجنب تكرار المفاتيح بصيغ مختلفة، تطبق Go خوارزمية التنسيق المعياري (MIME Canonicalization):
```go
http.CanonicalHeaderKey("content-type")  // النتيجة: "Content-Type"
http.CanonicalHeaderKey("x-request-id")  // النتيجة: "X-Request-Id"
```
تقوم دالتا `Set` و `Get` و `Add` تلقائياً بتطبيق `CanonicalHeaderKey` على المفتاح قبل البحث أو الإدخال في الخريطة.

---

## 🍪 4. هيكل الكوكيز ومعايير الحماية الحديثة: `http.Cookie`

يُعرّف الهيكل في ملف [`/usr/local/go/src/net/http/cookie.go`](file:///usr/local/go/src/net/http/cookie.go):

```go
type Cookie struct {
    Name        string
    Value       string
    Path        string
    Domain      string
    Expires     time.Time
    RawExpires  string
    MaxAge      int
    Secure      bool
    HttpOnly    bool
    SameSite    SameSite
    Raw         string
    Unparsed    []string
    Partitioned bool // Go 1.23+ لدعم معيار CHIPS في المتصفحات الحديثة
}
```

### خيارات الأمان الصارمة للكوكيز الإنتاجية

| الحقل | الوظيفة الهندسية والأمنية |
| :--- | :--- |
| **`HttpOnly`** | يمنع وصول برمجيات JavaScript (`document.cookie`) لقيمة الكوكي نهائياً، مما يقضي على هجمات سرقة الجلسات عبر XSS. |
| **`Secure`** | يمنع المتصفح من إرسال الكوكي إلا عبر الاتصالات المشفرة فقط (`HTTPS`). |
| **`SameSite`** | يتحكم في إرسال الكوكي مع الطلبات العابرة للمواقع (CSRF Protection):<br>• `SameSiteStrictMode`: لا يُرسل مطلقاً مع أي رابط خارجي.<br>• `SameSiteLaxMode`: يُرسل مع روابط التنقل العادية ويمنع في الطلبات المضمنة.<br>• `SameSiteNoneMode`: يُرسل دائماً (يتطلب تفعيل `Secure: true`). |
| **`Partitioned`** | ميزة حديثة تدعم مبادرة Privacy Sandbox ومعيار CHIPS، حيث يتم عزل الكوكي بحسب نطاق الموقع الأعلى (Top-Level Site Partition). |

### دوال التعامل مع الكوكيز
- `http.SetCookie(w ResponseWriter, cookie *Cookie)`: تضيف ترويسة `Set-Cookie` منسقة بدقة إلى رد الخادم.
- `(r *Request) Cookie(name string) (*Cookie, error)`: تبحث عن كوكي محدد بالاسم في ترويسات الطلب الوارد.
- `(r *Request) Cookies() []*Cookie`: تعيد تحليلاً لكافة الكوكيز المرفقة بالطلب.

---

## 🕵️ 5. استشعار نوع المحتوى الذاتي (MIME Sniffing): `DetectContentType`

```go
func DetectContentType(data []byte) string
```

### كيف تعمل الخوارزمية داخلياً؟
يُعرّف التنفيذ في ملف [`/usr/local/go/src/net/http/sniff.go`](file:///usr/local/go/src/net/http/sniff.go).
إذا قام المطور بكتابة بيانات عبر `w.Write(data)` دون تحديد ترويسة `Content-Type` مسبقاً، لا تقوم Go بإرسال نوع عشوائي، بل تستدعي خوارزمية فحص قياسية (وفق معيار WHATWG MIME Sniffing):

1. تأخذ الخوارزمية **أول 512 بايت فقط** من تدفق البيانات (`const sniffLen = 512`).
2. تفحص التواقيع الثنائية (Magic Bytes والوسوم):
   - وسوم HTML مثل `<!DOCTYPE html` أو `<html` تعيد `text/html; charset=utf-8`.
   - وسوم XML تعيد `text/xml; charset=utf-8`.
   - التواقيع الثنائية لصور PNG و JPEG و GIF و WebP.
   - ملفات PDF (`%PDF-`) والملفات المضغوطة ZIP.
   - الوسائط الصوتية والمرئية.
3. إذا احتوت البيانات على بايتات خالية أو ثنائية غير نصية دون تطابق محدد، تعيد القيمة الآمنة الافتراضية: `application/octet-stream`.
