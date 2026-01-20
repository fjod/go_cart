package main

import (
	"log"
	"os"
	"strconv"

	"github.com/fjod/go_cart/checkout-service/internal/repository"
)

func getEnv(key, defaultValue string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return defaultValue
}

func main() {
	log.Println("checkout-service started")

	dbHost := getEnv("DB_HOST", "localhost")
	dbPort := getEnv("DB_PORT", "5432")
	dbUser := getEnv("DB_USER", "postgres")
	dbPass := getEnv("DB_PASSWORD", "postgres")
	dbName := getEnv("DB_NAME", "ecommerce")
	migrationsPath := getEnv("MIGRATIONS_PATH", "./internal/repository/migrations")

	port, err := strconv.Atoi(dbPort)
	if err != nil {
		log.Fatalf("Invalid DB_PORT: %v", err)
	}
	creds := &repository.Credentials{
		Host:              dbHost,
		Port:              port,
		User:              dbUser,
		Password:          dbPass,
		DBName:            dbName,
		MigrationsDirPath: migrationsPath,
	}

	repo, err := repository.NewRepository(creds)
	if err != nil {
		log.Fatal(err)
	}
	defer repo.Close()

	if err := repo.RunMigrations(creds); err != nil {
		log.Fatalf("Failed to run migrations: %v", err)
	}

	log.Println("Migrations completed successfully")

	log.Println("checkout-service exited")
}
