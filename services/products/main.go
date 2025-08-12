package main

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"strconv"
	"time"

	"github.com/Antimatterr/psygateway/internal/logger"
	"github.com/joho/godotenv"
	_ "github.com/lib/pq"
)

type Product struct {
	ID            int       `json:"id"`
	Name          string    `json:"name"`
	Description   string    `json:"description"`
	Price         float64   `json:"price"`
	StockQuantity int       `json:"stock_quantity"`
	Category      string    `json:"category"`
	Status        string    `json:"status"`
	CreatedAt     time.Time `json:"created_at"`
	UpdatedAt     time.Time `json:"updated_at"`
}

var db *sql.DB

func init() {
	// Load environment variables
	err := godotenv.Load(".env")
	if err != nil {
		// Try loading from current directory
		err = godotenv.Load(".env")
		if err != nil {
			logger.Warn("Could not load .env file, using system environment variables", err)
		}
	}

	// Get database connection parameters from environment variables
	user := os.Getenv("POSTGRES_USER")
	password := os.Getenv("POSTGRES_PASSWORD")
	host := os.Getenv("POSTGRES_HOST")
	port := os.Getenv("POSTGRES_PORT")
	dbname := os.Getenv("POSTGRES_DB")

	dbURL := fmt.Sprintf("postgres://%s:%s@%s:%s/%s?sslmode=disable", user, password, host, port, dbname)

	db, err = sql.Open("postgres", dbURL)
	if err != nil {
		logger.Fatal("Failed to connect to database", err)
	}

	if err = db.Ping(); err != nil {
		logger.Fatal("Failed to ping database", err)
	}

	logger.Info("Connected to database", "host", host, "port", port, "dbname", dbname)
}

func getProducts() ([]Product, error) {
	query := `SELECT id, name, description, price, stock_quantity, 
	category, status, created_at, updated_at 
	FROM products`

	stmt, err := db.Prepare(query)
	if err != nil {
		logger.Error("Failed to prepare query", err)
	}
	defer stmt.Close()

	rows, err := stmt.Query()

	if err != nil {
		logger.Error("Failed to execute query", err)
		return nil, fmt.Errorf("failed to execute query: %v", err)
	}

	defer rows.Close()

	var products []Product

	for rows.Next() {
		var product Product
		if err := rows.Scan(&product.ID, &product.Name, &product.Description, &product.Price,
			&product.StockQuantity, &product.Category, &product.Status,
			&product.CreatedAt, &product.UpdatedAt); err != nil {
			logger.Error("Failed to scan row", err)
			return nil, fmt.Errorf("failed to scan row: %v", err)
		}
		products = append(products, product)
	}

	if err := rows.Err(); err != nil {
		logger.Error("Error iterating over rows", err)
		return nil, fmt.Errorf("error iterating over rows: %v", err)
	}

	logger.Info("Fetched products", "count", len(products))
	if len(products) == 0 {
		logger.Warn("No products found in the database")
	}

	for _, product := range products {
		logger.Debug("Product details", "ID", product.ID, "Name", product.Name, "Price", product.Price, "Category", product.Category)
	}

	return products, nil
}

func handleGetProducts(w http.ResponseWriter, r *http.Request) {
	products, err := getProducts()
	if err != nil {
		http.Error(w, "Failed to fetch products", http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(products)
}

func handleGetProductById(w http.ResponseWriter, r *http.Request) {
	idStr := r.URL.Path[len("/api/product/"):]
	id, err := strconv.Atoi(idStr)
	if err != nil {
		http.Error(w, "Invalid product ID", http.StatusBadRequest)
		logger.Error("Invalid product ID format", "ID", idStr, "Error", err)
		return
	}
	var product Product
	queryById := `SELECT id, name, description, price, stock_quantity, category, status, created_at, updated_at 
	FROM products WHERE id = $1`

	stmt, err := db.Prepare(queryById)
	if err != nil {
		logger.Error("Failed to prepare query", err)
		http.Error(w, "Internal server error", http.StatusInternalServerError)
		return
	}
	defer stmt.Close()

	err = stmt.QueryRow(id).Scan(&product.ID, &product.Name, &product.Description, &product.Price,
		&product.StockQuantity, &product.Category, &product.Status, &product.CreatedAt, &product.UpdatedAt)
	if err == sql.ErrNoRows {
		http.Error(w, "Product not found", http.StatusNotFound)
		logger.Warn("Product not found for: ", id)
		return
	}
	if err != nil {
		logger.Error("Failed to fetch product by ID", err)
		http.Error(w, "Internal server error", http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	logger.Info("Fetched product by ID", product.ID, "Name", product.Name)
	json.NewEncoder(w).Encode(product)
}

func healthCheck(w http.ResponseWriter, r *http.Request) {
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(map[string]string{"status": "ok"})
	logger.Info("Health check", "Product service is running")
}

func main() {
	http.HandleFunc("/api/products", handleGetProducts)
	http.HandleFunc("/api/product/", handleGetProductById)
	http.HandleFunc("/api/product/health", healthCheck)

	productPort := os.Getenv("PRODUCT_SERVICE_PORT")
	if productPort == "" {
		logger.Fatal("PRODUCT_SERVICE_PORT environment variable is not set")
	}
	logger.Info("Starting products service on :", productPort)
	if err := http.ListenAndServe(":"+productPort, nil); err != nil {
		logger.Fatal("Server failed to start", err)
	}
	logger.Info("Products service started successfully")
	logger.Info("Listening on port", productPort)
}
