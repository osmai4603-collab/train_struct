# 01. المعمارية المتقدمة وعزل نطاق الفشل (Architecture & Failure Domains)

> **الهدف المعماري:** بناء نظام أخطاء متعدد الطبقات يمنع تسريب تفاصيل التنفيذ الخارجي، ويفصل بصرامة بين احتياجات المستهلكين المختلفين للخطأ.

---

## نمط Ben Johnson: المستهلكون الثلاثة للخطأ

في التطبيقات الموزعة وتطبيقات الخدمات الخلفية، لا يوجد مستهلك وحيد للخطأ. نمط Ben Johnson (*Failure is your Domain*) يحدد 3 أطراف مستقلة:

```text
┌───────────────────────────────────────────────────────────────────┐
│                        المستهلكون الثلاثة للخطأ                    │
├───────────────────┬───────────────────────────┬───────────────────┤
│    1. التطبيق     │    2. المستخدم النهائي    │    3. المشغل      │
│  (The Application)│      (The End User)       │  (The Operator)   │
├───────────────────┼───────────────────────────┼───────────────────┤
│ • Code ثابت       │ • رسالة إنسانية واضحة     │ • مسار منطقي Op   │
│ • قرارات برمجية   │ • خالية من التفاصيل الفنية│ • الخطأ الأصلي Err│
│ • تعيين HTTP Status│ • لا تسريب لقواعد البيانات│ • معرّف الطلب     │
└───────────────────┴───────────────────────────┴───────────────────┘
```

### 1. التطبيق والمنطق البرمجي (`Code`)

يحتاج التطبيق إلى معرفة "طبيعة الفشل" دون فك نصوص الرسائل البرمجية. على سبيل المثال:

- إذا كان الفشل `NOT_FOUND`، يحوله الـ HTTP Handler إلى `404`.
- إذا كان `CONFLICT`، يحوله إلى `409` أو يتخذ مسار إعادة المحاولة.
- إذا كان `INVALID`، يرفض الطلب فوراً بـ `400`.

### 2. المستخدم النهائي (`Message`)

المستخدم النهائي لا يجب أبداً أن يرى رسائل مثل:
`pq: duplicate key value violates unique constraint "users_email_key"`
أو:
`dial tcp 10.0.1.5:5432: i/o timeout`
بل يحتاج إلى رسائل واضحة ومحددة مثل:
`an account with this email address already exists`
أو:
`the requested service is temporarily unavailable`

### 3. المهندس والمشغل (`Op`, `Err`, `RequestID`)

يحتاج المشغل إلى معرفة المسار الدقيق الذي مر به الطلب حتى الفشل (`Logical Stack Trace`)، والسبب الجذري الفعلي (Root Cause)، بالإضافة إلى `RequestID` لمطابقة الخطأ مع السجلات الموزعة.

---

## عزل نطاق الفشل في طبقة التخزين (Repository Failure Domain)

في المعمارية النظيفة (Clean Architecture)، يجب ألا تتسرب أخطاء المكتبات الخارجية (PostgreSQL driver، Redis client، AWS SDK) إلى طبقات التطبيق العليا.

```text
❌ تسريب البنية التحتية (ممنوع):
HTTP Handler ◄── UseCase ◄── Repository ◄── sql.ErrNoRows (تسرب تفاصيل التخزين)

✅ عزل نطاق الفشل (صحيح):
HTTP Handler ◄── UseCase ◄── Repository (ترجمة الخطأ إلى platformerr.NotFound)
```

### مثال عملي للترجمة في مستودع البيانات

```go
package storage

import (
    "context"
    "database/sql"
    "errors"

    platformerr "train/internal/platform/errors"
)

type PostgresOrderRepository struct {
    db *sql.DB
}

func (r *PostgresOrderRepository) GetOrderByID(ctx context.Context, orderID string) (*Order, error) {
    const op = "storage.PostgresOrderRepository.GetOrderByID"

    row := r.db.QueryRowContext(ctx, "SELECT id, total FROM orders WHERE id = $1", orderID)
    
    var order Order
    if err := row.Scan(&order.ID, &order.Total); err != nil {
        if errors.Is(err, sql.ErrNoRows) {
            // ترجمة الخطأ إلى خطأ نطاق آمن ومفهوم
            return nil, platformerr.NotFound(op, "order not found", err)
        }
        // الأخطاء الفنية الأخرى تُحجب تفاصيلها عن العميل وتُسجل للمشغل
        return nil, platformerr.Internal(op, "failed to query order database", err)
    }

    return &order, nil
}
```
