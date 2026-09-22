# مصفوفة أسبقية الإعدادات والمصادر (Priority Hierarchy Matrix)

تحدد هذه المصفوفة الترتيب الصارم لدمج الإعدادات من مصادرها المختلفة، مع توضيح أمثلة تطبيقية وقواعد التجاوز (Overrides).

---

## مصفوفة الطبقات وقواعد الدمج

| الأسبقية | الطبقة (Layer) | المصدر (Source) | مثال المدخل | الغرض التشغيلي |
| :---: | :--- | :--- | :--- | :--- |
| **5 (الأعلى)** | **Runtime Overrides** | برمجي عبر `WithRuntimeOverrides` | `cfg.Server.Port = "0"` | اختبارات التكامل الآلية والـ Ephemeral Ports |
| **4** | **CLI Flags** | وسائط سطر الأوامر | `--port 9090` | تجاوز سريع ومباشر من قبل مشغل النظام |
| **3** | **Environment** | متغيرات النظام والـ Docker | `PORT=8070`, `DB_HOST=pg` | تكوين الحاويات وبيئات Kubernetes |
| **2** | **Config File** | ملف JSON على القرص | `config/config.json` | الإعدادات الثابتة لبيئة العمل (Staging/Production) |
| **1 (الأدنى)** | **Defaults** | كود Go الصلب (`Defaults()`) | `Port: "8080"` | قيم آمنة تضمن إقلاع الخادم دون أي ملف خارجي |

---

## مصفوفة الأسماء البديلة وأسرار الملفات (Aliases & Secrets Mapping)

عند قراءة المتغيرات البيئية، يدعم المحرك الأسماء القياسية السحابية، وأسماء PostgreSQL المعتمدة، ونمط أسرار الملفات المحقونة (`*_FILE`):

| الحقل في Go | المتغير البيئي الأساسي (Canonical) | المتغيرات البديلة المدعومة (Aliases) | نمط أسرار الملفات (*_FILE) |
| :--- | :--- | :--- | :--- |
| `Server.Port` | `PORT` | `HTTP_PORT`, `TRAIN_PORT` | - |
| `Server.Host` | `HOST` | `HTTP_INTERFACE`, `BIND_ADDRESS` | - |
| `Server.MaxHeaderBytes` | `MAX_HEADER_BYTES` | `HEADER_LIMIT` | - |
| `Server.MaxBodySize` | `MAX_BODY_SIZE` | `BODY_LIMIT` | - |
| `Database.Host` | `DB_HOST` | `PGHOST`, `POSTGRES_HOST` | - |
| `Database.Port` | `DB_PORT` | `PGPORT`, `POSTGRES_PORT` | - |
| `Database.Name` | `DB_NAME` | `PGDATABASE`, `POSTGRES_DB` | - |
| `Database.User` | `DB_USER` | `PGUSER`, `POSTGRES_USER` | - |
| `Database.Password`| `DB_PASSWORD` | `PGPASSWORD`, `POSTGRES_PASSWORD` | `DB_PASSWORD_FILE` |
| `Database.DatabaseURL`| `DATABASE_URL` | `DB_URL`, `POSTGRES_URL` | `DATABASE_URL_FILE` |
| `Auth.JWTSecret` | `JWT_SECRET` | `AUTH_SECRET` | `JWT_SECRET_FILE` |

---

## سلوك المفاتيح الغائبة مقابل الفارغة (Absent vs Empty Keys)

- **المفتاح الغائب (Absent Key)**: إذا لم يرد المفتاح في الملف أو البيئة، يحتفظ التطبيق بالقيمة الافتراضية (`Defaults`).
- **المفتاح الفارغ الصريح (Explicit Empty String `""`)**: إذا قام المشغل بتعيين `DB_PASSWORD=""`، يتم اعتماد السلسلة الفارغة صراحة (مفيد في قواعد البيانات المحلية بدون كلمة مرور).
