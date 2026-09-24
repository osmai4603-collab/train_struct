package examples

import (
	"context"
	"errors"
	"fmt"
	"sync"
)

// Account يمثل كائن الحساب في النطاق
type Account struct {
	ID      string
	OwnerID string
	Balance int64
}

// AccountRepository الواجهة التي يطلبها كود الأعمال (Consumer-defined interface)
type AccountRepository interface {
	Save(ctx context.Context, acc *Account) error
	FindByID(ctx context.Context, id string) (*Account, error)
	Transfer(ctx context.Context, fromID, toID string, amount int64) error
}

// InMemAccountRepository تطبيق Fake كامل في الذاكرة وآمن للتزامن المتعدد
type InMemAccountRepository struct {
	mu       sync.RWMutex
	accounts map[string]*Account

	// خطافات لمحاكاة أخطاء الشبكة أثناء الاختبار (Fault Injection Hooks)
	SimulateError error
}

// NewInMemAccountRepository ينشئ مستودع Fake جديد
func NewInMemAccountRepository() *InMemAccountRepository {
	return &InMemAccountRepository{
		accounts: make(map[string]*Account),
	}
}

func (r *InMemAccountRepository) Save(ctx context.Context, acc *Account) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	if r.SimulateError != nil {
		return r.SimulateError
	}

	// حفظ نسخة عميقة (Deep Copy) لمنع التعديل الجانبي على المؤشر الخارجي
	r.accounts[acc.ID] = &Account{
		ID:      acc.ID,
		OwnerID: acc.OwnerID,
		Balance: acc.Balance,
	}
	return nil
}

func (r *InMemAccountRepository) FindByID(ctx context.Context, id string) (*Account, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	if r.SimulateError != nil {
		return nil, r.SimulateError
	}

	acc, exists := r.accounts[id]
	if !exists {
		return nil, fmt.Errorf("account with id %s not found: %w", id, errors.New("not_found"))
	}

	// إعادة نسخة عميقة للحفاظ على عزل الحالة
	return &Account{
		ID:      acc.ID,
		OwnerID: acc.OwnerID,
		Balance: acc.Balance,
	}, nil
}

func (r *InMemAccountRepository) Transfer(ctx context.Context, fromID, toID string, amount int64) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	if r.SimulateError != nil {
		return r.SimulateError
	}

	from, exists := r.accounts[fromID]
	if !exists {
		return fmt.Errorf("sender account %s not found", fromID)
	}

	to, exists := r.accounts[toID]
	if !exists {
		return fmt.Errorf("receiver account %s not found", toID)
	}

	if from.Balance < amount {
		return errors.New("insufficient balance")
	}

	from.Balance -= amount
	to.Balance += amount
	return nil
}
