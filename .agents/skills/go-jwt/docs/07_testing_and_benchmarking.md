# 07. استراتيجيات الاختبار والمحاكاة المتقدمة والقياس المعياري

## 1. فلسفة الاختبارات الأمنية السلبية (Negative Testing)

في اختبارات الـ JWT، لا يكفي التأكد من نجاح الرمز السليم، بل يجب كتابة اختبارات تتحقق من فشل النظام عند محاولات التلاعب:

1. **اختبار انتهاء الصلاحية:** التأكد من رفض الرمز إذا انتهى وقته حتى لو كان توقيعه سليماً.
2. **اختبار تبديل الخوارزمية:** محاولة إرسال رمز موقع بـ `HS256` لخادم ينتظر `RS256` والتأكد من إرجاع خطأ فوري.
3. **اختبار المفتاح الخبيث:** إنشاء زوج مفاتيح منفصل بواسطة المهاجم وتوقيع رمز يدعي أنه يحمل `kid` موثوق، والتأكد من رفض الخادم له.
4. **اختبار تزوير الجمهور (`aud` Mismatch):** إرسال رمز صادر لخدمة أخرى والتأكد من رفضه.

---

## 2. محاكاة خادم الـ JWKS في اختبارات Go

باستخدام `httptest.NewServer`، يمكن محاكاة خادم المفاتيح واختبار استجابة الكاش وتدوير المفاتيح:

```go
func setupMockJWKS(t *testing.T, pubKey *rsa.PublicKey) *httptest.Server {
    return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
        // تقديم بنية الـ JWKS المتوافقة مع RFC 7517
        w.Header().Set("Content-Type", "application/json")
        _ = json.NewEncoder(w).Encode(map[string]any{
            "keys": []map[string]any{
                {
                    "kty": "RSA",
                    "kid": "key-2026-v1",
                    "alg": "RS256",
                    "use": "sig",
                    "n":   base64.RawURLEncoding.EncodeToString(pubKey.N.Bytes()),
                    "e":   "AQAB",
                },
            },
        })
    }))
}
```

---

## 3. القياس المعياري للأداء وتتبع التخصيصات (Allocations Tracking)

```go
func BenchmarkJWTValidation(b *testing.B) {
    tokenStr := generateValidTestToken()
    verifier := setupProductionVerifier()

    b.ResetTimer()
    b.ReportAllocs() // حساب عدد التخصيصات في كل عملية B/op و allocs/op

    for i := 0; i < b.N; i++ {
        _, err := verifier.Validate(tokenStr)
        if err != nil {
            b.Fatalf("unexpected validation error: %v", err)
        }
    }
}
```

الهدف في بيئات الإنتاج: الوصول إلى **أقل من 5 allocations لكل عملية تحقق** وزمن استجابة أقل من 5 ميكروثانية لـ Ed25519 وأقل من 50 ميكروثانية لـ RSA.
