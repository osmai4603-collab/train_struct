# Event-Driven Architecture (EDA) — أفضل الممارسات العالمية في Go للمشاريع الكبيرة

> **تاريخ البحث:** 2026-09-25
>
> **المصادر:** مواقع رسمية ومرجعية (go.dev, threedots.tech, watermill.io, github.com, cloudevents.io,
> temporal.io, conduktor.io, وغيرها من المصادر الموثوقة).
>
> **النطاق:** جميع أنواع ومستويات EDA — من In-Process إلى Distributed، من Pub/Sub إلى Event Sourcing + CQRS + Saga.

---

## جدول المحتويات

1. [نظرة عامة — ما هي EDA ولماذا Go؟](#1-نظرة-عامة)
2. [المستوى الأول: In-Process Event Bus](#2-المستوى-الأول-in-process-event-bus)
3. [المستوى الثاني: Pub/Sub الموزع](#3-المستوى-الثاني-pubsub-الموزع)
4. [المستوى الثالث: CQRS](#4-المستوى-الثالث-cqrs)
5. [المستوى الرابع: Event Sourcing](#5-المستوى-الرابع-event-sourcing)
6. [المستوى الخامس: Saga Pattern](#6-المستوى-الخامس-saga-pattern)
7. [Domain Events و DDD](#7-domain-events-و-ddd)
8. [Event Schema Evolution و Versioning](#8-event-schema-evolution-و-versioning)
9. [Observability والمراقبة](#9-observability-والمراقبة)
10. [استراتيجيات الاختبار](#10-استراتيجيات-الاختبار)
11. [مقارنة Message Brokers](#11-مقارنة-message-brokers)
12. [أدوات ومكتبات Go الموصى بها](#12-أدوات-ومكتبات-go-الموصى-بها)
13. [أنماط مضادة (Anti-Patterns)](#13-أنماط-مضادة-anti-patterns)
14. [قرار المعمارية — متى تستخدم كل نمط؟](#14-قرار-المعمارية)

---

## 1. نظرة عامة

### ما هي Event-Driven Architecture؟

EDA هي نمط معماري يعتمد على **إنتاج واستهلاك ومعالجة الأحداث (Events)** بدلاً من الاستدعاءات
المتزامنة المباشرة. الحدث يمثل **حقيقة أعمال وقعت بالفعل** (مثل `OrderPlaced`, `PaymentProcessed`).

### لماذا Go مناسبة لـ EDA؟

| الميزة | التفاصيل |
| :--- | :--- |
| **Goroutines** | خفيفة الوزن (~2KB stack)، تمكّن من معالجة آلاف الأحداث بالتوازي |
| **Channels** | آلية طبيعية لتمرير البيانات بين goroutines بدون قفل صريح |
| **`context.Context`** | إدارة دورة حياة المعالجة، الإلغاء، والمُهل الزمنية |
| **`select` statement** | التعامل مع عدة قنوات ومُهل بشكل متزامن |
| **أداء عالي** | Compiled language مع garbage collector سريع |
| **بساطة** | لا تحتاج frameworks ثقيلة — يمكن بناء EDA بأدوات اللغة الأساسية |

---

## 2. المستوى الأول: In-Process Event Bus

### الوصف

نظام أحداث **داخلي** ضمن عملية واحدة (single process) لفك الارتباط بين الوحدات (modules)
داخل الخدمة الواحدة، **قبل** الانتقال إلى messaging موزع.

### النمطان الأساسيان

#### النمط 1: Mutex + Callbacks (Observer Pattern)

```go
type EventBus struct {
    mu       sync.RWMutex
    handlers map[string][]func(event any)
}

func (b *EventBus) Subscribe(topic string, handler func(event any)) {
    b.mu.Lock()
    defer b.mu.Unlock()
    b.handlers[topic] = append(b.handlers[topic], handler)
}

func (b *EventBus) Publish(topic string, event any) {
    b.mu.RLock()
    defer b.mu.RUnlock()
    for _, handler := range b.handlers[topic] {
        handler(event) // أو go handler(event) للتشغيل غير المتزامن
    }
}
```

**المزايا:** أسرع أداء، overhead أقل، ذاكرة أقل.
**العيوب:** إذا لم تُشغّل الـ callback في goroutine منفصلة، فإن handler بطيء سيُعطّل كل الـ dispatch.

#### النمط 2: Channel-Based Pub/Sub

```go
type Subscriber struct {
    Ch chan Event
}

type ChannelBus struct {
    mu          sync.RWMutex
    subscribers map[string][]Subscriber
}

func (b *ChannelBus) Publish(topic string, event Event) {
    b.mu.RLock()
    defer b.mu.RUnlock()
    for _, sub := range b.subscribers[topic] {
        select {
        case sub.Ch <- event:
        default:
            // القناة ممتلئة — قرار: تجاهل، تسجيل، أو blocking
        }
    }
}
```

**المزايا:** أكثر idiomatic في Go، backpressure طبيعي عبر buffered channels.
**العيوب:** overhead أعلى (إنشاء قنوات، context switching)، يتطلب إدارة دورة حياة القنوات.

### مقارنة سريعة

| الميزة | Mutex + Callbacks | Channels |
| :--- | :--- | :--- |
| **الأداء** | أعلى (overhead أقل) | أقل (allocations/scheduling) |
| **التعقيد** | بسيط للأنظمة الصغيرة | معقد (lifecycle, closing) |
| **Blocking** | يحجب الـ dispatcher إذا لم تُنتبه | يحجب إذا امتلأ buffer القناة |
| **الاستخدام المثالي** | إشعارات بسيطة، shared state | pipeline/worker patterns |

### أفضل الممارسات

1. **فصل التنفيذ:** لا تجعل الـ bus ينتظر subscriber بطيء — شغّل في goroutine أو مرّر لقناة.
2. **استخدام Generics (Go 1.18+):** أنشئ event bus type-safe باستخدام generic constraints.
3. **إدارة الاشتراكات:** وفّر دائماً طريقة لإلغاء الاشتراك (`Unsubscribe`) لمنع تسرب الذاكرة.
4. **اعرف حدود النمط:** In-process bus لفك ارتباط الوحدات الداخلية فقط — إذا احتجت ضمان التسليم
   عبر إعادة تشغيل التطبيق أو عبر عقد موزعة، انتقل إلى message broker خارجي.

---

## 3. المستوى الثاني: Pub/Sub الموزع

### الوصف

خدمات منفصلة تتواصل عبر **message broker** (Kafka, NATS, RabbitMQ) بدلاً من REST/gRPC المتزامن.
الخدمة تُنتج أحداثاً تمثل تغييرات حالة ذات معنى، والمشتركون يتفاعلون بشكل مستقل.

### الأنماط المعمارية

#### Choreography vs Orchestration

| الجانب | Choreography (لامركزي) | Orchestration (مركزي) |
| :--- | :--- | :--- |
| **التحكم** | كل خدمة تتفاعل مع الأحداث بشكل مستقل | خدمة منسّقة (Orchestrator) تدير التدفق |
| **الارتباط** | ضعيف — الخدمات لا تعرف عن بعضها | أقوى — الخدمات تعتمد على المنسّق |
| **التعقيد** | بسيط لتدفقات صغيرة، صعب التتبع عند التوسع | أسهل في التصحيح والمراقبة للتدفقات المعقدة |
| **الأفضل لـ** | تدفقات بسيطة | عمليات أعمال معقدة |

> **القاعدة:** فضّل Choreography للبساطة، وانتقل إلى Orchestration عندما يصبح التتبع صعباً.

### أفضل الممارسات

#### 1. التزامن والأداء

- **Worker Pools:** لا تُنشئ عدداً غير محدود من goroutines. استخدم نمط worker pool
  مع buffered channels كـ semaphores لحماية الموارد.
- **Buffered Channels:** عند استخدام قنوات داخلية، استخدم قنوات مُعازلة لمنع المنتجين
  من التوقف إذا تأخر المستهلكون مؤقتاً. راقب دائماً تشبّع القناة.
- **Graceful Shutdown:** استمع لإشارات OS (`SIGTERM`, `SIGINT`) لإيقاف المعالجة،
  تفريغ الأحداث المعلقة، وإغلاق اتصالات الـ broker بشكل نظيف.

```go
ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGTERM, syscall.SIGINT)
defer stop()

// ... تشغيل المستهلكين ...

<-ctx.Done()
// تنظيف: إغلاق الاتصالات، تفريغ القنوات
```

#### 2. الموثوقية والاتساق

- **Idempotency (عدم التكرار):** صمّم المستهلكين ليكونوا idempotent — معالجة نفس الحدث
  أكثر من مرة يجب ألا تُسبب آثاراً جانبية (مثل: تحقق من وجود السجل قبل إنشائه).
- **Transactional Outbox Pattern:** لضمان اتساق البيانات بين قاعدة البيانات والـ broker:
  1. اكتب الحدث في جدول `outbox` ضمن نفس المعاملة (transaction) مع منطق الأعمال.
  2. عملية خلفية منفصلة تقرأ من الجدول وتنشر الأحداث إلى الـ broker.
  3. هذا يمنع مشكلة "الكتابة المزدوجة" (dual-write).
- **Error Handling و Retries:** نفّذ منطق إعادة المحاولة مع exponential backoff.
  للأخطاء غير القابلة للاسترداد، استخدم **Dead Letter Queue (DLQ)** لعزل الأحداث
  المشكلة للفحص اليدوي دون حجب بقية الدفق.

#### 3. ترتيب الأحداث (Event Ordering)

- في الأنظمة الموزعة، قد تصل الأحداث بترتيب مختلف.
- إذا كان الترتيب مهماً: أضف **timestamps** أو **version numbers** في مخطط الحدث.
- Kafka يضمن الترتيب **داخل partition واحد** — استخدم partition key مناسب.

---

## 4. المستوى الثالث: CQRS

### Command Query Responsibility Segregation

فصل عمليات **الكتابة (Commands)** عن عمليات **القراءة (Queries)** في نماذج مختلفة.

### البنية

```
┌──────────────┐     Commands     ┌──────────────┐
│   Client     │ ───────────────► │  Write Model │ ──► Event Store
│   (API)      │                  │  (Aggregates)│     (Append-only)
└──────────────┘                  └──────────────┘
                                         │
                                    Domain Events
                                         │
                                         ▼
                                  ┌──────────────┐
                                  │  Read Model  │ ◄── Projections
                                  │  (Queries)   │     (Denormalized)
                                  └──────────────┘
```

### أفضل الممارسات

1. **فصل النماذج:** Write Model يركز على التحقق من قواعد الأعمال (invariants)
   وإصدار أحداث. Read Model هو إسقاط (projection) مُحسَّن للاستعلام.
2. **Read Model مخصص:** أنشئ read models مُصممة خصيصاً لكل حالة استخدام
   (PostgreSQL, MongoDB, Elasticsearch — حسب نمط الاستعلام).
3. **Eventual Consistency:** اعترف بأن الـ read model قد يتأخر عن الـ write model.
   صمّم تجربة المستخدم لتتعامل مع هذا (optimistic UI updates أو مؤشرات "جارٍ المعالجة").
4. **CQRS بدون Event Sourcing:** يمكن تطبيق CQRS بدون event sourcing —
   فقط افصل نماذج القراءة والكتابة. لا تُضف تعقيداً غير ضروري.

### مثال مبسط في Go

```go
// Command Side
type PlaceOrderCommand struct {
    OrderID    string
    CustomerID string
    Items      []OrderItem
}

type CommandHandler struct {
    repo       OrderRepository
    publisher  EventPublisher
}

func (h *CommandHandler) Handle(ctx context.Context, cmd PlaceOrderCommand) error {
    order, err := NewOrder(cmd.OrderID, cmd.CustomerID, cmd.Items)
    if err != nil {
        return fmt.Errorf("invalid order: %w", err)
    }

    if err := h.repo.Save(ctx, order); err != nil {
        return err
    }

    // نشر Domain Events
    for _, event := range order.DomainEvents() {
        if err := h.publisher.Publish(ctx, event); err != nil {
            return err
        }
    }
    return nil
}

// Query Side
type OrderView struct {
    OrderID    string
    Status     string
    Total      float64
    CreatedAt  time.Time
}

type QueryHandler struct {
    readDB *sql.DB
}

func (h *QueryHandler) GetOrder(ctx context.Context, id string) (OrderView, error) {
    // استعلام من read model مُحسَّن
    var view OrderView
    err := h.readDB.QueryRowContext(ctx,
        "SELECT order_id, status, total, created_at FROM order_views WHERE order_id = $1", id,
    ).Scan(&view.OrderID, &view.Status, &view.Total, &view.CreatedAt)
    return view, err
}
```

---

## 5. المستوى الرابع: Event Sourcing

### الوصف

بدلاً من تخزين **الحالة الحالية** فقط، يتم تخزين **سلسلة من الأحداث غير القابلة للتغيير**
(append-only log) التي أنتجت تلك الحالة. الحالة الحالية تُعاد بناؤها بإعادة تشغيل الأحداث.

### أفضل الممارسات

#### 1. تصميم الأحداث

- **أحداث كحقائق أعمال:** سمِّ الأحداث بصيغة الماضي وبمصطلحات الأعمال
  (`MoneyDeposited` وليس `BalanceUpdated`).
- **أحداث غنية:** ضمّن كل البيانات اللازمة لإعادة بناء الحالة — لا تعتمد على
  استعلامات خارجية أثناء الإعادة.

#### 2. Aggregate Roots كحدود

```go
type BankAccount struct {
    id      string
    balance int64
    version int
    events  []DomainEvent // أحداث غير منشورة
}

func (a *BankAccount) Deposit(amount int64) error {
    if amount <= 0 {
        return errors.New("amount must be positive")
    }
    a.apply(MoneyDeposited{
        AccountID: a.id,
        Amount:    amount,
        OccurredAt: time.Now(),
    })
    return nil
}

func (a *BankAccount) apply(event DomainEvent) {
    a.events = append(a.events, event)
    a.when(event) // تحديث الحالة الداخلية
}

func (a *BankAccount) when(event DomainEvent) {
    switch e := event.(type) {
    case MoneyDeposited:
        a.balance += e.Amount
        a.version++
    case MoneyWithdrawn:
        a.balance -= e.Amount
        a.version++
    }
}
```

#### 3. Snapshots

- للـ aggregates ذات تاريخ أحداث طويل، نفّذ **snapshotting** لتجنب إعادة
  تشغيل آلاف الأحداث عند كل تحميل.
- خزّن snapshot كل N حدث، وعند التحميل: حمّل آخر snapshot + الأحداث بعده فقط.

#### 4. Event Store

- استخدم **EventStoreDB** المتخصص أو **PostgreSQL** مع جدول append-only ومؤشرات مناسبة.
- لا تُعدّل أو تحذف أحداثاً أبداً — الأحداث غير قابلة للتغيير بتعريفها.

#### 5. Projections

- اشترك في الأحداث وحدّث read models مُحسَّنة.
- صمّم الـ projections لتكون **قابلة لإعادة البناء** من الصفر بإعادة تشغيل كل الأحداث.

#### متى لا تستخدم Event Sourcing

- إذا كان تطبيقك CRUD بسيط — Event Sourcing سيُضيف "تعقيداً عرضياً" يُعيق أكثر مما يُفيد.
- استخدمه فقط عندما يكون لديك سبب أعمال واضح: سجل تدقيق كامل، استعلامات زمنية معقدة،
  أو حاجة لتوسيع القراءة والكتابة بشكل مستقل.

---

## 6. المستوى الخامس: Saga Pattern

### الوصف

نمط لإدارة **المعاملات الموزعة** عبر خدمات متعددة، حيث ACID التقليدي غير عملي.
الـ Saga هي سلسلة من **معاملات محلية**، وإذا فشلت خطوة، تُنفَّذ **معاملات تعويضية**
(compensating transactions) للتراجع عن التغييرات السابقة.

### النوعان

#### 1. Choreography-Based Saga

```
Order Service ──(OrderPlaced)──► Payment Service
                                      │
                              (PaymentProcessed)
                                      │
                                      ▼
                               Shipping Service
                                      │
                              (ShipmentScheduled)
                                      │
                                      ▼
                              Notification Service
```

- **كل خدمة تتفاعل مع أحداث** من الخدمات الأخرى بدون "دماغ" مركزي.
- **مناسب لـ:** تدفقات بسيطة (3-4 خطوات).
- **المشكلة:** صعب التتبع والتصحيح مع نمو التعقيد.

#### 2. Orchestration-Based Saga

```
                    ┌─────────────────┐
                    │  Saga           │
                    │  Orchestrator   │
                    │  (State Machine)│
                    └────────┬────────┘
                             │
              ┌──────────────┼──────────────┐
              ▼              ▼              ▼
        Order Service  Payment Service  Shipping Service
```

- **خدمة منسّقة** (Orchestrator) تعمل كآلة حالة (state machine) — تستدعي
  الخدمات بترتيب وتدير الإعادة والتعويض.
- **مناسب لـ:** عمليات أعمال معقدة.
- **أسهل في التصحيح** لأن الحالة مركزية.

### أفضل الممارسات

1. **Transactional Outbox:** لتجنب مشكلة الكتابة المزدوجة — حدّث قاعدة البيانات
   وأرسل الحدث في نفس المعاملة.
2. **Idempotency إلزامي:** لأن الـ Saga تعتمد على retries وأحداث غير متزامنة.
3. **استخدام أُطر Orchestration:** بدلاً من بناء state machine من الصفر:
   - **[Temporal](https://temporal.io/):** موصى به بشدة لـ Go — يدير الاستمرارية
     والإعادة والمهل وإدارة الحالة تلقائياً.
4. **دلالات التعويض:** التعويض ليس "تراجع" (rollback) بل **عملية أعمال جديدة**
   تعكس أثر العملية السابقة (مثل: "استرداد" بدلاً من "حذف الدفعة").
5. **Correlation ID:** أضف معرّف ارتباط فريد في كل حدث/طلب لتتبع معاملة أعمال واحدة
   عبر عدة خدمات.

### متى تتجنب Saga

- الـ Saga تُضيف تعقيداً معمارياً كبيراً.
- إذا أمكن تجميع البيانات في خدمة واحدة (monolith أو colocation)، تجنب المعاملات الموزعة تماماً.

---

## 7. Domain Events و DDD

### تصميم الـ Aggregates

| المبدأ | التفاصيل |
| :--- | :--- |
| **Aggregate واحد لكل معاملة** | كل معاملة أعمال تعدّل aggregate واحد فقط. التحديثات عبر aggregates تتم عبر eventual consistency |
| **حماية الثوابت (Invariants)** | كل تغيير حالة يمر عبر Aggregate Root. إذا خُرقت قاعدة أعمال، أرجع خطأ فوراً |
| **الإشارة بالهوية** | لا تحمل مراجع مباشرة لـ aggregates أخرى — أشِر لها بالـ ID فقط |
| **حدود صغيرة** | صمّم أصغر حدود ممكنة تحافظ على الاتساق — aggregates كبيرة تسبب مشاكل تزامن |

### إصدار Domain Events

```go
type Order struct {
    id     string
    status OrderStatus
    events []DomainEvent // تُجمع ولا تُنشر مباشرة
}

func (o *Order) Confirm() error {
    if o.status != OrderStatusPending {
        return errors.New("can only confirm pending orders")
    }
    o.status = OrderStatusConfirmed
    o.events = append(o.events, OrderConfirmed{
        OrderID:    o.id,
        OccurredAt: time.Now(),
    })
    return nil
}

// DomainEvents تُقرأ من قبل Application Service أو Repository
func (o *Order) DomainEvents() []DomainEvent {
    return o.events
}

func (o *Order) ClearEvents() {
    o.events = nil
}
```

### أفضل الممارسات

1. **Emit, Don't Coordinate:** الـ aggregate يسجّل أن حدثاً وقع (يجمعها في slice داخلي).
   الـ Application Service أو Repository ينشرها.
2. **Outbox Pattern:** احفظ حالة الـ aggregate والأحداث في نفس معاملة قاعدة البيانات.
   عملية خلفية تلتقط الأحداث من جدول الـ outbox وتنشرها.
3. **بدون Frameworks ثقيلة:** DDD في Go يُنفَّذ بأفضل شكل بدون frameworks —
   فضّل بنية Hexagonal/Ports & Adapters حيث البنية التحتية هي plugin للـ domain.
4. **هيكل المجلدات:** نظّم بحسب **bounded context أو ميزة** وليس بحسب طبقة تقنية:

```
internal/
├── order/          # bounded context
│   ├── domain/     # aggregates, events, value objects
│   ├── app/        # application services (use cases)
│   ├── infra/      # repository implementations, broker adapters
│   └── port/       # interfaces (HTTP handlers, gRPC)
├── payment/
│   ├── domain/
│   ├── app/
│   ├── infra/
│   └── port/
```

1. **Constructors (Factories):** استخدم constructor functions (`NewOrder(...)`)
   لضمان أن الـ aggregate يُنشأ دائماً في حالة صالحة.

---

## 8. Event Schema Evolution و Versioning

### المشكلة

مع تطور متطلبات الأعمال، **يجب أن تتطور مخططات الأحداث** بدون كسر المستهلكين الحاليين.

### استراتيجيات الإصدار

| الاستراتيجية | التنفيذ | الأفضل لـ |
| :--- | :--- | :--- |
| **`dataschema` URI** (CloudEvents) | أشر إلى ملف/سجل مخطط مُصدَّر | مخططات موثقة ومعيارية |
| **Type Suffix** | أضف `.v1`, `.v2` لسمة `type` | متطلبات توجيه/تصفية بسيطة |
| **Schema Registry** | فرض مركزي للتوافق (Confluent) | أنظمة مؤسسية، فرق كبيرة |
| **تغييرات إضافية فقط** | لا حذف حقول ولا تغيير أنواع — حوكمة صارمة | تطور عالي الموثوقية |

### أفضل الممارسات

1. **فضّل التغييرات الإضافية:** لا تحذف حقولاً ولا تُغيّر أنواع بيانات.
   إضافة حقول اختيارية آمنة إذا كان المستهلكون مصممين لتجاهل الحقول المجهولة.
2. **دورات الإهمال (Deprecation):** إذا لزم تغيير جذري، حدّد الحقول القديمة
   كمُهملة مع جداول زمنية وأدلة ترحيل واضحة قبل حذفها.
3. **الظرف vs البيانات:** ضع منطق الإصدار في بيانات الحدث أو Registry وليس
   في ظرف CloudEvent — الظرف يجب أن يبقى مستقراً لضمان عمل البنية التحتية.
4. **مرونة المستهلك:** صمّم المستهلكين بارتباط ضعيف — تعامل بلطف مع الحقول
   الاختيارية المفقودة وتجاهل السمات غير المعروفة.
5. **اختبار التطور:** أضف اختبارات تطور المخطط في CI/CD — تحقق أن الإصدارات
   الجديدة لا تكسر المستهلكين الحاليين.

### CloudEvents في Go

```go
import cloudevents "github.com/cloudevents/sdk-go/v2"

event := cloudevents.NewEvent()
event.SetType("com.example.order.placed.v1")
event.SetSource("order-service")
event.SetDataSchema("https://schemas.example.com/order/v1.json")
event.SetData(cloudevents.ApplicationJSON, orderData)
```

---

## 9. Observability والمراقبة

### لماذا Observability حيوي في EDA

لأن الأحداث **غير متزامنة**، التصحيح صعب. لا يمكنك تتبع طلب أعمال واحد عبر
خدمات متعددة بدون أدوات مخصصة.

### الأعمدة الثلاثة

#### 1. Distributed Tracing

- **حقن Trace IDs** (OpenTelemetry) في headers الأحداث لتتبع طلب أعمال عبر عدة خدمات.
- **أدوات:** OpenTelemetry + Jaeger أو Tempo.

```go
import "go.opentelemetry.io/otel"

func publishEvent(ctx context.Context, event Event) {
    tracer := otel.Tracer("order-service")
    ctx, span := tracer.Start(ctx, "publish.OrderPlaced")
    defer span.End()

    // حقن trace context في headers الحدث
    carrier := propagation.MapCarrier{}
    otel.GetTextMapPropagator().Inject(ctx, carrier)

    event.SetHeaders(carrier)
    broker.Publish(ctx, event)
}
```

#### 2. المقاييس (Metrics)

| المقياس | الأهمية |
| :--- | :--- |
| **Throughput** | عدد الأحداث المنتجة/المستهلكة في الثانية |
| **Latency** | وقت المعالجة من الاستلام إلى الانتهاء |
| **Consumer Lag** | الفرق بين آخر حدث مُنتج وآخر حدث مُستهلك — **حيوي** لضمان عدم تأخر النظام |
| **Error Rate** | معدل فشل المعالجة |
| **Projection Lag** | التأخر بين استمرار الحدث وظهوره في read model |

- **أدوات:** Prometheus + Grafana.

#### 3. التسجيل المنظم (Structured Logging)

- استخدم `log/slog` مع Correlation IDs لربط جميع السجلات بمعاملة أعمال واحدة.

```go
slog.InfoContext(ctx, "event processed",
    slog.String("event_type", event.Type()),
    slog.String("event_id", event.ID()),
    slog.String("correlation_id", event.CorrelationID()),
    slog.Duration("processing_time", elapsed),
)
```

---

## 10. استراتيجيات الاختبار

### هرم الاختبار لـ EDA

| المستوى | الهدف | الاستراتيجية |
| :--- | :--- | :--- |
| **Unit** | صحة منطق الـ handler | Mocks, Table-driven tests |
| **Integration** | تفاعل الـ Broker والتدفق | Testcontainers, broker instances حقيقية |
| **Contract** | توافق المخططات | Schema Registry, Consumer-driven contracts |
| **End-to-End** | اتساق النظام ككل | Temporal/polling assertions |

### أفضل الممارسات

#### 1. Table-Driven Tests (نمط Go الاصطلاحي)

```go
func TestHandleOrderPlaced(t *testing.T) {
    tests := []struct {
        name    string
        event   OrderPlaced
        wantErr bool
    }{
        {
            name:    "valid order",
            event:   OrderPlaced{OrderID: "123", Amount: 100},
            wantErr: false,
        },
        {
            name:    "zero amount",
            event:   OrderPlaced{OrderID: "456", Amount: 0},
            wantErr: true,
        },
        {
            name:    "duplicate order (idempotency check)",
            event:   OrderPlaced{OrderID: "123", Amount: 100},
            wantErr: false, // يجب ألا يفشل عند المعالجة المتكررة
        },
    }

    for _, tt := range tests {
        t.Run(tt.name, func(t *testing.T) {
            handler := NewOrderHandler(mockRepo)
            err := handler.Handle(context.Background(), tt.event)
            if (err != nil) != tt.wantErr {
                t.Errorf("Handle() error = %v, wantErr %v", err, tt.wantErr)
            }
        })
    }
}
```

#### 2. Testcontainers لاختبارات التكامل

```go
func TestKafkaConsumer(t *testing.T) {
    ctx := context.Background()

    // تشغيل Kafka container حقيقي
    kafkaC, err := kafka.Run(ctx, "confluentinc/cp-kafka:7.6.1")
    testcontainers.CleanupContainer(t, kafkaC)
    require.NoError(t, err)

    brokers, err := kafkaC.Brokers(ctx)
    require.NoError(t, err)

    // ... اختبار Producer/Consumer مع broker حقيقي ...
}
```

#### 3. Temporal Assertions (Eventually Consistent)

```go
func assertEventually(t *testing.T, condition func() bool, timeout time.Duration) {
    t.Helper()
    deadline := time.Now().Add(timeout)
    for time.Now().Before(deadline) {
        if condition() {
            return
        }
        time.Sleep(100 * time.Millisecond)
    }
    t.Fatal("condition not met within timeout")
}

// الاستخدام:
assertEventually(t, func() bool {
    order, _ := readDB.GetOrder(ctx, "123")
    return order.Status == "confirmed"
}, 5*time.Second)
```

#### 4. اختبار Idempotency

- **اختبار حرج:** معالجة نفس الحدث مرتين يجب ألا تُسبب آثاراً مضاعفة
  (مثل إرسال إيميلين أو خصم مضاعف).

#### 5. التزامن

- استخدم `sync.WaitGroup` لانتظار المعالجة غير المتزامنة قبل التأكيد.
- شغّل مع `-race` flag لاكتشاف race conditions.
- لـ Go 1.24+: جرّب `testing/synctest` التجريبي للتحكم في اختبار الكود المتزامن.

---

## 11. مقارنة Message Brokers

### جدول المقارنة الشامل

| الميزة | **Apache Kafka** | **RabbitMQ** | **NATS (JetStream)** |
| :--- | :--- | :--- | :--- |
| **الاستخدام الأساسي** | بث أحداث عالي الإنتاجية، event logs | توجيه معقد، task queues | خدمات مصغرة خفيفة، latency منخفض |
| **البنية** | Distributed commit log | Broker-based (Exchange/Queue) | خفيف، mesh-native |
| **الاستمرارية** | دائمة (افتراضي) | قوائم دائمة | اختياري (عبر JetStream) |
| **الترتيب** | لكل partition | لكل queue | لكل subject (في JetStream) |
| **الجهد التشغيلي** | عالي | متوسط | منخفض |
| **التوسع** | ممتاز أفقياً | جيد | ممتاز |
| **Go SDK** | segmentio/kafka-go, confluent | amqp091-go | nats.go (رسمي) |

### إرشادات الاختيار (2025/2026)

- **اختر Kafka إذا:** بناء data pipeline ضخم، حاجة لتخزين أحداث طويل الأمد
  وإعادة التشغيل، فريق مخصص لعمليات المنصة.
- **اختر RabbitMQ إذا:** النظام يعتمد على توجيه معقد، معايير AMQP،
  أو أنماط task-queue تقليدية مُجربة.
- **اختر NATS/JetStream إذا:** الأولوية لـ latency منخفض، بساطة تشغيلية،
  وبناء خدمات Go cloud-native. **أفضل توازن بين الأداء وسهولة الاستخدام
  للمشاريع الجديدة في Go.**

---

## 12. أدوات ومكتبات Go الموصى بها

### المكتبات الأساسية

| المكتبة | الوصف | الرابط |
| :--- | :--- | :--- |
| **Watermill** | مكتبة شاملة لبناء تطبيقات event-driven — Pub/Sub, CQRS, Router, Outbox | [watermill.io](https://watermill.io/) |
| **Temporal** | منصة لـ workflow orchestration وSaga — يدير الاستمرارية والإعادة تلقائياً | [temporal.io](https://temporal.io/) |
| **CloudEvents SDK** | تطبيق معيار CloudEvents لتوحيد بنية الأحداث | [github.com/cloudevents/sdk-go](https://github.com/cloudevents/sdk-go) |

### مكتبات Message Brokers

| Broker | مكتبة Go | ملاحظات |
| :--- | :--- | :--- |
| **NATS** | `github.com/nats-io/nats.go` | مكتوب بـ Go، أفضل تجربة مطور |
| **Kafka** | `github.com/segmentio/kafka-go` | Pure Go, بدون CGO |
| **Kafka** | `github.com/confluentinc/confluent-kafka-go` | أداء أعلى (يعتمد على librdkafka) |
| **RabbitMQ** | `github.com/rabbitmq/amqp091-go` | المكتبة الرسمية |

### Watermill — لماذا هي مميزة؟

Watermill تقدم **واجهة Pub/Sub موحدة** عبر brokers مختلفة — يمكنك التبديل بين
Kafka, RabbitMQ, PostgreSQL, Redis, NATS, Go channels بدون تغيير منطق الأعمال.

**الميزات الرئيسية:**

- **Router:** مثل HTTP router لكن للرسائل — يدعم middleware (logging, recovery, correlation IDs, timeouts).
- **CQRS Component:** تجريد عالي المستوى — تعمل مباشرة مع Go structs بدلاً من raw messages.
- **Outbox/Forwarder:** حل جاهز لنمط Outbox لضمان ذرية تحديث الحالة ونشر الأحداث.
- **مكتبة وليس Framework:** لا تفرض بنية محددة — يمكن دمجها في أنظمة قائمة.

### مشاريع مرجعية

| المشروع | الوصف |
| :--- | :--- |
| **ThreeDotsLabs/wild-workouts-go-ddd-example** | مثال عملي شامل لـ DDD + CQRS + Clean Architecture في Go |
| **vardius/go-api-boilerplate** | boilerplate لـ Go API مع DDD وevent-driven patterns |
| **ThreeDotsLabs/go-web-app-antipatterns** | أنماط مضادة شائعة وكيفية تجنبها |

---

## 13. أنماط مضادة (Anti-Patterns)

### ❌ ما يجب تجنبه

| النمط المضاد | المشكلة | الحل |
| :--- | :--- | :--- |
| **Goroutines بلا حدود** | إنشاء goroutine لكل حدث دون حد → استنزاف الموارد | Worker Pool مع semaphore |
| **تجاهل Idempotency** | معالجة الحدث مرتين → بيانات مكررة | تحقق من المعرف قبل المعالجة |
| **Dual-Write** | تحديث DB ثم نشر حدث بشكل منفصل → عدم اتساق | Transactional Outbox Pattern |
| **قنوات بلا إغلاق** | قنوات مفتوحة بلا إغلاق → goroutine leaks | أغلق القنوات وأدِر الدورة بـ `context.Context` |
| **EDA لكل شيء** | تطبيق event-driven على CRUD بسيط → تعقيد غير مبرر | استخدم EDA فقط حيث تُضيف قيمة واضحة |
| **Aggregates كبيرة** | aggregate يحتوي كثيراً من البيانات → مشاكل تزامن | صمّم أصغر حدود ممكنة |
| **أحداث كأوامر** | تسمية الأحداث بصيغة أوامر (`CreateOrder`) | الأحداث حقائق ماضية (`OrderCreated`) |
| **تجاهل ترتيب الأحداث** | افتراض وصول الأحداث بالترتيب → حالة غير متسقة | أضف version/timestamp واتعامل مع الترتيب |
| **عدم مراقبة Consumer Lag** | المستهلكون يتأخرون دون علم → تدهور النظام | راقب consumer lag بـ Prometheus |

---

## 14. قرار المعمارية

### شجرة القرار — متى تستخدم كل نمط؟

```
هل تحتاج فك ارتباط داخل خدمة واحدة فقط؟
├── نعم ──► In-Process Event Bus (channels/callbacks)
└── لا ──► هل تحتاج تواصل بين خدمات؟
            ├── نعم ──► هل التواصل بسيط (إشعارات)؟
            │           ├── نعم ──► Pub/Sub (NATS/Kafka/RabbitMQ)
            │           └── لا ──► هل تحتاج فصل القراءة عن الكتابة؟
            │                       ├── نعم ──► CQRS
            │                       └── لا ──► هل تحتاج سجل تدقيق كامل؟
            │                                   ├── نعم ──► Event Sourcing + CQRS
            │                                   └── لا ──► Pub/Sub كافي
            └── لا ──► هل تحتاج معاملات عبر خدمات؟
                        ├── نعم ──► Saga Pattern
                        │           ├── بسيطة ──► Choreography
                        │           └── معقدة ──► Orchestration (Temporal)
                        └── لا ──► REST/gRPC المتزامن كافي
```

### ملخص مستويات النضج

| المستوى | النمط | التعقيد | متى تستخدم |
| :--- | :--- | :--- | :--- |
| **1** | In-Process Event Bus | منخفض | فك ارتباط الوحدات داخل خدمة واحدة |
| **2** | Pub/Sub الموزع | متوسط | تواصل غير متزامن بين خدمات |
| **3** | CQRS | متوسط-عالي | فصل نماذج القراءة والكتابة لتحسين الأداء |
| **4** | Event Sourcing | عالي | سجل تدقيق كامل، استعلامات زمنية |
| **5** | Saga | عالي جداً | معاملات موزعة عبر خدمات متعددة |

### القاعدة الذهبية

> **ابدأ بأبسط نمط يحل مشكلتك الفعلية، ثم تطوّر تدريجياً عند الحاجة.
> لا تُضف EDA لمجرد أنها "أفضل ممارسة" — أضفها عندما تكون هناك حاجة أعمال واضحة.**

---

## المصادر والمراجع

### مصادر رسمية ومرجعية

- [go.dev](https://go.dev/) — الموقع الرسمي للغة Go
- [watermill.io](https://watermill.io/) — مكتبة Watermill لـ Event-Driven Go
- [threedots.tech](https://threedots.tech/) — Three Dots Labs: DDD وEDA في Go
- [temporal.io](https://temporal.io/) — Temporal: Workflow Orchestration
- [cloudevents.io](https://cloudevents.io/) — معيار CloudEvents
- [github.com/cloudevents/sdk-go](https://github.com/cloudevents/sdk-go) — CloudEvents Go SDK
- [nats.io](https://nats.io/) — NATS Messaging System
- [kafka.apache.org](https://kafka.apache.org/) — Apache Kafka
- [rabbitmq.com](https://www.rabbitmq.com/) — RabbitMQ

### مشاريع مرجعية

- [ThreeDotsLabs/wild-workouts-go-ddd-example](https://github.com/ThreeDotsLabs/wild-workouts-go-ddd-example)
- [ThreeDotsLabs/watermill](https://github.com/ThreeDotsLabs/watermill)
- [vardius/go-api-boilerplate](https://github.com/vardius/go-api-boilerplate)

### مقالات ومحتوى تعليمي

- Three Dots Labs — "Go with the Domain" series
- conduktor.io — Kafka patterns و Saga comparison
- codeopinion.com — Event-Driven Architecture patterns
- milanjovanovic.tech — DDD Aggregate design
- testcontainers.org — Integration testing with Go
