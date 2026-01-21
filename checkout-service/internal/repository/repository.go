package repository

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/golang-migrate/migrate/v4"
	"github.com/golang-migrate/migrate/v4/database/postgres"
	_ "github.com/golang-migrate/migrate/v4/source/file"
	_ "github.com/lib/pq"
)

var (
	ErrIdempotencyKeyNotFound = errors.New("idempotencyKey not found")
)

// CheckoutSession represents a checkout session in the database.
// Maps to the checkout_sessions table.
type CheckoutSession struct {
	ID                     string          `db:"id"`
	UserID                 string          `db:"user_id"`
	CartSnapshot           json.RawMessage `db:"cart_snapshot"`
	Status                 string          `db:"status"`
	IdempotencyKey         string          `db:"idempotency_key"`
	InventoryReservationID *string         `db:"inventory_reservation_id"`
	PaymentID              *string         `db:"payment_id"`
	TotalAmount            string          `db:"total_amount"`
	Currency               string          `db:"currency"`
	CreatedAt              time.Time       `db:"created_at"`
	UpdatedAt              time.Time       `db:"updated_at"`
}

type Credentials struct {
	Host              string
	Port              int
	User              string
	Password          string
	DBName            string
	MigrationsDirPath string
}

type Repository struct {
	db *sql.DB
}

type RepoInterface interface {
	Close() error
	RunMigrations(*Credentials) error
	GetCheckoutSessionByIdempotencyKey(ctx context.Context, key string) (*string, *string, error)
}

func NewRepository(cred *Credentials) (*Repository, error) {
	psqlconn := fmt.Sprintf(
		"host=%s port=%d user=%s password=%s dbname=%s sslmode=disable",
		cred.Host,
		cred.Port,
		cred.User,
		cred.Password,
		cred.DBName)

	// open database
	db, err := sql.Open("postgres", psqlconn)

	if err != nil {
		return nil, fmt.Errorf("failed to open database: %w", err)
	}

	// check db
	if e2 := db.Ping(); e2 != nil {
		return nil, fmt.Errorf("failed to ping database: %w", e2)
	}

	db.SetMaxOpenConns(100)
	db.SetMaxIdleConns(10)
	fmt.Println("Connected to postgres!")
	return &Repository{db: db}, nil
}

func (r *Repository) GetCheckoutSessionByIdempotencyKey(ctx context.Context, key string) (*string, *string, error) {
	const query = `SELECT id, status FROM checkout_sessions WHERE idempotency_key = $1;`

	var id string
	var status string
	err := r.db.QueryRowContext(ctx, query, key).Scan(&id, &status)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, nil, ErrIdempotencyKeyNotFound
	}
	if err != nil {
		return nil, nil, fmt.Errorf("query checkout session: %w", err)
	}
	return &id, &status, nil
}

func (r *Repository) RunMigrations(cred *Credentials) error {
	driver, err := postgres.WithInstance(r.db, &postgres.Config{})
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

func (r *Repository) Close() error {
	return r.db.Close()
}
