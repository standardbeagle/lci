package main

import (
	"log"
	"net/http"

	"github.com/example/complex-project/internal/api"
	"github.com/example/complex-project/internal/config"
	"github.com/example/complex-project/pkg/database"
	"github.com/gorilla/mux"
)

func main() {
	// Load configuration
	cfg, err := config.Load()
	if err != nil {
		log.Fatalf("Failed to load config: %v", err)
	}

	// Initialize database
	db, err := database.Connect(cfg.DatabaseURL)
	if err != nil {
		log.Fatalf("Failed to connect to database: %v", err)
	}
	defer db.Close()

	// Set up API handlers
	router := mux.NewRouter()
	apiHandler := api.NewHandler(db)

	// Register routes
	router.PathPrefix("/api/").Handler(http.StripPrefix("/api", apiHandler.Router()))

	// Serve static files for frontend
	router.PathPrefix("/").Handler(http.FileServer(http.Dir("./frontend/dist/")))

	log.Printf("Server starting on port %d", cfg.Port)
	log.Fatal(http.ListenAndServe(fmt.Sprintf(":%d", cfg.Port), router))
}
