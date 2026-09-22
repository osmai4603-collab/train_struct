# التحقق الصارم الاستباقي (Fail-Fast Multi-Error Validation)

من أخطر العيوب في تصميم الخدمات السحابية هو أن يقلع الخادم بنجاح مع وجود خطأ خفي في الإعدادات، ليكتشف المستخدمون بعد ساعات أن الاتصال معطل أو أن مهلة الإغلاق تساوي صفراً مما يؤدي لقطع الطلبات قسراً.

---

## فلسفة الفشل الاستباقي (Fail-Fast Philosophy)

يجب أن تخضع الإعدادات لفحص شامل وصارم في **المرحلة الأولى من دورة حياة الخادم (Phase 1: Initialization)** قبل فتح أي منفذ شبكة (`net.Listen`) وقبل الاتصال بقاعدة البيانات. في حال وجود أي خطأ، تتوقف العملية فوراً برمز فشل `os.Exit(1)`.

---

## معضلة الفشل عند أول خطأ (Fail-on-First-Error Problem)

العديد من المطورين يكتبون دوال التحقق بهذا الشكل:

```go
// ❌ نهج سيئ ومحبط للمشغلين
if cfg.Port == "" { return errors.New("missing port") }
if cfg.DBHost == "" { return errors.New("missing db host") }
if cfg.JWTSecret == "" { return errors.New("missing jwt secret") }
```

- **المشكلة**: يحاول المهندس تشغيل التطبيق فيفشل عند `missing port`، فيصلحها ويعيد التشغيل ليفاجأ بـ `missing db host`، فيصلحها ليفاجأ بـ `missing jwt secret`!
- **الحل المعتمد**: **مجمّع الأخطاء المتعددة (Multi-Error Accumulator)**.

---

## آلية تجميع الأخطاء (Error Accumulation Engine)

يقوم محرك التحقق بفحص كافة الحقول وتجميع كافة الأخطاء المكتشفة في تقرير واحد منسق وواضح:

```go
type ErrorReport struct {
    errors []string
}

func (r *ErrorReport) Add(field, reason string) {
    r.errors = append(r.errors, fmt.Sprintf("  - %-25s : %s", field, reason))
}

func (r *ErrorReport) Err() error {
    if len(r.errors) == 0 {
        return nil
    }
    return fmt.Errorf("configuration validation failed with %d errors:\n%s",
        len(r.errors), strings.Join(r.errors, "\n"))
}
```

### القواعد الإلزامية للفحص (Invariants Checklist)

1. **المنافذ**: التحقق من أن رقم المنفذ يقع بين 1 و 65535.
2. **عناوين الشبكة**: التحقق من صحة صياغة الـ IP أو اسم المضيف.
3. **قواعد البيانات**: إذا كان المحرك `postgres`، يجب التأكد من تحديد المضيف واسم القاعدة والمستخدم.
4. **المهلات والعلاقات الزمنية**:
   - `ReadTimeout > 0` و `WriteTimeout > 0`.
   - علاقة التصريف: التأكد دائماً من أن `ShutdownTimeout > DrainDuration` لضمان تصريف الزيارات قبل الإغلاق القسري.
5. **طول الأسرار**: التأكد من أن مفاتيح التوقيع الرقمي (JWT Secrets) لا تقل عن 32 بايت في بيئات الإنتاج لمنع هجمات التخمين.
