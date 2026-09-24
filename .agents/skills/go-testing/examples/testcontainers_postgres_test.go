//go:build integration

package examples_test

import (
	"context"
	"database/sql"
	"testing"
	"time"

	_ "github.com/jackc/pgx/v5/stdlib"
	"github.com/testcontainers/testcontainers-go"
	"github.com/testcontainers/testcontainers-go/modules/postgres"
	"github.com/testcontainers/testcontainers-go/wait"
)

// Product يمثل سجل المنتج في قاعدة البيانات
type Product struct {
	ID    string
	Name  string
	Price int64
}

// setupPostgresContainer ينشئ حاوية بوستجريس عابرة مع تنظيف تلقائي عبر t.Cleanup
func setupPostgresContainer(t *testing.T) *sql.DB {
	t.Helper()
	ctx := t.Context()

	// تشغيل حاوية بوستجريس 16 مع انتظار رسالة الجاهزية
	pgContainer, err := postgres.Run(ctx,
		"postgres:16-alpine",
		postgres.WithDatabase("shop_test"),
		postgres.WithUsername("testuser"),
		postgres.WithPassword("testpass"),
		testcontainers.WithWaitStrategy(
			wait.ForLog("database system is ready to accept connections").
				WithOccurrence(2).
				WithStartupTimeout(20*time.Second),
		),
	)
	if err != nil {
		t.Fatalf("failed to start postgres container: %v", err)
	}

	// تسجيل تدمير الحاوية فور اكتمال الاختبار
	t.Cleanup(func() {
		if err := pgContainer.Terminate(context.Background()); err != nil {
			t.Logf("warning: failed to terminate postgres container: %v", err)
		}
	})

	connStr, err := pgContainer.ConnectionString(ctx, "sslmode=disable")
	if err != nil {
		t.Fatalf("failed to obtain database connection string: %v", err)
	}

	db, err := sql.Open("pgx", connStr)
	if err != nil {
		t.Fatalf("failed to open sql connection: %v", err)
	}

	t.Cleanup(func() {
		_ = db.Close()
	})

	// إنشاء المخطط الأولي
	createTableSQL := `
	CREATE TABLE IF NOT EXISTS products (
		id VARCHAR(64) PRIMARY KEY,
		name VARCHAR(255) NOT NULL,
		price BIGINT NOT NULL CHECK (price >= 0)
	);`
	if _, err := db.ExecContext(ctx, createTableSQL); err != nil {
		t.Fatalf("failed to apply migrations: %v", err)
	}

	return db
}

func TestProductRepository_PostgresIntegration(t *testing.T) {
	db := setupPostgresContainer(t)

	t.Run("insert and query product with constraints", func(t *testing.T) {
		ctx := t.Context()

		// 1. إدخال منتج صحيح
		_, err := db.ExecContext(ctx, "INSERT INTO products (id, name, price) VALUES ($1, $2, $3)", "prd_01", "Laptop", 1200)
		if err != nil {
			t.Fatalf("unexpected error inserting product: %v", err)
		}

		// 2. التحقق من القراءة
		var p Product
		row := db.QueryRowContext(ctx, "SELECT id, name, price FROM products WHERE id = $1", "prd_01")
		if err := row.Scan(&p.ID, &p.Name, &p.Price); err != nil {
			t.Fatalf("failed to scan product: %v", err)
		}

		if p.Name != "Laptop" || p.Price != 1200 {
			t.Errorf("product data mismatch: got %+v, want Name=Laptop Price=1200", p)
		}

		// 3. التحقق من تطبيق القيود الحقيقية لقاعدة البيانات (Check Constraint)
		_, err = db.ExecContext(ctx, "INSERT INTO products (id, name, price) VALUES ($1, $2, $3)", "prd_02", "BadProduct", -50)
		if err == nil {
			t.Fatalf("expected error due to check constraint price >= 0, got nil")
		}
	})
}
