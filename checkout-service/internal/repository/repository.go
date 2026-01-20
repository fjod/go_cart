package repository

import (
	"database/sql"
	"errors"
	"fmt"

	"github.com/golang-migrate/migrate/v4"
	"github.com/golang-migrate/migrate/v4/database/postgres"
	_ "github.com/golang-migrate/migrate/v4/source/file"
	_ "github.com/lib/pq"
)

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
