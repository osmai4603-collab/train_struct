# التحليل المخصص للأنواع والمهلات (Custom Types & Resilient Parsing)

في ملفات التكوين ومتغيرات البيئة، يعبر البشر عن القيم بطرق متباينة. يكتب مهندس العمليات المهلة كـ `"15s"` أو كعدد ثوانٍ صريح `15`، ويكتب حد الذاكرة كـ `"256MB"`، والمتغير المنطقي كـ `"true"` أو `"1"` أو `"yes"`.
الخدمة الإنتاجية يجب ألا تنهار بسبب هذه الاختلافات، بل توفر محللات مرنة وذكية (Resilient Parsers).

---

## 1. تحليل الفترات الزمنية المخصص (`Duration`)

بشكل افتراضي، تفشل حزمة `encoding/json` القياسية في Go عند محاولة تفكيك سلاسل مثل `"5s"` إلى `time.Duration` لأنها تتوقع نانو ثوانٍ كرقم.

### الحل المعماري: تغليف `time.Duration`

```go
type Duration time.Duration

func (d Duration) Duration() time.Duration {
    return time.Duration(d)
}

// UnmarshalJSON يدعم النصوص ("5s", "10m") والأرقام الصحيحة (بالثواني)
func (d *Duration) UnmarshalJSON(b []byte) error {
    var raw interface{}
    if err := json.Unmarshal(b, &raw); err != nil {
        return err
    }

    switch val := raw.(type) {
    case string:
        parsed, err := time.ParseDuration(val)
        if err != nil {
            return fmt.Errorf("invalid duration string: %w", err)
        }
        *d = Duration(parsed)
        return nil
    case float64:
        *d = Duration(time.Duration(val * float64(time.Second)))
        return nil
    default:
        return errors.New("duration must be a string (e.g. '5s') or integer seconds")
    }
}
```

---

## 2. تحليل أحجام الذاكرة والملفات (`ByteSize`)

التعامل مع حدود أحجام الترويسات (`MaxHeaderBytes`) وحدود أحجام جسم الطلبات (`MaxBodySize`):

- **دعم الوحدات الثنائية القياسية**: `B`, `KB`, `MB`, `GB`, `TB`.
- **التحويل الدقيق**: تحويل `"10MB"` إلى `10,485,760` بايت.
- **التطبيق البرمجي**:

  ```go
  type ByteSize int64

  func (b ByteSize) Bytes() int64 { return int64(b) }

  func (b *ByteSize) UnmarshalJSON(data []byte) error {
      // يدعم النصوص مثل "512MB" والأرقام كبايتات مباشرة
      return unmarshalByteSize(data, b)
  }
  ```

---

## 3. التحليل المتسامح للمتغيرات المنطقية (Flexible Booleans)

في المتغيرات البيئية، غالباً ما يكتب المستخدمون قيم التفعيل بصيغ مختلفة:

- **القيمة الموجبة (True)**: `"true"`, `"1"`, `"yes"`, `"on"`.
- **القيمة السالبة (False)**: `"false"`, `"0"`, `"no"`, `"off"`.

يتم استخدام `strconv.ParseBool` مع تنظيف السلسلة وتوحيد حالة الأحرف (`strings.ToLower(strings.TrimSpace(val))`) لدعم كافة الصيغ الشائعة بسلاسة.
