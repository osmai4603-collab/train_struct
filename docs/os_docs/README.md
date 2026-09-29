# توثيق المعمارية البرمجية لحزمة `os` في لغة Go

مرحباً بك في الدليل المعماري والتوثيق الهندسي الشامل لحزمة **`os`** (Standard Library Operating System Package) في لغة Go (الإصدار الحديث Go 1.24+ / Go 1.27).

تم إعداد هذا التوثيق ليكون مرجعاً هندسياً معمقاً يشرح الفلسفة التصميمية للحزمة، ومبررات وجودها، وتحليلاً تفصيلياً لكل دالة وواجهة وهيكل بيانات، مع تشريح الكواليس منخفضة المستوى (Low-Level Internals) وأفضل الممارسات والأنماط المضادة في بيئات الإنتاج.

---

## 📚 الفهرس العام وفصول التوثيق

| الفصل | الملف | المحتوى ومحاور الدراسة |
| :---: | :--- | :--- |
| **01** | [الفلسفة المعمارية ومبررات وجود حزمة `os`](./01_philosophy_and_architecture.md) | السياق التاريخي (Unix vs Windows NT vs Plan 9)، المبادئ المعمارية الثلاثة، المشاكل الجوهرية الست، مخطط البنية الطبقية للحزمة وتكاملها مع النواة. |
| **02** | [الواجهات والأنواع والهياكل الأساسية](./02_core_types_and_interfaces.md) | شرح واجهة `Signal`، هيكل `File` والتغليف الداخلي `*file`، عزل `Root`، أنواع `FileInfo`, `FileMode`, `DirEntry`، هياكل العمليات `Process` و `ProcessState`، هياكل الأخطاء (`PathError`, `LinkError`, `SyscallError`)، الثوابت والمتغيرات العامة. |
| **03** | [عمليات الملفات وهندسة الإدخال والإخراج](./03_file_operations_and_io.md) | دوال الفتح والإنشاء (`Open`, `Create`, `OpenFile`, `CreateTemp`)، القراءة والكتابة المتزامنة (`ReadAt`, `WriteAt`)، تقنيات التسريع بدون نسخ (Zero-Copy عبر `sendfile`, `copy_file_range`, `splice`)، المزامنة (`Sync`)، المهل الزمنية (`Deadlines`)، والوصول الخام (`SyscallConn`). |
| **04** | [إدارة المجلدات ونظام الملفات والمسارات](./04_filesystem_and_directory_management.md) | إنشاء وحذف المجلدات، هندسة الأمان في `RemoveAll` ضد سباقات الروابط، مسح المجلدات فائق السرعة عبر `ReadDir`، الروابط الصلبة والرمزية (`Link`, `Symlink`)، البيانات الوصفية (`Stat`, `Lstat`, `SameFile`)، الأذونات والملكيات، ومعايير XDG لمسارات المستخدم. |
| **05** | [نظام العزل الجذري وصناديق الحماية `os.Root`](./05_sandboxing_and_root_subsystem.md) | ثورة الأمان في Go 1.24+، القضاء على ثغرات Zip Slip و TOCTOU، تقنيات النواة (`openat2` و `RESOLVE_BENEATH`)، تفصيل كامل لكافة دوال كائن `Root`، ومثال عملي لفك الأرشيف بأمان مطلق. |
| **06** | [دورة حياة العمليات والبيئة النظامية](./06_process_lifecycle_and_environment.md) | إدارة العمليات الفرعية (`StartProcess`, `FindProcess`)، ثورة `pidfd` في Linux ومنع سباق إعادة تدوير الـ PID، تقرير حالة العملية `ProcessState` واستهلاك الموارد (`Rusage`)، دوال متغيرات البيئة (`LookupEnv` vs `Getenv`)، بطاقة هوية النظام، ومخاطر `os.Exit`. |
| **07** | [الكواليس العميقة وهندسة الأداء](./07_low_level_internals_and_performance.md) | تكامل الملفات مع مجدول Go ومراقب الشبكة (`poll.FD` & Netpoller)، معالجة انقطاع الاستدعاءات عبر `ignoringEINTR`، تفعيل `O_CLOEXEC` ذرّياً لمنع تسريب المقابض، حوض الذاكرة المؤقتة `sync.Pool` في قراءة المجلدات، وتشريح فك الأخطاء. |
| **08** | [أفضل الممارسات البرمجية والأنماط المضادة](./08_best_practices_security_and_antipatterns.md) | القواعد الإنتاجية الست (الاستبدال الذري للملفات، فرض `os.Root` للمدخلات غير الموثوقة، المزامنة الإلزامية `Sync`)، الأنماط الستة المضادة القاتلة، وقوالب برمجية جاهزة للأقفال الاستشارية (`flock`). |

---

## 🗺️ جدول الحصر والمطابقة الشامل لكافة دوال وعناصر الحزمة (Comprehensive API Matrix)

يوضح الجدول التالي كافة الدوال والأنواع المصدرة في حزمة `os` وموقع شرحها في ملفات التوثيق:

| العنصر البرمجي (Function / Type / Var) | التصنيف | الوصف الهندسي الموجز | الفصل المرجعي |
| :--- | :---: | :--- | :---: |
| **`Signal`** | واجهة Interface | تمثيل إشارات نظام التشغيل | [الفصل 02](./02_core_types_and_interfaces.md) |
| **`File`** | هيكل Struct | كائن واصف الملف التجريدي الآمن | [الفصل 02](./02_core_types_and_interfaces.md), [الفصل 03](./03_file_operations_and_io.md) |
| **`FileInfo`** | واجهة مستعارة | البيانات الوصفية للملف (حجم، وقت، أذونات) | [الفصل 02](./02_core_types_and_interfaces.md) |
| **`FileMode`** | نوع Type | بنية بتات الأذونات ونوع الملف | [الفصل 02](./02_core_types_and_interfaces.md) |
| **`DirEntry`** | واجهة مستعارة | عنصر مجلد فائق السرعة وخفيف الوزن | [الفصل 02](./02_core_types_and_interfaces.md), [الفصل 04](./04_filesystem_and_directory_management.md) |
| **`Root`** | هيكل Struct | صندوق حماية وعزل جذري للمجلدات (Go 1.24+) | [الفصل 02](./02_core_types_and_interfaces.md), [الفصل 05](./05_sandboxing_and_root_subsystem.md) |
| **`Process`** | هيكل Struct | مقبض العملية المشتقة في النظام | [الفصل 02](./02_core_types_and_interfaces.md), [الفصل 06](./06_process_lifecycle_and_environment.md) |
| **`ProcessState`** | هيكل Struct | تقرير حالة انتهاء العملية واستهلاك الموارد | [الفصل 02](./02_core_types_and_interfaces.md), [الفصل 06](./06_process_lifecycle_and_environment.md) |
| **`ProcAttr`** | هيكل Struct | سمات إطلاق العمليات (مسار، بيئة، واصفات) | [الفصل 02](./02_core_types_and_interfaces.md), [الفصل 06](./06_process_lifecycle_and_environment.md) |
| **`PathError`** | هيكل خطأ Struct | خطأ مسار غني بالسياق (العملية، المسار، السبب) | [الفصل 02](./02_core_types_and_interfaces.md), [الفصل 07](./07_low_level_internals_and_performance.md) |
| **`LinkError`** | هيكل خطأ Struct | خطأ روابط أو إعادة تسمية ملفات | [الفصل 02](./02_core_types_and_interfaces.md) |
| **`SyscallError`** | هيكل خطأ Struct | خطأ استدعاء نظام مباشر من النواة | [الفصل 02](./02_core_types_and_interfaces.md) |
| **`Stdin / Stdout / Stderr`** | متغيرات Vars | مجاري الإدخال والإخراج والأخطاء القياسية | [الفصل 02](./02_core_types_and_interfaces.md) |
| **`Args`** | متغير Var | مصفوفة وسائط سطر الأوامر | [الفصل 02](./02_core_types_and_interfaces.md) |
| **`Open`** | دالة Function | فتح ملف للقراءة فقط | [الفصل 03](./03_file_operations_and_io.md) |
| **`Create`** | دالة Function | إنشاء ملف جديد أو تصفيره للقراءة والكتابة | [الفصل 03](./03_file_operations_and_io.md) |
| **`OpenFile`** | دالة Function | محرك فتح الملفات المخصص بالرايات والصلاحيات | [الفصل 03](./03_file_operations_and_io.md) |
| **`NewFile`** | دالة Function | تحويل واصف نظام خام إلى كائن `*File` | [الفصل 03](./03_file_operations_and_io.md) |
| **`CreateTemp`** | دالة Function | إنشاء ملف مؤقت ذري وآمن ضد السباقات | [الفصل 03](./03_file_operations_and_io.md) |
| **`ReadFile`** | دالة Function | قراءة كامل محتوى الملف دفعة واحدة | [الفصل 03](./03_file_operations_and_io.md) |
| **`WriteFile`** | دالة Function | كتابة كامل بيانات الملف واستبداله دفعة واحدة | [الفصل 03](./03_file_operations_and_io.md) |
| **`File.Read / Write`** | توابع Methods | قراءة وكتابة متدفقة مع تحريك المؤشر | [الفصل 03](./03_file_operations_and_io.md) |
| **`File.ReadAt / WriteAt`** | توابع Methods | قراءة وكتابة متزامنة وخيطية آمنة دون تحريك المؤشر | [الفصل 03](./03_file_operations_and_io.md) |
| **`File.ReadFrom / WriteTo`** | توابع Methods | نقل صفري فائق السرعة عبر `sendfile`/`splice` | [الفصل 03](./03_file_operations_and_io.md), [الفصل 07](./07_low_level_internals_and_performance.md) |
| **`File.Seek`** | تابع Method | تحريك مؤشر الموضع في الملف | [الفصل 03](./03_file_operations_and_io.md) |
| **`File.Sync`** | تابع Method | إجبار كتابة البيانات على وسيط التخزين (`fsync`) | [الفصل 03](./03_file_operations_and_io.md), [الفصل 08](./08_best_practices_security_and_antipatterns.md) |
| **`File.Truncate` / `Truncate`** | دالة/تابع | تقليص أو توسيع حجم الملف | [الفصل 03](./03_file_operations_and_io.md) |
| **`File.SetDeadline`** | تابع Method | ضبط مهل زمنية للإدخال والإخراج للأنابيب والمقابس | [الفصل 03](./03_file_operations_and_io.md) |
| **`File.SyscallConn`** | تابع Method | وصول آمن لواصف النظام الخام دون تعطيل Netpoller | [الفصل 03](./03_file_operations_and_io.md), [الفصل 08](./08_best_practices_security_and_antipatterns.md) |
| **`File.Fd`** | تابع Method | جلب رقم واصف الملف (يحول للوضع التعطيلي) | [الفصل 03](./03_file_operations_and_io.md) |
| **`File.Close`** | تابع Method | إغلاق الملف وفك تسجيله بأمان وعد مرجعي | [الفصل 03](./03_file_operations_and_io.md) |
| **`Mkdir / MkdirAll`** | دوال Functions | إنشاء مجلدات فردية أو تكرارية | [الفصل 04](./04_filesystem_and_directory_management.md) |
| **`MkdirTemp`** | دالة Function | إنشاء مجلد مؤقت باسم عشوائي وصلاحيات 0700 | [الفصل 04](./04_filesystem_and_directory_management.md) |
| **`Remove`** | دالة Function | حذف ملف مفرد أو مجلد فارغ | [الفصل 04](./04_filesystem_and_directory_management.md) |
| **`RemoveAll`** | دالة Function | حذف تكراري محصن عبر `openat`/`unlinkat` | [الفصل 04](./04_filesystem_and_directory_management.md) |
| **`ReadDir`** | دالة Function | قراءة محتويات المجلد كـ `[]DirEntry` بأعلى كفاءة | [الفصل 04](./04_filesystem_and_directory_management.md) |
| **`Link / Symlink / Readlink`** | دوال Functions | إدارة الروابط الصلبة والرمزية وقراءة وجهتها | [الفصل 04](./04_filesystem_and_directory_management.md) |
| **`Stat / Lstat`** | دوال Functions | فحص خصائص الملف (مع أو بدون تتبع الروابط) | [الفصل 04](./04_filesystem_and_directory_management.md) |
| **`SameFile`** | دالة Function | مقارنة دقيقة للملفات بالـ Inode ورقم الجهاز | [الفصل 04](./04_filesystem_and_directory_management.md) |
| **`Chmod / Chown / Chtimes`** | دوال Functions | تعديل أذونات وملكية وتوقيتات الملفات | [الفصل 04](./04_filesystem_and_directory_management.md) |
| **`Getwd / Chdir`** | دوال Functions | قراءة وتغيير مجلد العمل الحالي للعملية | [الفصل 04](./04_filesystem_and_directory_management.md) |
| **`TempDir / User*Dir`** | دوال Functions | استعلام المسارات المعيارية للمستخدم (XDG) | [الفصل 04](./04_filesystem_and_directory_management.md) |
| **`DirFS / CopyFS`** | دوال Functions | تجريد شجرة المجلدات ونسخ أنظمة `fs.FS` كاملة | [الفصل 04](./04_filesystem_and_directory_management.md) |
| **`Pipe`** | دالة Function | إنشاء أنبوب اتصال متزامن في الذاكرة | [الفصل 04](./04_filesystem_and_directory_management.md) |
| **`OpenRoot / OpenInRoot`** | دوال Functions | فتح صندوق حماية جذري معزول للنظام | [الفصل 05](./05_sandboxing_and_root_subsystem.md) |
| **`Root.*` (كافة توابع الجذر)** | توابع Methods | عمليات ملفات مقيدة داخل المجلد الجذري فقط | [الفصل 05](./05_sandboxing_and_root_subsystem.md) |
| **`StartProcess / FindProcess`** | دوال Functions | إطلاق العمليات الفرعية والعثور عليها | [الفصل 06](./06_process_lifecycle_and_environment.md) |
| **`Process.Kill / Signal / Wait`** | توابع Methods | إنهاء، إرسال إشارات، وانتظار العمليات وتفريغها | [الفصل 06](./06_process_lifecycle_and_environment.md) |
| **`Getenv / LookupEnv`** | دوال Functions | قراءة متغيرات البيئة (التمييز بين الفارغ والمعدوم) | [الفصل 06](./06_process_lifecycle_and_environment.md) |
| **`Setenv / Unsetenv / Clearenv`** | دوال Functions | تعديل وحذف وتفريغ متغيرات البيئة للعملية | [الفصل 06](./06_process_lifecycle_and_environment.md) |
| **`Environ / Expand / ExpandEnv`** | دوال Functions | سرد وتمديد نصوص المتغيرات البيئية | [الفصل 06](./06_process_lifecycle_and_environment.md) |
| **`Getuid / Geteuid / Getgid...`** | دوال Functions | استعلام الهوية الأمنية للمستخدم والمجموعات | [الفصل 06](./06_process_lifecycle_and_environment.md) |
| **`Getpagesize / Hostname / Executable`** | دوال Functions | استعلام حجم صفحة الذاكرة واسم الجهاز ومسار البرنامج | [الفصل 06](./06_process_lifecycle_and_environment.md) |
| **`Exit`** | دالة Function | إنهاء فوري للعملية (مع تخطي دوال `defer`) | [الفصل 06](./06_process_lifecycle_and_environment.md), [الفصل 08](./08_best_practices_security_and_antipatterns.md) |
| **`IsNotExist / IsExist...`** | دوال فحص | دوال مساعدة لفحص تصنيف الأخطاء | [الفصل 07](./07_low_level_internals_and_performance.md) |

---

## 💡 الخلاصة والتوصية الهندسية

تُثبت حزمة `os` في لغة Go أن البساطة لا تعني المساومة على الأداء أو الأمان. عبر دراستك لهذه الملفات، أصبحت تمتلك الفهم العميق لكيفية تخاطب كود Go مع أنوية النظم المختلفة، وكيفية توظيف هذه المعرفة لبناء خدمات سحابية ونظم خلفية تمتاز بالسرعة الخارقة والمناعة الأمنية الصارمة.
