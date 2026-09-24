# 05. إدارة التزامن وكبح كتابة النشاط (`Concurrency & Write Throttling`)

تتعامل خوادم Go المبنية على `net/http` مع كل طلب وارد في **Goroutine منفصلة**. يؤدي هذا التوازي بطبيعته إلى تحديات أداء وتزامن حادة عند إدارة الجلسات في بيئات الإنتاج عالية التردد (High QPS). يشرح هذا الدليل كيفية معالجة هذه المعضلات.

---

## 1. معضلة "آخر كتابة تفوز" (Last-Write-Wins Race Hazard)

عندما يفتح المستخدم عدة تبويبات أو يرسل المتصفح طلبات متزامنة (Parallel AJAX Calls):

```text
Browser (2 Concurrent Requests)
   │
   ├── Request A ──> [Goroutine 101] ──> Read Session (Cart: 2 items) ──> Add Item ──> Write (3 items)
   │
   └── Request B ──> [Goroutine 102] ──> Read Session (Cart: 2 items) ──> Set Flash ──> Write (Flash Set)
```

إذا حفظت Goroutine 102 نسختها بعد Goroutine 101، فإنها ستكتب فوق بيانات العربة القديمة وتلغي إضافة العنصر الثالث!

### الحلول المعمارية في Go

1. **فصل حالة التطبيق عن الجلسة:** جعل الجلسة مقتصرة فقط على بيانات المصادقة الصارمة (`UserID`, `Roles`, `CSRFToken`)، وتخزين سلة التسوق والبيانات المتغيرة في جداول قاعدة البيانات المخصصة مع معاملات ACID.
2. **التعديل الذري للحقول (Atomic Field Mutation):** استخدام Redis Hashes (`HSET` / `HGET`) لتعديل الحقل المعني فقط بدلاً من استبدال كائن الجلسة بالكامل.
3. **القفل التفاؤلي (Optimistic Locking):** استخدام رقم إصدار للجلسة والتحقق منه قبل التحديث.

---

## 2. تقنية كبح كتابة وقت النشاط (Rolling Expiration Write Throttling)

في الأنظمة التي تستقبل آلاف الطلبات في الثانية (High-Throughput Services)، يؤدي تحديث وقت نشاط الجلسة (`LastActiveAt`) وإعادة ضبط مهلة Redis TTL مع كل طلب HTTP منفرد إلى:

- اختناق اتصالات Redis (Connection Pool Saturation).
- زيادة غير مبررة في زمن استجابة الـ API.

### الاستراتيجية المعتمدة عالمياً

نقوم بتمديد وقت الجلسة وتحديث المتجر فقط إذا انقضت فترة محددة (مثل **دقيقة واحدة** أو **ربع مدة الخمول**) منذ آخر تحديث مسجل:

```go
package session

import (
 "context"
 "time"
)

const DefaultWriteThrottleInterval = 1 * time.Minute

// ShouldThrottleWrite يتحقق مما إذا كان تحديث وقت النشاط يجب تأجيله
func ShouldThrottleWrite(lastActive time.Time, interval time.Duration) bool {
 if interval <= 0 {
  interval = DefaultWriteThrottleInterval
 }
 return time.Since(lastActive) < interval
}

// UpdateActivityWithThrottling يُحدث وقت الجلسة فقط عند تجاوز فاصل الكبح
func (m *Manager) UpdateActivityWithThrottling(ctx context.Context, tokenHash string, data *SessionData) error {
 now := time.Now().UTC()

 // إذا لم ينقضِ فاصل الكبح، نتجاوز الكتابة إلى Redis تماماً
 if ShouldThrottleWrite(data.LastActiveAt, time.Minute) {
  return nil
 }

 data.LastActiveAt = now
 remainingTTL := time.Until(data.AbsoluteExp)
 if remainingTTL > m.config.IdleTimeout {
  remainingTTL = m.config.IdleTimeout
 }

 return m.store.Set(ctx, tokenHash, data, remainingTTL)
}
```

---

## 3. العمليات الذرية عبر Redis Lua Scripts

لضمان سلامة العمليات المعقدة (مثل تجديد الرمز أو استهلاك الرموز أحادية الاستخدام)، تُنفذ العمليات عبر سكربت Lua لضمان تنفيذها كوحدة ذرية واحدة داخل Redis:

```lua
-- refresh session script
local oldKey = KEYS[1]
local newKey = KEYS[2]
local userSetKey = KEYS[3]
local sessionData = ARGV[1]
local ttlSeconds = ARGV[2]
local oldHash = ARGV[3]
local newHash = ARGV[4]

-- التحقق من وجود الجلسة القديمة
if redis.call("EXISTS", oldKey) == 0 then
    return 0
end

-- حفظ الجلسة الجديدة
redis.call("SET", newKey, sessionData, "EX", ttlSeconds)

-- حذف الجلسة القديمة
redis.call("DEL", oldKey)

-- تحديث مجموعة جلسات المستخدم
redis.call("SREM", userSetKey, oldHash)
redis.call("SADD", userSetKey, newHash)

return 1
```
