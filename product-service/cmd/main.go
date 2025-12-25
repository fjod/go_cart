package main

import (
	"database/sql"
	"log"

	repository "github.com/fjod/go_cart/product-service/internal/db"
)

func main() {
	log.Println("Product-service started")
	db, err := sql.Open("sqlite", "./internal/db/products.db")
	if err != nil {
		log.Fatal(err)
	}
	defer db.Close()

	// Run migrations
	if err := repository.RunMigrations(db, "./internal/db/migrations"); err != nil {
		log.Fatalf("Failed to run migrations: %v", err)
	}

	log.Println("Migrations completed successfully")
}
