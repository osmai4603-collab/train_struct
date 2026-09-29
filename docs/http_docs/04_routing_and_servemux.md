# 04. موجه الطلبات المتقدم ومعمارية `ServeMux` في Go الحديثة

شهد موجه الطلبات القياسي `http.ServeMux` في إصدار **Go 1.22** وما يليه ثورة تصميمية ومعمارية هائلة أعادت تعريف تطوير تطبيقات الويب في Go، حيث أصبحت المكتبة القياسية تغني رسمياً عن الحاجة إلى أطر ومكتبات التوجيه الخارجية (مثل Chi أو Gorilla Mux) في الغالبية العظمى من المشاريع الإنتاجية.

---

## ⏳ التطور التاريخي لـ `ServeMux`

### مرحلة ما قبل Go 1.22 (التوجيه بالبادئة - Simple Prefix Matching)

كان الموجه القديم يقدم مطابقة بدائية جداً تقوم على قاعدتين فقط:

1. **المسار المحدد تماماً:** مثل `/index.html`.
2. **المسار المنتهي بشرطة مائلة (Prefix Subtree):** مثل `/images/` يطابق كل ما يبدأ بهذه البادئة.

**العيوب التي أرهقت المطورين لسنوات:**

- عجز تام عن تصفية الطلبات بحسب طريقة الـ HTTP (`GET`, `POST`, `DELETE`). كان على المطور فحص `if r.Method != http.MethodPost` يدوياً داخل كل معالج.
- عدم دعم المتغيرات في المسارات (Path Parameters) مثل `/users/{id}`.
- غياب مطابقة النهايات الدقيقة للجذور.

### مرحلة Go 1.22 وما بعدها (الموجه المتقدم عالي الأداء)

أعاد فريق Go (بقيادة Jonathan Amsterdam) بناء البنية التحتية للتوجيه عبر 3 ملفات مصدريّة متقدمة:

- [`/usr/local/go/src/net/http/pattern.go`](file:///usr/local/go/src/net/http/pattern.go): تحليل وبناء شجرة الأنماط.
- [`/usr/local/go/src/net/http/routing_tree.go`](file:///usr/local/go/src/net/http/routing_tree.go): خوارزمية شجرة التوجيه الهرمية.
- [`/usr/local/go/src/net/http/routing_index.go`](file:///usr/local/go/src/net/http/routing_index.go): فهرسة المقاطع لتسريع البحث دون استهلاك الذاكرة.

---

## 📐 بناء الأنماط وقواعد الصياغة (Pattern Syntax)

تتبع الأنماط في Go الحديثة الصيغة العامة التالية:

$$\text{[METHOD ][HOST]/[PATH]}$$

حيث جميع المكونات اختيارية باستثناء الشرطة المائلة الأولى `/`.

```text
       GET    api.example.com    /users/{id}
      └──┬─┘ └───────┬───────┘  └─────┬─────┘
    Method         Host             Path
   (اختياري)      (اختياري)        (إلزامي)
```

### 1. تحديد طريقة الـ HTTP (Method Matching)

يمكنك بادئة النمط بطريقة الطلب متبوعة بمسافة واحدة أو علامة جدولة (Tab):

```go
mux.HandleFunc("GET /posts", listPosts)
mux.HandleFunc("POST /posts", createPost)
mux.HandleFunc("DELETE /posts/{id}", deletePost)
```

### 2. المتغيرات والمسارات البرمجية المقتطعة (Wildcards)

- **المتغير الفردي `{name}`:** يطابق جزءاً مسارياً واحداً فقط بين شرطتين مائلتين:
  - النمط: `/users/{id}` يطابق `/users/123` و `/users/abc`.
  - لا يطابق `/users/123/profile` (لأنه يتضمن جزءاً إضافياً).
- **المتغير الشامل لبقية المسار `{name...}` (Catch-All Wildcard):** يجب أن يقع في نهاية النمط، ويطابق كل ما تبقى من المسار:
  - النمط: `/files/{path...}` يطابق `/files/docs/2026/report.pdf`.

### 3. التطابق التام للنهايات عبر `{$}`

في الإصدارات السابقة، كان النمط `/` يطابق كل مسارات التطبيق كمسار افتراضي. أما الآن، يتيح الرمز المبتكر `{$}` مطابقة نهاية المسار حرفياً دون مطابقة المتفرعات:

```go
// يطابق حصراً المسار الجذري "/"
mux.HandleFunc("GET /{$}", homePage)

// يطابق حصراً "/admin/" ولا يطابق "/admin/dashboard"
mux.HandleFunc("GET /admin/{$}", adminRoot)
```

### 4. التوجيه بحسب اسم النطاق (Host-Based Routing)

يمكن توجيه الطلبات بحسب ترويسة `Host` الواردة، مما يتيح استضافة عدة نطاقات فرعية داخل نفس خادم التطبيق:

```go
mux.HandleFunc("api.example.com/", apiHandler)
mux.HandleFunc("admin.example.com/", adminHandler)
```

---

## ⚖️ خوارزمية الفرز وقواعد الأسبقية (Precedence & Conflict Resolution)

تعتمد الحزمة قاعدة حاسمة غير قابلة للغموض: **"النمط الأكثر تحديداً يفوز دائماً (Most Specific Pattern Wins)"**.

### مصفوفة المقارنة والتحديد

| الحالة | النمط الأول | النمط الثاني | الفائز ولماذا؟ |
| :--- | :--- | :--- | :--- |
| **تحديد الطريقة** | `GET /posts` | `/posts` | `GET /posts` لأن تحديد الطريقة أكثر تخصصاً من عدم تحديدها. |
| **تحديد النطاق** | `api.example.com/items` | `/items` | `api.example.com/items` لأن تحديد النطاق أكثر تخصصاً. |
| **النص الحرفي مقابل المتغير** | `/users/me` | `/users/{id}` | `/users/me` يفوز لأن النص الحرفي أضيق من المتغير العام. |
| **المتغير مقابل الشامل** | `/files/{file}` | `/files/{path...}` | `/files/{file}` لأن المتغير المفرد أضيق من الشامل المتعدد. |

### كشف التعارضات والتنبيه المبكر (Conflict Detection)

إذا قمت بتسجيل نمطين متطابقين أو متداخلين بحيث لا يمكن لأحدهما أن يكون أكثر تحديداً من الآخر (مثل `/posts/{id}/comments` و `/{type}/123/comments`)، فإن الدالة `mux.Handle` **ترمي فوراً خطأ فادحاً (Panic)** أثناء إقلاع التطبيق، مع رسالة توضح موقع التعارض البرمجي، مما يمنع حدوث أخطاء سلوكية غامضة في بيئة الإنتاج.

---

## 🛠️ الدوال والأساليب البرمجية لـ `ServeMux`

```go
type ServeMux struct {
    // يحتوي داخلياً على شجرة التوجيه والفهارس المحمية بأقفال متزامنة
}
```

### 1. دالة البناء: `NewServeMux()`

```go
func NewServeMux() *ServeMux
```

تنشئ كائن موجه جديد مستقل ونظيف. **يُوصى دائماً بإنشاء موجه خاص وتجنب الاعتماد على الموجه العام الافتراضي.**

### 2. تسجيل المعالجات: `Handle` و `HandleFunc`

```go
func (mux *ServeMux) Handle(pattern string, handler Handler)
func (mux *ServeMux) HandleFunc(pattern string, handler func(ResponseWriter, *Request))
```

تسجل المعالج المرتبط بالنمط. تقوم الدالة داخلياً بتحليل النمط والتحقق من صحته وإدراجه داخل شجرة التوجيه.

### 3. استرجاع ومطابقة المعالج: `Handler`

```go
func (mux *ServeMux) Handler(r *Request) (h Handler, pattern string)
```

تطابق كائن الطلب `r` ضد شجرة الأنماط وتعيد المعالج المخصص مع النمط الذي تمت مطابقته.

### 4. تنفيذ توجيه الطلب: `ServeHTTP`

```go
func (mux *ServeMux) ServeHTTP(w ResponseWriter, r *Request)
```

الدالة التي تحقق واجهة `http.Handler`. تقوم بمطابقة الطلب، واستخراج قيم المتغيرات وإرفاقها بالطلب، ثم استدعاء المعالج النهائي.

---

## 🔍 قراءة وحقن المتغيرات: `PathValue` و الأداء الفائق

### قراءة المتغيرات داخل المعالج

توفر الحزمة أسلوباً مباشراً وسلساً في كائن `*http.Request`:

```go
func (r *Request) PathValue(name string) string
```

### مثال تطبيقي متكامل

```go
package main

import (
    "fmt"
    "net/http"
)

func getUserHandler(w http.ResponseWriter, r *http.Request) {
    // استخراج المتغير مباشرة
    userID := r.PathValue("id")
    fmt.Fprintf(w, "معرف المستخدم المطلوب: %s", userID)
}

func getFilePathHandler(w http.ResponseWriter, r *http.Request) {
    // استخراج المتغير الشامل لبقية المسار
    filePath := r.PathValue("path")
    fmt.Fprintf(w, "المسار الكامل للملف: %s", filePath)
}

func main() {
    mux := http.NewServeMux()
    mux.HandleFunc("GET /users/{id}", getUserHandler)
    mux.HandleFunc("GET /static/{path...}", getFilePathHandler)

    http.ListenAndServe(":8080", mux)
}
```

### سر الأداء الهندسي: لماذا البحث الخطي (Linear Search)؟

في الشيفرة المصدرية لكائن `Request`:

```go
func (r *Request) PathValue(name string) string {
    if i := r.patIndex(name); i >= 0 {
        return r.matches[i]
    }
    return r.otherValues[name]
}
```

بدلاً من تخصيص خريطة (`map[string]string`) لكل طلب وارد، تقوم Go بحفظ المتغيرات في شريحة متراصة مسطحة (`slice`). أظهرت الاختبارات القياسية لمطوري Go أن البحث الخطي في شريحة صغيرة تحتوي على متغيرين أو ثلاثة أسرع بمراحل من تخصيص خريطة في الذاكرة (Heap Allocation) وتوليد ضغط على جامع القمامة (Zero Garbage Collection Pressure).

---

## ⏪ التوافقية العكسية ومفتاح `GODEBUG`

إذا كان لديك نظام قديم يعتمد على التفسير الحرفي للأنماط القديمة وتخشى تأثر المسارات التي كانت تحتوي على أقواس معقوفة كجزء من الرابط الحرفي، توفر Go مفتاح التوافقية العكسية:

```bash
GODEBUG=httpmuxgo121=1 ./myserver
```

عند تفعيل هذا المتغير، يعود `ServeMux` للعمل بقواعد Go 1.21 القديمة دون تحليل الأنماط المتقدمة.
