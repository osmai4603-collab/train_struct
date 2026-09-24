# 03. المقارنات العميقة: go-cmp مقابل testify

## 1. فلسفة المقارنة: لماذا تفضل Google `google/go-cmp`؟

في بيئات تطوير الأنظمة الكبيرة، تفرض Google والعديد من المؤسسات الهندسية المعيار التالي:
> **"Avoid assertion libraries in favor of standard if-statements and google/go-cmp."**

### الأسباب الهندسية الأساسية

1. **وضوح رسائل الفشل الفوري:** في مكتبات مثل `testify`، ينتج الخطأ: `assert.Equal(t, a, b)` كرسالة غامضة: `Not equal: 1 != 2`، دون توضيح اسم الحقل أو سياق المشكلة.
2. **فروقات نصية واضحة كـ Git Diff:** عند مقارنة كائنات معقدة تحتوي على عشرات الحقول، ينتج `cmp.Diff` مخرجات توضح بالضبط الحقل المختلف دون الحاجة لفحص يدوي.
3. **التحكم الهندسي في المقارنات:** عبر حزمة `cmpopts`، يمكن تجاهل حقول الوقت العشوائية (`time.Now()`) والمعرفات الفريدة (`UUID`) دون الحاجة لتعديل الكود الأصلي.

---

## 2. الدليل العملي لمكتبة `google/go-cmp`

```go
package orders_test

import (
 "testing"
 "time"

 "github.com/google/go-cmp/cmp"
 "github.com/google/go-cmp/cmp/cmpopts"
)

type OrderItem struct {
 SKU      string
 Quantity int
 Price    float64
}

type Order struct {
 ID        string
 Customer  string
 Items     []OrderItem
 CreatedAt time.Time
 secretKey string // حقل غير مصدّر (unexported)
}

func TestCreateOrder(t *testing.T) {
 got := CreateOrder("cust_123", []OrderItem{
  {SKU: "A1", Quantity: 2, Price: 15.5},
  {SKU: "B2", Quantity: 1, Price: 40.0},
 })

 want := &Order{
  ID:        "", // سيتم تجاهله
  Customer:  "cust_123",
  Items: []OrderItem{
   {SKU: "B2", Quantity: 1, Price: 40.0}, // الترتيب مختلف عمداً
   {SKU: "A1", Quantity: 2, Price: 15.5},
  },
  CreatedAt: time.Now(), // سيتم تجاهله
  secretKey: "internal_hash",
 }

 // تكوين خيارات المقارنة الاحترافية
 opts := cmp.Options{
  // 1. تجاهل الحقول الديناميكية المولدة عشوائياً
  cmpopts.IgnoreFields(Order{}, "ID", "CreatedAt"),

  // 2. السماح بمقارنة الحقول غير المصدرة في الهيكل
  cmp.AllowUnexported(Order{}),

  // 3. تجاهل ترتيب العناصر في الشريحة إذا لم يكن الترتيب جوهرياً
  cmpopts.SortSlices(func(a, b OrderItem) bool {
   return a.SKU < b.SKU
  }),

  // 4. معاملة الشرائح الفارغة والـ nil بالتساوي
  cmpopts.EquateEmpty(),

  // 5. مقارنة الأرقام العشرية مع هامش خطأ مسموح
  cmpopts.EquateApprox(0.001, 0),
 }

 if diff := cmp.Diff(want, got, opts...); diff != "" {
  t.Errorf("CreateOrder() mismatch (-want +got):\n%s", diff)
 }
}
```

---

## 3. الضوابط الصارمة عند استخدام `testify`

إذا اعتمد مشروعك على `github.com/stretchr/testify`، يجب الالتزام بالقواعد التالية لمنع الأخطاء الشائعة:

### 3.1 الفرق الحاسم بين `require` و `assert`

- **`require` للشروط المسبقة الحتمية:** التي يستحيل إكمال الاختبار بدونها (مثل نجاح الاتصال بقاعدة البيانات، أو عدم وجود خطأ، أو التحقق من أن المؤشر ليس `nil`).
- **`assert` للتحقق من القيم والخصائص:** لكي تستمر بقية الفحوصات في حال فشل حقل معين، مما يوفر تشخيصاً كاملاً.

```go
func TestUserFetch(t *testing.T) {
    user, err := repo.GetUser(ctx, "usr_1")

    // شرط مسبق: يمنع وقوع Panic في السطر التالي
    require.NoError(t, err, "failed to query repository")
    require.NotNil(t, user, "user pointer must not be nil")

    // فحوصات حالة عادية
    assert.Equal(t, "usr_1", user.ID, "user ID mismatch")
    assert.Equal(t, "active", user.Status, "user status should be active")
}
```

### 3.2 توفير رسائل سياقية مخصصة

لا تترك رسائل التأكيد فارغة:

```go
// ❌ خطأ: رسالة غامضة
assert.Equal(t, 200, res.StatusCode)

//  صحيح: رسالة تشخيصية محددة
assert.Equal(t, 200, res.StatusCode, "expected successful status code for healthcheck endpoint")
```
