# مصفوفة الأنماط المضادة وتصحيحها في Go Context (Anti-Patterns Matrix)

| # | النمط المضاد (Anti-Pattern) | المخاطر المترتبة في الإنتاج | الكود الخاطئ | الكود المعياري المعتمد |
|:---|:---|:---|:---|:---|
| 1 | **تخزين السياق في Struct** | سياق قديم منتهي، تداخل بين الطلبات المتزامنة، فقدان أمان التزامن | `type Svc struct { ctx context.Context }` | تمرير `ctx` كأول معامل صراحة: `func (s *Svc) Run(ctx context.Context)` |
| 2 | **تمرير `nil` كقيمة سياق** | انهيار التطبيق الفوري بـ `panic: runtime error: nil pointer` | `client.Do(nil, data)` | تمرير `context.Background()` أو `context.TODO()` |
| 3 | **إهمال استدعاء `cancel()`** | تسريب مؤقتات النظام وتراكم عقد الذاكرة (Timer & Memory Leaks) | `ctx, _ = context.WithTimeout(p, 5*time.Second)` | `ctx, cancel := context.WithTimeout(...)`<br>`defer cancel()` |
| 4 | **تمرير `r.Context()` لمهمة خلفية** | إلغاء المهمة فور خروج الدالة وإرسال رد الـ HTTP للمتصفح | `go mailer.Send(r.Context(), mail)` | استخدام `context.WithoutCancel(r.Context())` مع مهلة جديدة |
| 5 | **استخدام `string` كمفتاح في `WithValue`** | تصادم المفاتيح بين الحزم المختلفة واستبدال البيانات خفية | `ctx = context.WithValue(ctx, "user_id", id)` | نوع هيكل فارغ غير مصدّر: `type key struct{}` |
| 6 | **حقن التبعيات عبر `WithValue`** | نمط Service Locator سيئ السمعة، اختفاء التبعيات، كسر التحقق النوعي | `db := ctx.Value("db").(*sql.DB)` | حقن التبعية في باني الخدمة: `NewService(db *sql.DB)` |
| 7 | **تظليل المتغيرات (Shadowing)** | استخدام السياق غير المقيد بالمهلة خارج نطاق كتلة الـ `if` | `if cond { ctx, cancel := context.WithTimeout(...) }` | `var cancel context.CancelFunc`<br>`ctx, cancel = context.WithTimeout(...)` |
| 8 | **تجاهل `<-ctx.Done()` في الحلقات** | تسريب الـ Goroutine وتعليقه في الذاكرة للأبد عند توقف القناة | `for msg := range ch { ... }` | استخدام `select` مع `case <-ctx.Done(): return ctx.Err()` |
| 9 | **حقن سمات منفصلة $O(N)$** | بطء استرجاع القيم وزيادة استهلاك الذاكرة عبر قائمة مترابطة طويلة | استدعاء `WithValue` 20 مرة في مسار الطلب | نمط التجميع: `WithRequestMetadata(ctx, &Metadata{...})` |
| 10 | **استخدام دوال `database/sql` القديمة** | استمرار الاستعلامات الباهظة في محرك الـ DB رغم قطع العميل للاتصال | `db.Query("SELECT ...")` | استخدام توابع السياق: `db.QueryContext(ctx, "SELECT ...")` |
