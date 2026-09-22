# قائمة فحص الجاهزية للإنتاج (Production Readiness Checklist)

استخدم هذه القائمة لمراجعة أي خدمة أو مستودع يتعامل مع PostgreSQL للتأكد من استيفاء أعلى معايير الاستقرار والأمان:

## 1. إعدادات المجمع والأداء (Pool Tuning & Performance)

- [ ] تم تحديد `MaxConns` و `MinConns` وفقاً لمعادلة أبعاد الخادم وليس عشوائياً.
- [ ] تم ضبط `MaxConnLifetime` (مثلاً `1h`) لتجنب تسرب الذاكرة ولتوزيع الحمل على النسخ المتماثلة.
- [ ] تم تفعيل `HealthCheckPeriod` (مثلاً `30s`) لفحص الاتصالات الخاملة في الخلفية.
- [ ] يتم فحص الاتصال الأولي عبر `pool.Ping(ctx)` لضمان الفشل الفوري (`Fail-Fast`) عند عدم توفر قاعدة البيانات.
- [ ] يتم إغلاق المجمع بأمان عند انتهاء الخدمة عبر `postgres.ClosePool(pool, logger)`.

## 2. عزل نطاق الفشل والأمان (Failure Domains & Security)

- [ ] تمر كافة أخطاء الاستعلامات والمستودعات عبر `postgres.TranslateError(op, err)`.
- [ ] لا يتم تسريب أي كائن `*pgconn.PgError` أو نص استعلام SQL إلى طبقة الـ HTTP أو رسائل العميل.
- [ ] نص الاتصال وكلمة المرور محجوبة في السجلات المشتركة باستخدام `cfg.RedactedDSN()`.
- [ ] نمط الاتصال المشفر `sslmode=require` أو `sslmode=verify-full` مفعل في بيئات الإنتاج والسحاب.

## 3. المعاملات وإعادة المحاولة (Transactions & Resilience)

- [ ] تُنفذ كافة العمليات متعددة الاستعلامات الذرية عبر `postgres.ExecTx` أو `postgres.ExecTxWithRetry`.
- [ ] تم التأكد من أن الدوال الممررة لـ `ExecTxWithRetry` هي دوال آمنة التكرار (`Idempotent`).
- [ ] إعادة المحاولة محصورة بالأخطاء العابرة فقط (`IsTransient`) مثل `40001` (Serialization) و `40P01` (Deadlock).
- [ ] خوارزمية إعادة المحاولة تحترم سياق الإلغاء `context.Context` وتطبق التشويش الكامل (`Full Jitter`).

## 4. المخططات وفحص الجاهزية (Migrations & Probing)

- [ ] ملفات الترحيل منظمة بتسلسل زمني ثابت (Timestamps) ومتطابقة بين `.up.sql` و `.down.sql`.
- [ ] يتم تشغيل الترحيلات تلقائياً عند الإقلاع عبر `postgres.RunMigrations` المضمنة عبر `go:embed`.
- [ ] تم ربط فحص الصحة `checker.Check(ctx)` بنقطة النهاية `/readyz` لمراقبي Kubernetes.
- [ ] مقاييس المجمع `checker.Stats()` مفعلة ومراقبة لملاحظة مؤشرات الاختناق (`EmptyAcquireCount`).
