package main

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os"
	"strconv"

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
		log.Fatalf("Failed to connect to DB: %v", err)
	}
	if err = db.Ping(); err != nil {
		log.Fatalf("Failed to ping DB: %v", err)
	}

	log.Printf("Connected to database: %s:%s/%s", host, port, dbname)
}

func getUsers(w http.ResponseWriter, r *http.Request) {
	rows, err := db.Query("SELECT id, username, email, role, status FROM users")
	if err != nil {
		http.Error(w, "Failed to fetch users", http.StatusInternalServerError)
		return
	}
	defer rows.Close()

	var users []User
	for rows.Next() {
		var user User
		if err := rows.Scan(&user.ID, &user.Username, &user.Email, &user.Role, &user.Status); err != nil {
			http.Error(w, "Failed to scan user", http.StatusInternalServerError)
			return
		}
		users = append(users, user)
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(users)
}

func getUserByID(w http.ResponseWriter, r *http.Request) {
	idStr := r.URL.Path[len("/api/user/"):]
	id, err := strconv.Atoi(idStr)
	if err != nil {
		http.Error(w, "Invalid user ID", http.StatusBadRequest)
		return
	}
	var user User
	err = db.QueryRow("SELECT id, username, email, role, status FROM users WHERE id=$1", id).Scan(&user.ID, &user.Username, &user.Email, &user.Role, &user.Status)
	if err == sql.ErrNoRows {
		http.Error(w, "User not found", http.StatusNotFound)
		return
	} else if err != nil {
		http.Error(w, "Failed to fetch user", http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(user)
}

func main() {
	http.HandleFunc("/api/users", getUsers)
	http.HandleFunc("/api/user/", getUserByID)
	userPort := os.Getenv("USER_SERVICE_PORT")
	if userPort == "" {
		logger.Fatal("USER_SERVICE_PORT environment variable is not set")
	}

	if err := http.ListenAndServe(":"+userPort, nil); err != nil {
		logger.Fatal("Server failed to start", err)
	}
}
