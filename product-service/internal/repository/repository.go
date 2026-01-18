package repository

import (
	"context"
	"database/sql"
	"errors"
	"fmt"

	"github.com/fjod/go_cart/product-service/internal/domain"
	"github.com/golang-migrate/migrate/v4"
	"github.com/golang-migrate/migrate/v4/database/sqlite"
	_ "github.com/golang-migrate/migrate/v4/source/file"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	_ "modernc.org/sqlite"
)

type Repository struct {
	db *sql.DB
}

type RepoInterface interface {
	GetAllProducts(ctx context.Context) ([]*domain.Product, error)
	GetProduct(ctx context.Context, id int64) (*domain.Product, error)
	Close() error
	RunMigrations(string) error
}

func (r *Repository) RunMigrations(migrationsPath string) error {
	driver, err := sqlite.WithInstance(r.db, &sqlite.Config{})
	if err != nil {
		return fmt.Errorf("could not create migration driver: %w", err)
	}

	m, err := migrate.NewWithDatabaseInstance(
		fmt.Sprintf("file://%s", migrationsPath),
		"sqlite",
		driver,
	)
	if err != nil {
		return fmt.Errorf("could not create migrate instance: %w", err)
	}

	if err := m.Up(); err != nil && !errors.Is(err, migrate.ErrNoChange) {
		return fmt.Errorf("could not run migrations: %w", err)
	}

	return nil
}

func NewRepository(dbPath string) (*Repository, error) {
	db, err := sql.Open("sqlite", dbPath)
	if err != nil {
		return nil, fmt.Errorf("failed to open database: %w", err)
	}

	if err := db.Ping(); err != nil {
		return nil, fmt.Errorf("failed to ping database: %w", err)
	}

	return &Repository{db: db}, nil
}

func (r *Repository) GetAllProducts(ctx context.Context) ([]*domain.Product, error) {
	query := `
		SELECT id, name, description, price, image_url, created_at
		FROM products
		ORDER BY id
	`

	rows, err := r.db.QueryContext(ctx, query)
	if err != nil {
		return nil, fmt.Errorf("failed to query products: %w", err)
	}
	defer rows.Close()

	var products []*domain.Product
	for rows.Next() {
		p := &domain.Product{}
		err := rows.Scan(
			&p.ID,
			&p.Name,
			&p.Description,
			&p.Price,
			&p.ImageURL,
			&p.CreatedAt,
		)
		if err != nil {
			return nil, fmt.Errorf("failed to scan product: %w", err)
		}
		products = append(products, p)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("row iteration error: %w", err)
	}

	return products, nil
}

func (r *Repository) GetProduct(ctx context.Context, id int64) (*domain.Product, error) {
	query := `
		SELECT id, name, description, price, image_url, created_at
		FROM products
		WHERE id = $1
	`

	rows, err := r.db.QueryContext(ctx, query, id)

	if err != nil {
		return nil, fmt.Errorf("failed to query products: %w", err)
	}
	defer rows.Close()

	var product *domain.Product
	for rows.Next() {
		p := &domain.Product{}
		err := rows.Scan(
			&p.ID,
			&p.Name,
			&p.Description,
			&p.Price,
			&p.ImageURL,
			&p.CreatedAt,
		)
		if err != nil {
			return nil, fmt.Errorf("failed to scan product: %w", err)
		}
		product = p
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("row iteration error: %w", err)
	}

	if product == nil {
		return nil, status.Error(codes.NotFound, "product not found")
	}
	return product, nil
}

func (r *Repository) Close() error {
	return r.db.Close()
}
