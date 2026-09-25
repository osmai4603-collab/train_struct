# 02. الواجهات والأنواع والهياكل الأساسية في حزمة `os`

تعتمد حزمة `os` على مجموعة منتقاة بعناية من الواجهات (Interfaces)، والأنواع البديلة (Type Aliases المستوردة من `io/fs`)، وهياكل البيانات الصريحة (Structs)، والمتغيرات العامة، والثوابت. يشرح هذا الملف بنية كل نوع، ومجاله، ودوافعه التصميمية، وتفاصيله الداخلية.

---

## 🧩 1. الواجهات (Interfaces)

### واجهة `os.Signal`

```go
type Signal interface {
    String() string
    Signal() // دالة فارغة للتمييز عن بقية الـ Stringers
}
```

#### التحليل المعماري
- تمثل إشارات نظام التشغيل (OS Signals) مثل `SIGINT` و `SIGTERM` و `SIGHUP`.
- تحتوي الواجهة على دالة تمييزية خاصة `Signal()` لا تفعل شيئاً سوى منع الأنواع النصية الأخرى التي تطبق `fmt.Stringer` من التوافق الخاطئ مع واجهة الإشارات (`marker method`).
- **التطبيق العملي في الأنظمة:**
  - في أنظمة Unix / Linux / macOS: النوع الحقيقي الذي يطبق هذه الواجهة هو `syscall.Signal` (وهو مجرد رقم صحيح `int`).
  - في نظام Plan 9: يطبقها `syscall.Note` (وهو نص `string`).
  - في نظام Windows: يتم دعم إشارات محدودة محاكاة برمجياً (مثل `os.Interrupt`).

#### الإشارات العامة المسبقة:
```go
var (
    Interrupt Signal = syscall.SIGINT  // إشارة المقاطعة (Ctrl+C)
    Kill      Signal = syscall.SIGKILL // إشارة الإنهاء الجبري غير القابل للاعتراض
)
```

---

## 🏗️ 2. الهياكل الأساسية والأنواع (Core Structs & Types)

### أ. هيكل الملف `os.File` والتغليف الداخلي `file`

```go
type File struct {
    *file // مؤشر غير مكشوف لهيكل النظام الفعلي
}
```

#### لماذا تم استخدام مستوى غير مباشر (`*file`)؟
إذا كان `File` هيكلاً عادياً يحتوي الحقول مباشرة، لكان بإمكان كود العميل الخارجي نسخ القيمة (`f2 := *f1`) مما يؤدي إلى وجود نسختين تشيران إلى نفس واصف الملف (`fd`). وإذا قام مجمع القمامة (GC) بتشغيل المُنهي النهائي (`Finalizer`) لأحد النسخ، سيُغلق الـ `fd` بينما النسخة الأخرى ما زالت تستخدمه، أو يتم إغلاق واصف ملف جديد أُعيد استخدامه!
لذلك، جعلت Go كائن `*file` داخلياً فريداً، والمُنهي النهائي يراقب مؤشر `*file` الداخلي حصراً.

#### الحقول الداخلية لهيكل `file` في أنظمة Unix:
```go
type file struct {
    pfd         poll.FD                 // واصف الملف المدمج مع Go Netpoller
    name        string                  // اسم الملف عند الفتح
    dirinfo     atomic.Pointer[dirInfo] // معلومات التخزين المؤقت لقراءة المجلدات
    nonblock    bool                    // هل تم تفعيل الوضع غير التعطيلي
    stdoutOrErr bool                    // هل يمثل هذا المجرى Stdout أو Stderr
    appendMode  bool                    // هل فُتح الملف بوضعية الإلحاق O_APPEND
    inRoot      bool                    // هل فُتح الملف تحت نطاق os.Root
}
```

---

### ب. هيكل المجلد الجذري المعزول `os.Root` (Go 1.24+)

```go
type Root struct {
    root *root
}

type root struct {
    name   string
    mu     sync.Mutex
    fd     sysfdType // واصف المجلد في النواة
    refs   int       // عداد العمليات الجارية التي تستخدم الـ fd
    closed bool      // هل تم إغلاق الجذر
}
```

#### التحليل المعماري
- يوفر صندوق حماية (Sandbox) للوصول إلى نظام الملفات محصوراً داخل شجرة المجلد المحددة فقط.
- يعتمد داخلياً على تقنية العد المرجعي (`refs`): عند استدعاء `Close()`، إذا كانت هناك Goroutines أخرى تنفذ عمليات قراءة أو إنشاء ملفات تحت الجذر، لا يُغلق واصف المجلد فوراً، بل يُؤجل الإغلاق الفعلي حتى تنتهي آخر Goroutine مستخدمة، مما يمنع إغلاق المقبض أثناء العمليات.

---

### ج. واصفات ومعلومات الملفات `FileInfo` و `FileMode` و `DirEntry`

مع إصدار Go 1.16، تم توحيد تجريدات نظام الملفات في حزمة `io/fs`. تستورد حزمة `os` هذه الأنواع كبدائل متطابقة (`Type Aliases`):

#### 1. نوع `FileInfo`
```go
type FileInfo = fs.FileInfo
```
واجهة تصف البيانات الوصفية (Metadata) للملف أو المجلد:
```go
type FileInfo interface {
    Name() string       // الاسم الأساسي للملف فقط (Base Name)
    Size() int64        // حجم الملف بالبايت
    Mode() FileMode     // بتات نوع الملف وأذونات الوصول
    ModTime() time.Time // وقت آخر تعديل
    IsDir() bool        // اختصار لفحص Mode().IsDir()
    Sys() any           // البنية التحتية للنظام (مثل *syscall.Stat_t في Unix)
}
```

#### 2. نوع `FileMode` وبنية البتات (Bitmask Layout)
```go
type FileMode = fs.FileMode
```
يمثل `uint32` يحمل أذونات Unix القياسية في البتات الـ 9 الصغرى، بينما تحمل البتات العليا خصائص الملف ونوعه:

```text
31             24 23             16 15        9 8       0
┌────────────────┬────────────────┬────────────┬─────────┐
│   File Type    │ Special Modes  │  Unused    │ rwxrwxrwx
│ (Dir, Link...) │(Setuid, Sticky)│            │ Unix Perm
└────────────────┴────────────────┴────────────┴─────────┘
```

**أهم ثوابت `FileMode`:**
- `ModeDir` (1<<31): يشير إلى أن الملف مجلد.
- `ModeAppend` (1<<30): ملف يُكتب في نهايته فقط (Append-only).
- `ModeExclusive` (1<<29): استخدام حصري (Lock/Plan 9).
- `ModeTemporary` (1<<28): ملف مؤقت.
- `ModeSymlink` (1<<27): رابط رمزي (Symbolic link).
- `ModeDevice` (1<<26): ملف جهاز نظام (Device file).
- `ModeNamedPipe` (1<<25): أنبوب مسمى (FIFO).
- `ModeSocket` (1<<24): مقبس شبكة أو يونكس (Unix Domain Socket).
- `ModeSetuid` (1<<23): خاصية Setuid.
- `ModeSetgid` (1<<22): خاصية Setgid.
- `ModeCharDevice` (1<<21): جهاز حرفي لنظام يونكس (عندما يكون ModeDevice مفعلاً).
- `ModeSticky` (1<<20): خاصية الـ Sticky bit.
- `ModeIrregular` (1<<19): ملف غير منتظم لا يعرف عنه شيء آخر.
- `ModePerm` (`0777`): قناع البتات الخاص بأذونات Unix القياسية للمستخدم والمجموعة والآخرين.

**دوال `FileMode`:**
- `(m FileMode) IsDir() bool`: هل هو مجلد.
- `(m FileMode) IsRegular() bool`: هل هو ملف بيانات عادي (ليس مجلداً، ولا رابطاً، ولا مقبساً، ولا جهازاً).
- `(m FileMode) Perm() FileMode`: استخلاص أذونات Unix فقط (`m & ModePerm`).
- `(m FileMode) Type() FileMode`: استخلاص بتات نوع الملف فقط.
- `(m FileMode) String() string`: تمثيل نصي بصيغة Unix (مثل `-rwxr-xr-x` أو `drwxrwxrwt`).

#### 3. نوع `DirEntry` وثورة الأداء مقارنة بـ `FileInfo`
```go
type DirEntry = fs.DirEntry
```
واجهة خفيفة جداً لتمثيل عناصر المجلد:
```go
type DirEntry interface {
    Name() string
    IsDir() bool
    Type() FileMode
    Info() (FileInfo, error)
}
```

> [!TIP]
> **مقارنة الأداء الجوهرية (DirEntry vs FileInfo):**
> عند قراءة مجلد يحتوي على 10,000 ملف:
> - استخدام `os.ReadDir` يعود بـ `[]DirEntry`. في أنظمة Linux، يستخرج أسماء الملفات وأنواعها مباشرة من بنية `struct dirent` القادمة من استدعاء النواة `getdents64` في عملية واحدة بذاكرة موحدة وبدون أي استدعاء إضافي!
> - الطريقة القديمة عبر `f.Readdir` كانت تعود بـ `[]FileInfo`، مما يُجبر وقت التشغيل على استدعاء `stat()` منفصل لكل ملف على حدة (10,000 استدعاء نظام إضافي)، مما يسبب بطئاً هائلاً وخنقاً لمعالج النظام. لا تستدعي `entry.Info()` إلا إذا كنت بحاجة حقيقية لحجم الملف أو وقت تعديله!

---

### د. هياكل إدارة العمليات `Process` و `ProcessState` و `ProcAttr`

#### 1. هيكل `os.ProcAttr`
يحدد سمات إطلاق عملية جديدة عبر `StartProcess`:
```go
type ProcAttr struct {
    Dir   string                  // مجلد العمل الأولي للعملية
    Env   []string                // متغيرات البيئة ("KEY=VALUE")
    Files []*File                 // ملفات المجرى القياسي (0: Stdin, 1: Stdout, 2: Stderr, إلخ)
    Sys   *syscall.SysProcAttr    // إعدادات النواة المتقدمة (مثل Chroot, Namespaces, Credentials)
}
```

#### 2. هيكل `os.Process`
يمثل مقبض العملية في نظام التشغيل:
```go
type Process struct {
    Pid     int             // المعرف العددي للعملية
    state   atomic.Uint32   // حالة العملية الذرية (نشطة، منتهية، محررة)
    sigMu   sync.RWMutex    // قفل الحماية بين الإشارات والانتظار في الأنظمة القديمة
    handle  *processHandle  // مؤشر المقبض المتقدم (pidfd في Linux / Handle في Windows)
    cleanup runtime.Cleanup // آلية التنظيف التلقائي في Go الحديثة
}
```

#### 3. هيكل `os.ProcessState`
يخزن تقرير حالة العملية بعد انتهائها وخروجها من خلال `Wait()`:
```go
type ProcessState struct {
    pid    int                // معرف العملية التي انتهت
    status syscall.WaitStatus // كود الحالة الخام من النواة
    rusage *syscall.Rusage    // إحصائيات استهلاك الموارد (CPU, RAM, Page faults)
}
```
**دوال `ProcessState`:**
- `ExitCode() int`: يعود برمز الخروج (0 للنجاح، 1..255 للخطأ، أو 1- إذا قُتلت بإشارة).
- `Exited() bool`: هل انتهت العملية بشكل طبيعي.
- `Success() bool`: هل انتهت بنجاح كامل (رمز الخروج = 0).
- `Pid() int`: معرف العملية.
- `UserTime() time.Duration`: إجمالي وقت المعالج المستهلك في وضع المستخدم (User Mode).
- `SystemTime() time.Duration`: إجمالي وقت المعالج المستهلك داخل نواة النظام (Kernel Mode).
- `Sys() any`: يعود بحالة الانتظار الخام لنظام التشغيل (`syscall.WaitStatus`).
- `SysUsage() any`: يعود بإحصائيات الموارد التفصيلية (`*syscall.Rusage`).
- `String() string`: وصف نصي لحالة الخروج.

---

## ⚠️ 3. هياكل الأخطاء المخصصة (Error Structs)

تعتمد حزمة `os` على أخطاء غنية السياق تفكك المشكلة بالكامل:

### أ. هيكل `os.PathError`
```go
type PathError = fs.PathError

type PathError struct {
    Op   string // العملية التي فشلت (مثل "open", "stat", "unlink")
    Path string // مسار الملف المعني
    Err  error  // الخطأ الجذري من النواة (مثل syscall.ENOENT)
}
```
- تطبق دوال `Error()` و `Unwrap() error` و `Timeout() bool`.
- التنسيق النصي التلقائي: `"open /path/to/file: no such file or directory"`.

### ب. هيكل `os.LinkError`
```go
type LinkError struct {
    Op  string // "link", "symlink", "rename"
    Old string // المسار القديم أو المصدر
    New string // المسار الجديد أو الهدف
    Err error  // سبب الفشل
}
```
- تطبق `Error() string` و `Unwrap() error`.
- التنسيق النصي: `"rename /old /new: file already exists"`.

### ج. هيكل `os.SyscallError`
```go
type SyscallError struct {
    Syscall string // اسم استدعاء النظام (مثل "setenv", "getgroups")
    Err     error  // خطأ النواة errno
}
```
- تطبق `Error() string` و `Unwrap() error` و `Timeout() bool`.

---

## 🌐 4. المتغيرات العامة (Global Variables)

### أ. مجاري الإدخال والإخراج القياسية (Standard Streams)
```go
var (
    Stdin  = NewFile(uintptr(syscall.Stdin), "/dev/stdin")   // الواصف 0
    Stdout = NewFile(uintptr(syscall.Stdout), "/dev/stdout") // الواصف 1
    Stderr = NewFile(uintptr(syscall.Stderr), "/dev/stderr") // الواصف 2
)
```
> [!CAUTION]
> **تحذير خطير بخصوص إغلاق `Stderr`:**
> يعتمد محرك تشغيل Go (Go Runtime) على `Stderr` لكتابة رسائل الانهيار وحالات الذعر (Panics & Crashes). إذا قام برنامجك بإغلاق `os.Stderr.Close()`، فقد يؤدي ذلك إلى كتابة تقارير الذعر في ملف آخر يتم فتحه لاحقاً ويأخذ واصف الملف رقم 2، مما يتسبب في تلوث بيانات ملفاتك!

### ب. الأخطاء القياسية المعرفة مسبقاً (Sentinel Errors)
```go
var (
    ErrInvalid          = fs.ErrInvalid          // "invalid argument"
    ErrPermission       = fs.ErrPermission       // "permission denied"
    ErrExist            = fs.ErrExist            // "file already exists"
    ErrNotExist         = fs.ErrNotExist         // "file does not exist"
    ErrClosed           = fs.ErrClosed           // "file already closed"
    ErrNoDeadline       = errNoDeadline()       // "file type does not support deadline"
    ErrDeadlineExceeded = errDeadlineExceeded() // "i/o timeout"
    ErrProcessDone      = errors.New("os: process already finished")
    ErrNoHandle         = errors.New("os: process handle unavailable")
)
```
جميع هذه الأخطاء متوافقة تماماً مع دالة `errors.Is(err, os.ErrNotExist)`.

### ج. وسائط سطر الأوامر `Args`
```go
var Args []string
```
مصفوفة تحتوي وسائط تشغيل البرنامج، حيث `Args[0]` هو اسم البرنامج أو مسار الملف التنفيذي، وتليه الوسائط الأخرى.

---

## 🔒 5. الثوابت الأساسية (Constants)

### أ. رايات فتح الملفات (Open Flags)
```go
const (
    O_RDONLY int = syscall.O_RDONLY // فتح للقراءة فقط
    O_WRONLY int = syscall.O_WRONLY // فتح للكتابة فقط
    O_RDWR   int = syscall.O_RDWR   // فتح للقراءة والكتابة معاً
    O_APPEND int = syscall.O_APPEND // الكتابة في نهاية الملف دائماً
    O_CREATE int = syscall.O_CREAT  // إنشاء الملف إن لم يكن موجوداً
    O_EXCL   int = syscall.O_EXCL   // مع O_CREATE: يفشل إذا كان الملف موجوداً مسبقاً (ذري)
    O_SYNC   int = syscall.O_SYNC   // مزامنة فورية للقرص (Synchronous I/O)
    O_TRUNC  int = syscall.O_TRUNC  // تصفير حجم الملف وحذف محتواه القديم عند الفتح
)
```

### ب. ثوابت تحديد الموقع في الملف (Seek Whence)
```go
const (
    SEEK_SET int = 0 // إزاحة نسبة إلى بداية الملف (Deprecated: استخدم io.SeekStart)
    SEEK_CUR int = 1 // إزاحة نسبة إلى الموقع الحالي (Deprecated: استخدم io.SeekCurrent)
    SEEK_END int = 2 // إزاحة نسبة إلى نهاية الملف  (Deprecated: استخدم io.SeekEnd)
)
```

### ج. فواصل المسارات وأسماء الأجهزة
```go
const (
    PathSeparator     = '/' // في Unix يكون '/' وفي Windows يكون '\'
    PathListSeparator = ':' // في Unix يكون ':' وفي Windows يكون ';' (مثل متغير PATH)
    DevNull           = "/dev/null" // في Unix يكون "/dev/null" وفي Windows يكون "NUL"
)
```
