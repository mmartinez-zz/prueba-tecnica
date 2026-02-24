package main

import (
	"fmt"
	"log"
	"net/http"
	"os"

	"github.com/go-chi/chi/v5"
	chimiddleware "github.com/go-chi/chi/v5/middleware"
	"github.com/rs/cors"

	"github.com/KemenyStudio/task-manager/internal/db"
	"github.com/KemenyStudio/task-manager/internal/handler"
	"github.com/KemenyStudio/task-manager/internal/middleware"
)

// NOTE: No graceful shutdown implemented.
// The server will terminate abruptly on SIGINT/SIGTERM,
// potentially leaving in-flight requests incomplete.
func main() {
	// Connect to database
	if err := db.Connect(); err != nil {
		log.Fatalf("Failed to connect to database: %v", err)
	}
	defer db.Close()

	r := chi.NewRouter()

	// Middleware
	r.Use(chimiddleware.Logger)
	r.Use(chimiddleware.Recoverer)
	r.Use(chimiddleware.RequestID)

	// CORS
	corsHandler := cors.New(cors.Options{
		AllowedOrigins:   []string{"http://localhost:3000"},
		AllowedMethods:   []string{"GET", "POST", "PUT", "DELETE", "OPTIONS"},
		AllowedHeaders:   []string{"Accept", "Authorization", "Content-Type"},
		AllowCredentials: true,
	})
	r.Use(corsHandler.Handler)

	// Health check
	r.Get("/health", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(`{"status": "ok"}`))
	})

	// Public routes
	r.Post("/api/auth/login", handler.LoginHandler)

	// Protected routes
	r.Route("/api", func(r chi.Router) {
		r.Use(middleware.AuthMiddleware)

		// Tasks CRUD
		r.Get("/tasks", handler.ListTasks)
		r.Post("/tasks", handler.CreateTask)
		r.Get("/tasks/{id}", handler.GetTask)
		r.Put("/tasks/{id}", handler.UpdateTask)
		r.Delete("/tasks/{id}", handler.DeleteTask)

		// Task extras
		r.Get("/tasks/{id}/history", handler.GetTaskHistory)
		r.Get("/tasks/search", handler.SearchTasks)
		r.Post("/tasks/{id}/classify", handler.ClassifyTask)

		// Dashboard
		r.Get("/dashboard/stats", handler.GetDashboardStats)
	})

	port := os.Getenv("PORT")
	if port == "" {
		port = "8080"
	}

	addr := fmt.Sprintf(":%s", port)
	log.Printf("Server starting on %s", addr)

	// NOTE: No graceful shutdown — server just dies on SIGINT
	if err := http.ListenAndServe(addr, r); err != nil {
		log.Fatalf("Server failed: %v", err)
	}
}
