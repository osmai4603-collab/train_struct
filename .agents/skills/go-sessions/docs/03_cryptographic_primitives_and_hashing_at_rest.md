# 03. التوليد التشفيري والتجزئة الآمنة عند التخزين (`Cryptographic Primitives & Hashing at Rest`)

تعتمد سلامة نظام الجلسات على استحالة تخمين معرف الجلسة، ومناعة النظام ضد تسريب الجلسات النشطة في حال اختراق وسيط التخزين. يوضح هذا الدليل المبادئ التشفيرية لتوليد وتخزين معرفات الجلسات في مشاريع Go المتقدمة.

---

## 1. حساب الإنتروبيا والتوليد العشوائي (Entropy & CSPRNG)

توصي معايير **NIST SP 800-63B** و **OWASP** بألا تقل درجة العشوائية التشفيرية (Entropy) لمعرف الجلسة عن **128 بت**، ويُعتمد معيار **256 بت (32 بايت)** كأفضل ممارسة في الأنظمة الكبيرة:

$$\text{Search Space} = 2^{256} \approx 1.15 \times 10^{77} \text{ possibilities}$$

### الحظر الصارم لـ `math/rand`

- خوارزميات `math/rand` و `math/rand/v2` حتمية وتعتمد على بذور أولية (PRNGs)؛ يمكن للمهاجم التنبؤ بالمعرفات بمجرد تخمين البذرة.
- يجب استخدام `crypto/rand` حصراً، حيث يتصل بمصدر العشوائية التابع لنظام التشغيل (`/dev/urandom` أو `getrandom(2)`).

```go
func GenerateRandomBytes(length int) ([]byte, error) {
 b := make([]byte, length)
 if _, err := io.ReadFull(rand.Reader, b); err != nil {
  return nil, fmt.Errorf("failed to read secure random bytes: %w", err)
 }
 return b, nil
}
```

---

## 2. معمارية تجزئة المعرف عند التخزين (Session ID Hashing at Rest)

في الشركات الرائدة عالمياً (GitHub, Stripe, Shopify)، **لا يُخزن معرّف الجلسة الخام أبداً في قاعدة البيانات أو Redis!**

```text
┌─────────────────┐       Raw Token (32 bytes Hex / Base64URL) ┌─────────────────────────┐
│ Browser Client  │ ──────────────────────────────────────────> │       Go API Server     │
└─────────────────┘                                             └───────────┬─────────────┘
                                                                            │
                                                                   SHA-256(Raw Token)
                                                                            │
                                                                            ▼
                                                                ┌─────────────────────────┐
                                                                │ Session Store (Redis)   │
                                                                │ Key: "sess:<hex_hash>"  │
                                                                └─────────────────────────┘
```

### الأثر الأمني

- **في حال تسريب قاعدة البيانات أو كاش Redis (Data Dump Leakage):** يجد المخترق أمامه فقط قيم تجزئة `SHA-256`. ونظراً لأن دالة التجزئة أحادية الاتجاه وغير قابلة للعكس، يعجز المهاجم عن صياغة كوكي جلسة صالح للاستخدام، مما يحمي كافة حسابات المستخدمين النشطة من الاختطاف.

```go
package session

import (
 "crypto/rand"
 "crypto/sha256"
 "encoding/hex"
 "io"
)

// GenerateSessionTokens يولد الرمز الخام والهاش المقابل للتخزين
func GenerateSessionTokens() (rawToken string, tokenHash string, err error) {
 bytes := make([]byte, 32) // 256 bits of entropy
 if _, err := io.ReadFull(rand.Reader, bytes); err != nil {
  return "", "", err
 }

 rawToken = hex.EncodeToString(bytes)
 tokenHash = HashToken(rawToken)
 return rawToken, tokenHash, nil
}

// HashToken يُحول الرمز الخام إلى هاش SHA-256 للبحث في المتجر
func HashToken(rawToken string) string {
 h := sha256.Sum256([]byte(rawToken))
 return hex.EncodeToString(h[:])
}
```

---

## 3. المقارنة بالوقت الثابت لمنع هجمات التوقيت (Constant-Time Verification)

عند مقارنة أي رموز أمنية (مثل رمز الـ CSRF المخزن بالجلسة مع الرمز الوارد من العميل)، فإن المقارنة العادية `==` تتوقف عند أول بايت غير متطابق. هذا يتيح للمهاجم قياس الفارق الزمني بالميكروثانية واستنتاج الرمز تدريجياً (Side-Channel Timing Attack).

**الحل:** استخدام دالة المقارنة بالوقت الثابت `crypto/subtle.ConstantTimeCompare`:

```go
import "crypto/subtle"

func ValidateCSRFToken(clientToken, sessionToken string) bool {
 if len(clientToken) == 0 || len(sessionToken) == 0 {
  return false
 }
 return subtle.ConstantTimeCompare([]byte(clientToken), []byte(sessionToken)) == 1
}
```
