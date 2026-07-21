package main

import (
	"context"
	"log"
	"os"
	"time"

	"github.com/restorte/lzhuff-store/internal/db"
)

func main() {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	dsn := os.Getenv("DATABASE_URL")
	if dsn == "" {
		log.Fatal("DATABASE_URL is not set")
	}

	pool, err := db.New(ctx, dsn)
	if err != nil {
		log.Fatal(err)
	}
	defer pool.Close()
	log.Println("connected to database")

}
