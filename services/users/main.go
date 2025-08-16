package main

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"strconv"
	"syscall"
	"time"

	"github.com/Antimatterr/psygateway/internal/discovery"
	"github.com/Antimatterr/psygateway/internal/logger"
	"github.com/joho/godotenv"
	_ "github.com/lib/pq"
)

type User struct {
	ID       int    `json:"id"`
	Username string `json:"username"`
	Email    string `json:"email"`
	Role     string `json:"role"`
	Status   string `json:"status"`
}

var db *sql.DB

func init() {
	// Load environment variables
	err := godotenv.Load(".env")
	if err != nil {
		log.Printf("Error loading .env file: %v", err)
	}

	// Get database connection parameters from environment variables
	user := os.Getenv("POSTGRES_USER")
	password := os.Getenv("POSTGRES_PASSWORD")
	host := os.Getenv("POSTGRES_HOST")
	port := os.Getenv("POSTGRES_PORT")
	dbname := os.Getenv("POSTGRES_DB")

	// Construct database URL
	dbURL := fmt.Sprintf("postgres://%s:%s@%s:%s/%s?sslmode=disable", user, password, host, port, dbname)

	db, err = sql.Open("postgres", dbURL)
	if err != nil {
		logger.Fatal("Failed to connect to DB", err)
	}
	if err = db.Ping(); err != nil {
		logger.Fatal("Failed to ping DB", err)
	}

	logger.Info("Connected to database", fmt.Sprintf("%s:%s/%s", host, port, dbname))
}

func getUsers(w http.ResponseWriter, r *http.Request) {
	rows, err := db.Query("SELECT id, username, email, role, status FROM users")
	if err != nil {
		http.Error(w, "Failed to fetch users", http.StatusInternalServerError)
		logger.Error("Failed to fetch users", err)
		return
	}
	defer rows.Close()

	var users []User
	for rows.Next() {
		var user User
		if err := rows.Scan(&user.ID, &user.Username, &user.Email, &user.Role, &user.Status); err != nil {
			http.Error(w, "Failed to scan user", http.StatusInternalServerError)
			logger.Error("Failed to scan user", err)
			return
		}
		users = append(users, user)
	}
	logger.Info("Fetched users", fmt.Sprintf("Count: %d", len(users)))
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(users)
}

func getUserByID(w http.ResponseWriter, r *http.Request) {
	idStr := r.URL.Path[len("/api/user/"):]
	id, err := strconv.Atoi(idStr)
	if err != nil {
		http.Error(w, "Invalid user ID", http.StatusBadRequest)
		logger.Error("Invalid user ID", err)
		return
	}
	var user User
	err = db.QueryRow("SELECT id, username, email, role, status FROM users WHERE id=$1", id).Scan(&user.ID, &user.Username, &user.Email, &user.Role, &user.Status)
	if err == sql.ErrNoRows {
		http.Error(w, "User not found", http.StatusNotFound)
		logger.Error("User not found", fmt.Errorf("user ID %d does not exist", id))
		return
	} else if err != nil {
		http.Error(w, "Failed to fetch user", http.StatusInternalServerError)
		logger.Error("Failed to fetch user", err)
		return
	}
	logger.Info("Fetched user", fmt.Sprintf("ID: %d, Username: %s", user.ID, user.Username))
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(user)
}

func healthCheck(w http.ResponseWriter, r *http.Request) {
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(map[string]string{"status": "ok"})
	logger.Info("Health check", "User service is running")
}

func main() {
	http.HandleFunc("/api/users", getUsers)
	http.HandleFunc("/api/user/", getUserByID)
	http.HandleFunc("/api/user/health", healthCheck)

	userPort := os.Getenv("USER_SERVICE_PORT")
	consulAddress := os.Getenv("CONSUL_ADDRESS")

	//service discovery client
	sd, err := discovery.NewServiceDiscovery(consulAddress)

	if err != nil {
		logger.Fatal("Failed to create service discovery client", err)
	}

	if userPort == "" {
		logger.Fatal("USER_SERVICE_PORT environment variable is not set", fmt.Errorf("USER_SERVICE_PORT is empty"))
	}

	// Convert port string to int
	port, err := strconv.Atoi(userPort)
	if err != nil {
		logger.Fatal("Invalid USER_SERVICE_PORT", err)
	}

	// Register this service with Consul
	err = sd.RegisterService(
		"user-service",
		"user-service", // Use container name for Docker networking
		port,           // Use the actual port from environment
		"/api/user/health",
	)
	if err != nil {
		logger.Fatal("Failed to register user service with Consul", err)
	}
	logger.Info("User service registered with Consul", "port", port)

	// Create HTTP server
	server := &http.Server{Addr: ":" + userPort}

	// Handle graceful shutdown
	go func() {
		sigChan := make(chan os.Signal, 1)
		signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)
		<-sigChan

		logger.Info("Shutting down user service...")

		// Deregister from Consul
		if err := sd.DeregisterService("user-service"); err != nil {
			logger.Error("Failed to deregister service", err)
		}

		// Shutdown server
		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()
		server.Shutdown(ctx)
	}()

	logger.Info("Starting user service", "port", userPort)
	if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
		logger.Fatal("Server failed to start", err)
	}

}
