package repository

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	d "github.com/fjod/go_cart/checkout-service/domain"
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
	ID                     string           `db:"id"`
	UserID                 string           `db:"user_id"`
	CartSnapshot           json.RawMessage  `db:"cart_snapshot"`
	Status                 d.CheckoutStatus `db:"status"`
	IdempotencyKey         string           `db:"idempotency_key"`
	InventoryReservationID *string          `db:"inventory_reservation_id"`
	PaymentID              *string          `db:"payment_id"`
	TotalAmount            string           `db:"total_amount"`
	Currency               string           `db:"currency"`
	CreatedAt              time.Time        `db:"created_at"`
	UpdatedAt              time.Time        `db:"updated_at"`
}

type OutboxEvent struct {
	ID          int        `db:"id"`
	AggregateId string     `db:"aggregate_id"`
	EventType   string     `db:"event_type"`
	Payload     []byte     `db:"payload"`
	CreatedAt   time.Time  `db:"created_at"`
	ProcessedAt *time.Time `db:"processed_at"` // can be nil
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
	GetCheckoutSessionByIdempotencyKey(ctx context.Context, key string) (*string, *d.CheckoutStatus, error)
	CreateCheckoutSession(ctx context.Context, session *CheckoutSession) error
	UpdateCheckoutSessionStatus(ctx context.Context, id *string, s *d.CheckoutStatus) error
	SetReservation(ctx context.Context, id *string, s *d.CheckoutStatus, reserveId *string) error
	SetPayment(ctx context.Context, id *string, s *d.CheckoutStatus, payId *string) error
	CompleteCheckoutSession(ctx context.Context, id *string, snapshot []byte, s *d.CheckoutStatus) error
	GetUnprocessedEvents(ctx context.Context, limit int) ([]*OutboxEvent, error)
	MarkEventAsProcessed(ctx context.Context, id int) error
	GetStuckSessions(ctx context.Context) ([]*CheckoutSession, error)
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

func (r *Repository) GetCheckoutSessionByIdempotencyKey(ctx context.Context, key string) (*string, *d.CheckoutStatus, error) {
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
	retStatus := d.CheckoutStatus(status)
	return &id, &retStatus, nil
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

func (r *Repository) CreateCheckoutSession(ctx context.Context, s *CheckoutSession) error {
	query := `INSERT INTO checkout_sessions (id, user_id, cart_snapshot, idempotency_key,  status, total_amount, created_at, updated_at) 
               VALUES ($1, $2, $3, $4, $5, $6, NOW(), NOW())`

	_, insertErr := r.db.ExecContext(ctx, query,
		s.ID,                      // id
		s.UserID,                  // user_id
		s.CartSnapshot,            // cart_snapshot
		s.IdempotencyKey,          // idempotency_key
		d.CheckoutStatusInitiated, // status
		s.TotalAmount)

	if insertErr != nil {
		return fmt.Errorf("insert checkout session: %w", insertErr)
	}
	return nil
}

func (r *Repository) UpdateCheckoutSessionStatus(ctx context.Context, id *string, s *d.CheckoutStatus) error {
	query := `UPDATE checkout_sessions SET status = $1, updated_at = NOW() WHERE id = $2`
	result, update := r.db.ExecContext(ctx, query,
		*s,
		*id)

	if update != nil {
		return fmt.Errorf("update checkout session: %w", update)
	}
	rows, e := result.RowsAffected()
	if e != nil {
		return fmt.Errorf("checking rows affected: %w", e)
	}
	if rows == 0 {
		return fmt.Errorf("checkout session not found: %s", *id)
	}
	return nil
}

func (r *Repository) SetReservation(ctx context.Context, id *string, s *d.CheckoutStatus, reserveId *string) error {
	query := `UPDATE checkout_sessions SET status = $1, updated_at = NOW(), inventory_reservation_id = $2 WHERE id = $3`
	result, update := r.db.ExecContext(ctx, query,
		*s,
		*reserveId,
		*id)

	if update != nil {
		return fmt.Errorf("update checkout session: %w", update)
	}
	rows, e := result.RowsAffected()
	if e != nil {
		return fmt.Errorf("checking rows affected: %w", e)
	}
	if rows == 0 {
		return fmt.Errorf("checkout session not found: %s", *id)
	}
	return nil
}

func (r *Repository) SetPayment(ctx context.Context, id *string, s *d.CheckoutStatus, payId *string) error {
	query := `UPDATE checkout_sessions SET status = $1, updated_at = NOW(), payment_id = $2 WHERE id = $3`
	result, update := r.db.ExecContext(ctx, query,
		*s,
		*payId,
		*id)

	if update != nil {
		return fmt.Errorf("update checkout session: %w", update)
	}
	rows, e := result.RowsAffected()
	if e != nil {
		return fmt.Errorf("checking rows affected: %w", e)
	}
	if rows == 0 {
		return fmt.Errorf("checkout session not found: %s", *id)
	}
	return nil
}

func (r *Repository) Close() error {
	return r.db.Close()
}

func (r *Repository) CompleteCheckoutSession(ctx context.Context, id *string, snapshot []byte, s *d.CheckoutStatus) error {
	txOpts := sql.TxOptions{Isolation: sql.LevelReadCommitted}
	tx, txe := r.db.BeginTx(ctx, &txOpts)
	if txe != nil {
		return fmt.Errorf("failed to start transaction: %w", txe)
	}
	defer tx.Rollback()
	query := `UPDATE checkout_sessions SET status = $1, updated_at = NOW() WHERE id = $2`
	result, update := tx.ExecContext(ctx, query,
		*s,
		*id)
	if update != nil {
		return fmt.Errorf("complete checkout session: %w", update)
	}
	rows, e := result.RowsAffected()
	if e != nil {
		return fmt.Errorf("complete rows affected: %w", e)
	}
	if rows == 0 {
		return fmt.Errorf("complete session not found: %s", *id)
	}

	query = `INSERT INTO outbox_events (aggregate_id, event_type, payload, created_at) 
               VALUES ($1, $2, $3, NOW())`

	result, update = tx.ExecContext(
		ctx,
		query,
		*id,
		"CheckoutCompleted",
		snapshot)

	if update != nil {
		return fmt.Errorf("complete checkout session on insert: %w", update)
	}
	err := tx.Commit()
	if err != nil {
		return fmt.Errorf("failed to commit transaction: %w", err)
	}
	return nil
}

func (r *Repository) GetUnprocessedEvents(ctx context.Context, limit int) ([]*OutboxEvent, error) {
	query := `SELECT * FROM outbox_events where processed_at is null limit $1`

	rows, err := r.db.QueryContext(ctx, query, limit)
	if err != nil {
		return nil, fmt.Errorf("failed to query events: %w", err)
	}
	defer rows.Close()

	var events []*OutboxEvent
	for rows.Next() {
		p := &OutboxEvent{}
		err := rows.Scan(
			&p.ID,
			&p.AggregateId,
			&p.EventType,
			&p.Payload,
			&p.CreatedAt,
			&p.ProcessedAt)
		if err != nil {
			return nil, fmt.Errorf("failed to scan event: %w", err)
		}
		events = append(events, p)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("row iteration error: %w", err)
	}

	return events, nil
}

func (r *Repository) MarkEventAsProcessed(ctx context.Context, id int) error {
	query := `UPDATE outbox_events SET processed_at = NOW() WHERE id = $1`
	result, update := r.db.ExecContext(ctx, query, id)

	if update != nil {
		return fmt.Errorf("update outbox_event: %w", update)
	}
	rows, e := result.RowsAffected()
	if e != nil {
		return fmt.Errorf("outbox_events rows affected: %w", e)
	}
	if rows == 0 {
		return fmt.Errorf("outbox_event not found: %d", id)
	}
	return nil
}

func (r *Repository) GetStuckSessions(ctx context.Context) ([]*CheckoutSession, error) {
	query := `
        SELECT cs.*
        FROM checkout_sessions cs
        LEFT JOIN outbox_events oe ON oe.aggregate_id = cs.id
        WHERE cs.status = 'PAYMENT_COMPLETED'
          AND cs.updated_at < NOW() - INTERVAL '5 minutes'  -- Grace period
          AND oe.id IS NULL  -- No outbox event exists
          `

	rows, err := r.db.QueryContext(ctx, query)
	if err != nil {
		return nil, fmt.Errorf("failed to query stuck sessions: %w", err)
	}
	defer rows.Close()

	var sessions []*CheckoutSession
	for rows.Next() {
		p := &CheckoutSession{}
		err := rows.Scan(
			&p.ID,
			&p.UserID,
			&p.CartSnapshot,
			&p.Status,
			&p.IdempotencyKey,
			&p.InventoryReservationID,
			&p.PaymentID,
			&p.TotalAmount,
			&p.Currency,
			&p.CreatedAt,
			&p.UpdatedAt)
		if err != nil {
			return nil, fmt.Errorf("failed to scan session: %w", err)
		}
		sessions = append(sessions, p)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("row iteration error: %w", err)
	}

	return sessions, nil
}
