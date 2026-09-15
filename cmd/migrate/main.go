package main

import (
	"context"
	"fmt"
	"github.com/D9veth/RBPO/internal/database"
	"os"
	"time"
)

func main() {
	ctx, cancel := context.WithTimeout(context.Background(), time.Minute)
	defer cancel()
	pool, err := database.Open(ctx, os.Getenv("DATABASE_URL"))
	if err == nil {
		defer pool.Close()
		err = database.Migrate(ctx, pool)
	}
	if err != nil {
		fmt.Fprintln(os.Stderr, "Migration failed:", err)
		os.Exit(1)
	}
	fmt.Println("Migrations applied.")
}
