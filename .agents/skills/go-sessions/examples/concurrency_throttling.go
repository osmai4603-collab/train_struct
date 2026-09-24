package examples

import (
	"context"
	"sync"
	"time"
)

// ActivityThrottler يدير كبح كتابة وقت النشاط في الذاكرة لتخفيف الضغط عن المخزن المركزي
type ActivityThrottler struct {
	mu            sync.Mutex
	lastPersisted map[string]time.Time
	interval      time.Duration
}

func NewActivityThrottler(interval time.Duration) *ActivityThrottler {
	if interval <= 0 {
		interval = 1 * time.Minute
	}
	return &ActivityThrottler{
		lastPersisted: make(map[string]time.Time),
		interval:      interval,
	}
}

// ShouldUpdate يفحص ما إذا كان التحديث مطلوباً في هذه اللحظة
func (t *ActivityThrottler) ShouldUpdate(tokenHash string, now time.Time) bool {
	t.mu.Lock()
	defer t.mu.Unlock()

	last, ok := t.lastPersisted[tokenHash]
	if !ok || now.Sub(last) >= t.interval {
		t.lastPersisted[tokenHash] = now
		return true
	}

	return false
}

// Remove يمحو معرف الجلسة من ذاكرة الكبح عند تسجيل الخروج
func (t *ActivityThrottler) Remove(tokenHash string) {
	t.mu.Lock()
	defer t.mu.Unlock()
	delete(t.lastPersisted, tokenHash)
}

// CleanStale يحذف السجلات القديمة دورياً لمنع تراكم الذاكرة
func (t *ActivityThrottler) CleanStale(threshold time.Duration) {
	t.mu.Lock()
	defer t.mu.Unlock()

	cutoff := time.Now().Add(-threshold)
	for k, v := range t.lastPersisted {
		if v.Before(cutoff) {
			delete(t.lastPersisted, k)
		}
	}
}

// SafeSessionMutator نموذج لتعديل بيانات الجلسة بأمان تفاؤلي
type SafeSessionMutator struct {
	store SessionStore
}

func NewSafeSessionMutator(store SessionStore) *SafeSessionMutator {
	return &SafeSessionMutator{store: store}
}

// MutateField يُعدل حقلاً معيناً مع إعادة المحاولة في حال وجود تضارب
func (m *SafeSessionMutator) MutateField(ctx context.Context, tokenHash string, mutator func(meta map[string]interface{})) error {
	data, err := m.store.Get(ctx, tokenHash)
	if err != nil {
		return err
	}

	if data.Metadata == nil {
		data.Metadata = make(map[string]interface{})
	}

	mutator(data.Metadata)

	ttl := time.Until(data.AbsoluteExp)
	return m.store.Set(ctx, tokenHash, data, ttl)
}
