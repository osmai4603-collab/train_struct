# 03. هندسة الأداء العالي ومسارات Hot Paths (Performance & Zero-Allocations)

> **المصادر المرجعية:** Go Official Blog (*Performance section in slog*), Go Runtime Profiling & Allocations analysis.

---

## 1. مشكلة تخصيص الذاكرة (Memory Allocations) في السجلات

تُعد السجلات أحد أكثر المصادر الخفية لاستهلاك الذاكرة وضغط جامع المهملات (Garbage Collector) في خوادم Go عالية الكثافة (High-Throughput Services).

عند كتابة:
```go
logger.Info("request handled", "method", r.Method, "status", status, "duration", d)
```
تستقبل الدالة المعلمات بصيغة `...any`. تحويل القيم البسيطة (مثل `int`, `time.Duration`, `string`) إلى واجهة فارغة `any` (`interface{}`) يجبر بيئة تشغيل Go على إجراء **Boxing** وتخصيص كائنات على كومة الذاكرة (Heap Allocations).

---

## 2. تقنية `slog.LogAttrs` للصفر تخصيصات (Zero-Allocations)

في المسارات الساخنة (Hot Paths) التي يتم استدعاؤها آلاف المرات في الثانية (مثل معالجات الشبكة والحزم والعمليات المالية)، يُنصح بشدة باستخدام الدالة `slog.LogAttrs` مع دوال بناء السمات ذات الأنواع الصريحة:

```go
// ❌ استهلاك ذاكرة إضافي في المسارات الساخنة:
logger.InfoContext(ctx, "network packet processed",
    "bytes", n,
    "source", srcIP,
    "elapsed", latency,
)

// ✅ أداء فائق بدون أي تخصيص على الـ Heap (Zero Allocations):
logger.LogAttrs(ctx, slog.LevelInfo, "network packet processed",
    slog.Int("bytes", n),
    slog.String("source", srcIP),
    slog.Duration("elapsed", latency),
)
```

### قائمة دوال البناء الصريحة الشائعة:
- `slog.String(key, val)`
- `slog.Int(key, val)`, `slog.Int64(key, val)`
- `slog.Uint64(key, val)`
- `slog.Float64(key, val)`
- `slog.Bool(key, val)`
- `slog.Duration(key, val)`
- `slog.Time(key, val)`
- `slog.Any(key, val)` (يُستخدم فقط إذا كان النوع مركباً وغير متوفر له دالة صريحة)

---

## 3. التحقق المسبق عبر `logger.Enabled()` للعمليات المكلفة

إذا كان تسجيل الحدث يتطلب حسابات رياضية معقدة، أو تسطيح شجرة بيانات (Tree Traversal)، أو التقاط لقطة حالة (Snapshot):

```go
// ❌ خطأ فادح: يتم تنفيذ الدالة expensiveStateSnapshot() حتى لو كان المستوى Debug معطلاً!
logger.DebugContext(ctx, "system snapshot", "state", expensiveStateSnapshot())

// ✅ الأمثل: التحقق السريع من المستوى قبل دفع تكلفة الحساب
if logger.Enabled(ctx, slog.LevelDebug) {
    logger.DebugContext(ctx, "system snapshot", "state", expensiveStateSnapshot())
}
```

دالة `Enabled()` تقوم بفحص ذري بسيط وسريع جداً لمستوى السجل الحالي، وتتجنب أي حسابات إذا كان المستوى معطلاً في البيئة الحالية.

---

## 4. المعالجة المسبقة للسمات المتكررة عبر `logger.With()`

عند تكرار نفس السمات في عدة أسطر متتالية، استخدم `logger.With()` لحساب التنسيق الداخلي مرة واحدة وإعادة استخدامه:

```go
// ❌ تكرار تنسيق المفاتيح في كل سطر
logger.InfoContext(ctx, "job starting", "job_id", j.ID, "tenant", j.Tenant)
logger.InfoContext(ctx, "job in progress", "job_id", j.ID, "tenant", j.Tenant)
logger.InfoContext(ctx, "job completed", "job_id", j.ID, "tenant", j.Tenant)

// ✅ احتساب وبناء مسبق:
jobLogger := logger.With(
    slog.String("job_id", j.ID),
    slog.String("tenant", j.Tenant),
)
jobLogger.InfoContext(ctx, "job starting")
jobLogger.InfoContext(ctx, "job in progress")
jobLogger.InfoContext(ctx, "job completed")
```

---

## 5. قاعدة الحلقات الضيقة (Tight Loops Rule)

> **قاعدة ذهبية:** يُحظر وضع استدعاءات السجلات داخل حلقات التكرار السريعة (`for i := 0; i < 1_000_000; i++`).

الطباعة في السجلات داخل الحلقات الضيقة تؤدي إلى:
1. اختناق كامل في عمليات الإدخال والإخراج (I/O Bottleneck).
2. إغراق أنظمة المراقبة بملايين السجلات المتطابقة.
3. استهلاك غير مبرر للذاكرة والمعالج.

**البديل:** سجّل بداية الدفعة، وأي أخطاء فردية فقط، وملخصاً مجمعاً عند انتهاء الدفعة.
