package main

import (
	"database/sql"
	"flag"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"strings"
	"time"

	"github.com/Antimatterr/psygateway/internal/logger"
	"github.com/joho/godotenv"
	_ "github.com/lib/pq" // PostgreSQL driver
)

type Route struct {
	ID           int
	PathPattern  string
	ServiceName  string
	Method       string
	AuthRequired bool
	RateLimit    int
	CacheTTL     int
	TargetURL    string
	Enabled      bool
}

type Gateway struct {
	db         *sql.DB
	routes     []Route
	httpClient *http.Client
}

func NewGateway(dbURL string) (*Gateway, error) {
	db, err := sql.Open("postgres", dbURL)
	if err != nil {
		return nil, fmt.Errorf("failed to connect to database: %v", err)
	}

	if err := db.Ping(); err != nil {
		return nil, fmt.Errorf("failed to ping database: %v", err)
	}

	// Create HTTP client for proxying requests
	httpClient := &http.Client{
		Timeout: 30 * time.Second,
	}

	gateway := &Gateway{db: db, httpClient: httpClient}

	if err := gateway.loadRoutes(); err != nil {
		return nil, fmt.Errorf("failed to load routes: %v", err)
	}

	return gateway, nil
}

func (g *Gateway) loadRoutes() error {
	query := `
		SELECT id, path_pattern, service_name, method, auth_required, 
		       rate_limit, cache_ttl, target_url, enabled
		FROM routes 
		WHERE enabled = true
		ORDER BY path_pattern DESC`

	stmt, err := g.db.Prepare(query)
	if err != nil {
		logger.Error("Failed to prepare query", err)
		return fmt.Errorf("failed to prepare query: %v", err)
	}
	defer stmt.Close()

	rows, err := stmt.Query()
	if err != nil {
		logger.Error("Failed to execute query", err)
		return fmt.Errorf("failed to execute query: %v", err)
	}
	defer rows.Close()

	var routes []Route
	for rows.Next() {
		var r Route
		if err := rows.Scan(&r.ID, &r.PathPattern, &r.ServiceName, &r.Method, &r.AuthRequired,
			&r.RateLimit, &r.CacheTTL, &r.TargetURL, &r.Enabled); err != nil {
			logger.Error("Failed to scan row", err)
			return fmt.Errorf("failed to scan row: %v", err)
		}
		routes = append(routes, r)
	}

	if err := rows.Err(); err != nil {
		logger.Error("Error iterating over rows", err)
		return fmt.Errorf("error iterating over rows: %v", err)
	}

	g.routes = routes

	logger.Info("Loaded routes from database", len(routes))

	// Print routes for debugging
	for _, route := range routes {
		logger.Debug("Route details", route)
	}

	return nil
}

func (g *Gateway) findRoute(path, method string) (*Route, error) {
	logger.Debug("Finding route", "path", path, "method", method)

	for _, route := range g.routes {
		logger.Debug("Checking route", "pattern", route.PathPattern, "routeMethod", route.Method, "enabled", route.Enabled)

		// Check if method matches (or route accepts ANY method)
		if route.Method != "ANY" && route.Method != method {
			logger.Debug("Method mismatch: ", route.Method, "requestMethod", method)
			continue
		}

		// Check if path pattern matches
		if g.matchPattern(route.PathPattern, path) {
			logger.Debug("Route matched: ", route.PathPattern, "path", path)
			return &route, nil
		} else {
			logger.Debug("Pattern mismatch: ", route.PathPattern, "path", path)
		}
	}

	logger.Error("No route found", "path", path, "method", method)
	return nil, fmt.Errorf("route not found for path: %s, method: %s", path, method)
}

func (g *Gateway) matchPattern(pattern, path string) bool {
	if pattern == path {
		return true
	}

	if strings.HasSuffix(pattern, "/*") {
		prefix := strings.TrimSuffix(pattern, "/*")
		return strings.HasPrefix(path, prefix)
	}

	return false
}

func (g *Gateway) buildTargetURL(targetBaseURL, requestPath, routePattern string) (string, error) {
	baseUrl, err := url.Parse(targetBaseURL)
	if err != nil {
		logger.Error("Failed to parse target URL", err)
		return "", err
	}

	var targetPath string

	// Handle wildcard routes: /api/products/*
	if strings.HasSuffix(routePattern, "/*") {
		// Remove the /* from the pattern to get the prefix
		prefix := strings.TrimSuffix(routePattern, "/*")

		// If request path starts with the prefix, extract the remaining part
		if strings.HasPrefix(requestPath, prefix) {
			// Get the part after the prefix (e.g., "/123" from "/api/products/123")
			remainingPath := strings.TrimPrefix(requestPath, prefix)
			targetPath = remainingPath
		} else {
			// If somehow the path doesn't match the prefix, use the full path
			targetPath = requestPath
		}
	} else {
		// For exact matches, use the original request path
		targetPath = requestPath
	}

	// Use proper URL joining instead of string concatenation
	baseUrl.Path = strings.TrimSuffix(baseUrl.Path, "/") + targetPath

	return baseUrl.String(), nil
}

func (g *Gateway) copyHeaders(src, dst http.Header) {
	for key, values := range src {
		// Skip certain headers that shouldn't be forwarded
		if g.shouldSkipHeader(key) {
			continue
		}

		for _, value := range values {
			dst.Add(key, value)
		}
	}
}

func (g *Gateway) shouldSkipHeader(header string) bool {
	// Headers that shouldn't be forwarded
	skipHeaders := []string{
		"Connection",
		"Keep-Alive",
		"Proxy-Authenticate",
		"Proxy-Authorization",
		"Te",
		"Trailers",
		"Transfer-Encoding",
		"Upgrade",
	}

	header = strings.ToLower(header)
	for _, skip := range skipHeaders {
		if strings.ToLower(skip) == header {
			return true
		}
	}
	return false
}

func (g *Gateway) proxyRequest(w http.ResponseWriter, r *http.Request, route *Route) {

	targetUrl, err := g.buildTargetURL(route.TargetURL, r.URL.Path, route.PathPattern)
	if err != nil {
		http.Error(w, "Failed to build target URL", http.StatusInternalServerError)
		logger.Error("Failed to build target URL", err)
		return
	}
	logger.Info("Proxying request", "Method", r.Method, "Path", r.URL.Path, "Target URL", targetUrl)

	//create new request to backened service
	proxyRequest, err := http.NewRequest(r.Method, targetUrl, r.Body)
	if err != nil {
		http.Error(w, "Failed to create proxy request", http.StatusInternalServerError)
		logger.Error("Failed to create proxy request", err)
		return
	}
	g.copyHeaders(r.Header, proxyRequest.Header)

	// Add some gateway headers
	proxyRequest.Header.Set("X-Gateway", "api-gateway")
	proxyRequest.Header.Set("X-Forwarded-For", r.RemoteAddr)
	proxyRequest.Header.Set("X-Original-Host", r.Host)

	resp, err := g.httpClient.Do(proxyRequest)
	if err != nil {
		http.Error(w, "Failed to proxy request", http.StatusInternalServerError)
		logger.Error("Failed to proxy request", err)
		return
	}
	defer resp.Body.Close()

	g.copyHeaders(resp.Header, w.Header())
	w.WriteHeader(resp.StatusCode)

	// Copy response body back to client
	_, err = io.Copy(w, resp.Body)
	if err != nil {
		http.Error(w, "Failed to copy response body", http.StatusInternalServerError)
		logger.Error("Failed to copy response body", err)
		return
	}

}

func (g *Gateway) handleRequest(w http.ResponseWriter, r *http.Request) {
	logger.Debug("Received request", r.Method, r.URL.Path)
	route, err := g.findRoute(r.URL.Path, r.Method)
	if err != nil {
		http.Error(w, err.Error(), http.StatusNotFound)
		logger.Error("Route not found for handleRRequest", err)
		return
	}

	logger.Info("Matched route", route.PathPattern, "for method", r.Method)

	if route.ServiceName == "gateway" {
		g.handleGatewayEndpoint(w, r)
		return
	}

	if route.AuthRequired {
		if !g.checkAuth(r) {
			http.Error(w, "Unauthorized", http.StatusUnauthorized)
			logger.Error("Unauthorized access", fmt.Errorf("unauthorized access to route: %s", route.PathPattern))
			return
		}
	}

	// Check authentication if required
	if route.AuthRequired {
		if !g.checkAuth(r) {
			http.Error(w, "Authentication required", http.StatusUnauthorized)
			return
		}
	}

	// Proxy the request to the backend service
	g.proxyRequest(w, r, route)

}

// handleGatewayEndpoint handles requests for the gateway itself
func (g *Gateway) handleGatewayEndpoint(w http.ResponseWriter, r *http.Request) {
	switch r.URL.Path {
	case "/health":
		fmt.Fprint(w, "Gateway is healthy!")
	case "/routes":
		g.listRoutes(w)
	default:
		http.Error(w, "Not found", http.StatusNotFound)
		logger.Error("Unknown endpoint", fmt.Errorf("unknown endpoint: %s", r.URL.Path))
	}
}

func (g *Gateway) listRoutes(w http.ResponseWriter) {
	w.Header().Set("Content-Type", "text/plain")
	fmt.Fprintf(w, "Configured Routes (%d):\n\n", len(g.routes))

	for _, route := range g.routes {
		fmt.Fprintf(w, "Path: %s\n", route.PathPattern)
		fmt.Fprintf(w, "Service: %s\n", route.ServiceName)
		fmt.Fprintf(w, "Auth: %v\n", route.AuthRequired)
		fmt.Fprintf(w, "Target: %s\n", route.TargetURL)
		fmt.Fprintf(w, "---\n")
	}
}

func (g *Gateway) checkAuth(r *http.Request) bool {
	auth := r.Header.Get("Authorization")
	return auth != "" // Very basic check for now
}

func main() {

	verbose := flag.Bool("verbose", false, "Enable verbose logging")
	flag.Parse()
	if *verbose {
		logger.SetVerbose(true) // Enable verbose mode in dev
	} else {
		logger.SetVerbose(false) // Disable verbose mode in production
	}

	err := godotenv.Load()
	if err != nil {
		logger.Error("Error loading .env file", err)
	}

	// Get database connection parameters from environment variables
	user := os.Getenv("POSTGRES_USER")
	password := os.Getenv("POSTGRES_PASSWORD")
	host := os.Getenv("POSTGRES_HOST")
	port := os.Getenv("POSTGRES_PORT")
	dbname := os.Getenv("POSTGRES_DB")

	// Construct database URL
	dbURL := fmt.Sprintf("postgres://%s:%s@%s:%s/%s?sslmode=disable", user, password, host, port, dbname)

	// Create gateway instance
	gateway, err := NewGateway(dbURL)
	if err != nil {
		logger.Fatal("Failed to create gateway", err)
	}
	defer gateway.db.Close()

	// Set up HTTP server
	http.HandleFunc("/", gateway.handleRequest)

	logger.Info("Starting gateway server on :8000")

	if err := http.ListenAndServe(":8000", nil); err != nil {
		logger.Fatal("Server failed to start", err)
	}

}
