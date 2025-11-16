package main

import (
	"Backend/internal/api"
	"Backend/internal/repository/postgres"
	"Backend/internal/service"
	"context"
	"database/sql"
	"fmt"
	"log"
	"net/http"
	"os"
	"time"

	_ "github.com/lib/pq"
)

func main() {
	ctx := context.Background()

	log.Println("Initializing PostgreSQL Repository...")

	dbHost := os.Getenv("DB_HOST")
	dbPort := os.Getenv("DB_PORT")
	dbUser := os.Getenv("DB_USER")
	dbPass := os.Getenv("DB_PASSWORD")
	dbName := os.Getenv("DB_NAME")

	if dbHost == "" || dbName == "" {
		log.Fatal("FATAL: Required environment variables (DB_HOST, DB_NAME, etc.) for PostgreSQL are not set.")
	}

	connStr := fmt.Sprintf("host=%s port=%s user=%s password=%s dbname=%s sslmode=disable",
		dbHost, dbPort, dbUser, dbPass, dbName)

	db, err := sql.Open("postgres", connStr)
	if err != nil {
		log.Fatalf("FATAL: Failed to open DB connection: %v", err)
	}

	ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()

	if err = db.PingContext(ctx); err != nil {
		log.Fatalf("FATAL: Failed to ping database: %v. Check docker-compose and environment variables.", err)
	}

	pgRepo := postgres.NewPostgresRepository(db)
	if err = pgRepo.Init(context.Background()); err != nil {
		log.Fatalf("FATAL: Failed to initialize PostgreSQL schema: %v", err)
	}

	log.Println("PostgreSQL connection established and schema initialized.")

	repoImpl := pgRepo

	prService := service.NewPRService(repoImpl, repoImpl)
	teamService := service.NewTeamService(repoImpl)
	userService := service.NewUserService(repoImpl, repoImpl)

	prHandler := api.NewPRHandler(prService)
	teamHandler := api.NewTeamHandler(teamService)
	userHandler := api.NewUserHandler(userService)

	r := api.NewRouter(prHandler, teamHandler, userHandler)

	server := &http.Server{
		Addr:    ":8080",
		Handler: r,
	}

	log.Printf("Server starting on port 8080. Repository: PostgreSQL")
	if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
		log.Fatalf("Could not listen on :8080: %v\n", err)
	}
}
