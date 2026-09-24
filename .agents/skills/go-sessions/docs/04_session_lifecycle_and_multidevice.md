# 04. دورة حياة الجلسة وإدارتها متعددة الأجهزة (`Session Lifecycle & Multi-Device Governance`)

تُعد حوكمة دورة حياة الجلسة وإدارة وجود المستخدم عبر أجهزته المتعددة من أهم المتطلبات الأمنية والتنظيمية في التطبيقات الحديثة. يشرح هذا الدليل كيفية تطبيق هذه المفاهيم في مشاريع Go الموزعة.

---

## 1. استراتيجية الانتهاء المزدوجة (Dual Timeout Policy)

وفقاً لإرشادات **OWASP** و **NIST SP 800-63B**، يجب ألا تعتمد الجلسة على مهلة واحدة، بل على نظام مهلتين متكاملتين:

```text
Session Created (t0) ────────────────────────────────────────────────> Absolute Timeout (t0 + 12h)
       │                                                                      ▲
       ├── Request 1 (t0 + 10m)  ──> Extend Idle (now + 30m)                  │
       ├── Request 2 (t0 + 25m)  ──> Extend Idle (now + 30m)                  │
       └── Idle for 35m          ──> Session Expired by Inactivity!           │
                                                                              │
(حتى لو ظل المستخدم نشطاً باستمرار، تنتهي الجلسة قسراً عند الوصول إلى المهلة المطلقة)
```

1. **مهلة الخمول (Idle / Inactivity Timeout):**
   - المدة الموصى بها: **15 إلى 30 دقيقة**.
   - تتمدد الجلسة مع كل نشاط جديد للمستخدم، وتنتهي إذا توقف عن إرسال الطلبات خلال هذه المدة.
2. **المهلة المطلقة (Absolute Hard Cap Timeout):**
   - المدة الموصى بها: **8 إلى 24 ساعة** (أو أيام محدودة للجلسات الممتدة).
   - سقف زمني نهائي صارم يبدأ من لحظة تسجيل الدخول الأصلية، وتنتهي الجلسة عنده حتماً وتجبر المستخدم على إعادة تسجيل الدخول حتى لو كان نشطاً في تلك اللحظة.

---

## 2. منع هجوم تثبيت الجلسة (Session Fixation Prevention)

- **الهجوم:** يستغل المهاجم معرف جلسة غير مصدق (Anonymous Session ID)، ويجعل الضحية تستخدمه لتسجيل الدخول. إذا لم يتغير المعرف، يظل المهاجم قادراً على الوصول لحساب الضحية.
- **الحل:** تجديد المعرف بالكامل عبر دالة `RegenerateToken` فور نجاح عملية تسجيل الدخول، أو ترقية الصلاحيات، أو تغيير كلمة المرور:

```go
func (m *Manager) RegenerateToken(ctx context.Context, w http.ResponseWriter, r *http.Request) error {
 oldRawToken := m.extractToken(r)
 if oldRawToken == "" {
  return ErrSessionNotFound
 }
 oldHash := HashToken(oldRawToken)

 data, err := m.store.Get(ctx, oldHash)
 if err != nil {
  return err
 }

 // توليد معرف جديد كلياً
 newRawToken, newHash, err := GenerateSessionTokens()
 if err != nil {
  return err
 }

 // نقل البيانات للمعرف الجديد وحذف المعرف القديم ذرياً
 ttl := time.Until(data.AbsoluteExp)
 if ttl > m.config.IdleTimeout {
  ttl = m.config.IdleTimeout
 }

 if err := m.store.Set(ctx, newHash, data, ttl); err != nil {
  return err
 }
 _ = m.store.Delete(ctx, oldHash)

 // كتابة الكوكي الجديد
 WriteSessionCookie(w, m.config.CookieName, newRawToken, ttl, m.config.CookieSecure)
 return nil
}
```

---

## 3. معمارية الفهرسة الثانوية للأجهزة (Secondary Indexing in Redis)

للتمكن من استعراض الأجهزة النشطة وطرد جلسات محددة، نعتمد نمط الفهرس الثانوي في Redis:

```text
Session Data Key:    "sess:<token_hash>"       -> Hash / JSON of session attributes
User's Sessions Set: "user_sessions:<user_id>" -> Set of token_hashes
```

### العمليات الأساسية

1. **تسجيل دخول جهاز جديد:**
   - إضافة الجلسة في `sess:<hash>`.
   - إضافة المعرف إلى مجموعة المستخدم: `SADD user_sessions:<user_id> <hash>`.
2. **عرض جميع الأجهزة النشطة:**
   - استعلام أعضاء المجموعة: `SMEMBERS user_sessions:<user_id>`.
   - جلب بيانات كل جلسة متوفرة (مع استبعاد المعرفات المنتهية).
3. **تسجيل الخروج من جميع الأجهزة الأخرى:**
   - استرجاع كافة أعضاء المجموعة.
   - حذف جميع المفاتيح ما عدا الهاش الخاص بالجلسة الحالية.
   - إبقاء الجلسة الحالية فقط في المجموعة.

---

## 4. تقييد عدد الجلسات المتزامنة (Concurrent Session Limits)

عند تحديد عدد أقصى للجلسات (مثلاً: 3 أجهزة متزامنة):

- عند محاولة تسجيل دخول جهاز رابع:
  - استخراج أقدم جلسة بناءً على `CreatedAt` أو `LastActiveAt`.
  - حذف أقدم جلسة من Redis ومحوها من مجموعة المستخدم (FIFO Eviction).
  - إخطار الجلسة القديمة أو طردها فورياً عند طلبها التالي.
