package repository

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"

	"github.com/fjod/go_cart/orders-service/internal/domain"
	"github.com/golang-migrate/migrate/v4"
	"github.com/golang-migrate/migrate/v4/database/postgres"
	_ "github.com/golang-migrate/migrate/v4/source/file"
	"github.com/google/uuid"
	"github.com/lib/pq"
	_ "github.com/lib/pq"
)

type Repository struct {
	db *sql.DB
}

func NewRepository(cred *Credentials) (*Repository, error) {
	psqlconn := fmt.Sprintf(
		"host=%s port=%d user=%s password=%s dbname=%s sslmode=disable",
		cred.Host,
		cred.Port,
		cred.User,
		cred.Password,
		cred.DBName)

	db, err := sql.Open("postgres", psqlconn)
	if err != nil {
		return nil, fmt.Errorf("failed to open database: %w", err)
	}

	if e2 := db.Ping(); e2 != nil {
		return nil, fmt.Errorf("failed to ping database: %w", e2)
	}

	db.SetMaxOpenConns(100)
	db.SetMaxIdleConns(10)
	fmt.Println("Connected to postgres!")
	return &Repository{db: db}, nil
}

func (r *Repository) RunMigrations(cred *Credentials) error {
	driver, err := postgres.WithInstance(r.db, &postgres.Config{
		MigrationsTable: "orders_schema_migrations",
	})
	if err != nil {
		return fmt.Errorf("could not create migration driver: %w", err)
	}

	m, err := migrate.NewWithDatabaseInstance(
		fmt.Sprintf("file://%s", cred.MigrationsDirPath),
		"postgres",
		driver,
	)
	if err != nil {
		return fmt.Errorf("could not create migrate instance: %w", err)
	}

	if e2 := m.Up(); e2 != nil && !errors.Is(e2, migrate.ErrNoChange) {
		return fmt.Errorf("could not run migrations: %w", e2)
	}

	return nil
}

func (r *Repository) CreateOrder(ctx context.Context, order *domain.Order) error {
	itemsJSON, err := json.Marshal(order.Items)
	if err != nil {
		return fmt.Errorf("failed to marshal order items: %w", err)
	}

	query := `INSERT INTO orders (id, checkout_id, user_id, total_amount, currency, status, items, created_at, updated_at)
	          VALUES ($1, $2, $3, $4, $5, $6, $7, NOW(), NOW())`

	_, insertErr := r.db.ExecContext(ctx, query,
		order.ID,
		order.CheckoutID,
		order.UserID,
		order.TotalAmount,
		order.Currency,
		order.Status,
		itemsJSON)

	if insertErr != nil {
		var pqErr *pq.Error
		if errors.As(insertErr, &pqErr) && pqErr.Code == "23505" {
			return ErrDuplicateCheckout
		}
		return fmt.Errorf("insert order: %w", insertErr)
	}
	return nil
}

func (r *Repository) GetOrderByID(ctx context.Context, id uuid.UUID) (*domain.Order, error) {
	query := `SELECT id, checkout_id, user_id, total_amount, currency, status, items, created_at, updated_at
	          FROM orders WHERE id = $1`

	var order domain.Order
	var itemsJSON []byte
	err := r.db.QueryRowContext(ctx, query, id).Scan(
		&order.ID,
		&order.CheckoutID,
		&order.UserID,
		&order.TotalAmount,
		&order.Currency,
		&order.Status,
		&itemsJSON,
		&order.CreatedAt,
		&order.UpdatedAt,
	)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, ErrOrderNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("query order by id: %w", err)
	}

	if err := json.Unmarshal(itemsJSON, &order.Items); err != nil {
		return nil, fmt.Errorf("unmarshal order items: %w", err)
	}

	return &order, nil
}

func (r *Repository) ListOrdersByUserID(ctx context.Context, userID string) ([]*domain.Order, error) {
	query := `SELECT id, checkout_id, user_id, total_amount, currency, status, items, created_at, updated_at
	          FROM orders WHERE user_id = $1 ORDER BY created_at DESC`

	rows, err := r.db.QueryContext(ctx, query, userID)
	if err != nil {
		return nil, fmt.Errorf("query orders by user id: %w", err)
	}
	defer rows.Close()

	var orders []*domain.Order
	for rows.Next() {
		var order domain.Order
		var itemsJSON []byte
		if err := rows.Scan(
			&order.ID,
			&order.CheckoutID,
			&order.UserID,
			&order.TotalAmount,
			&order.Currency,
			&order.Status,
			&itemsJSON,
			&order.CreatedAt,
			&order.UpdatedAt,
		); err != nil {
			return nil, fmt.Errorf("scan order row: %w", err)
		}
		if err := json.Unmarshal(itemsJSON, &order.Items); err != nil {
			return nil, fmt.Errorf("unmarshal order items: %w", err)
		}
		orders = append(orders, &order)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("row iteration error: %w", err)
	}

	return orders, nil
}

func (r *Repository) Close() error {
	return r.db.Close()
}
