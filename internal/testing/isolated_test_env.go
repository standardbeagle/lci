package testing

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
	"github.com/standardbeagle/lci/internal/config"
)

// IsolatedTestEnv provides a real temporary directory with controlled gitignore for integration testing
type IsolatedTestEnv struct {
	t      *testing.T
	tempDir string
	config  *config.Config
}

// NewIsolatedTestEnv creates a temporary directory with controlled gitignore patterns
func NewIsolatedTestEnv(t *testing.T, gitignorePatterns ...string) *IsolatedTestEnv {
	tempDir := t.TempDir()

	// Create controlled .gitignore if patterns provided
	if len(gitignorePatterns) > 0 {
		gitignorePath := filepath.Join(tempDir, ".gitignore")
		gitignoreContent := strings.Join(gitignorePatterns, "\n")
		require.NoError(t, os.WriteFile(gitignorePath, []byte(gitignoreContent), 0644))
	}

	return &IsolatedTestEnv{
		t:       t,
		tempDir: tempDir,
		config: &config.Config{
			Project: config.Project{
				Root: tempDir,
			},
			Include: []string{"**/*"}, // Include everything, let gitignore handle exclusions
			Exclude: []string{},       // No explicit excludes, rely on gitignore
			Index: config.Index{
				MaxFileSize:      10 * 1024 * 1024, // 10MB
				RespectGitignore: true,              // Enable gitignore processing
			},
			Performance: config.Performance{
				MaxMemoryMB: 2000,
				MaxGoroutines: 8,
			},
		},
	}
}

// NewIsolatedTestEnvWithConfig creates a test environment with custom configuration
func NewIsolatedTestEnvWithConfig(t *testing.T, cfg *config.Config) *IsolatedTestEnv {
	tempDir := t.TempDir()

	// Ensure the config uses the temp directory as root
	if cfg.Project.Root == "" {
		cfg.Project.Root = tempDir
	}

	return &IsolatedTestEnv{
		t:       t,
		tempDir: tempDir,
		config:  cfg,
	}
}

// TempDir returns the temporary directory path
func (ite *IsolatedTestEnv) TempDir() string {
	return ite.tempDir
}

// Config returns the test configuration
func (ite *IsolatedTestEnv) Config() *config.Config {
	return ite.config
}

// WriteFile creates a file in the test environment
func (ite *IsolatedTestEnv) WriteFile(path string, content string) {
	fullPath := filepath.Join(ite.tempDir, path)

	// Create parent directories if they don't exist
	dir := filepath.Dir(fullPath)
	require.NoError(ite.t, os.MkdirAll(dir, 0755))

	require.NoError(ite.t, os.WriteFile(fullPath, []byte(content), 0644))
}

// WriteFileBytes creates a file with binary content
func (ite *IsolatedTestEnv) WriteFileBytes(path string, content []byte) {
	fullPath := filepath.Join(ite.tempDir, path)

	// Create parent directories if they don't exist
	dir := filepath.Dir(fullPath)
	require.NoError(ite.t, os.MkdirAll(dir, 0755))

	require.NoError(ite.t, os.WriteFile(fullPath, content, 0644))
}

// MkdirAll creates a directory (and any necessary parents)
func (ite *IsolatedTestEnv) MkdirAll(path string) {
	fullPath := filepath.Join(ite.tempDir, path)
	require.NoError(ite.t, os.MkdirAll(fullPath, 0755))
}

// Exists checks if a file or directory exists
func (ite *IsolatedTestEnv) Exists(path string) bool {
	fullPath := filepath.Join(ite.tempDir, path)
	_, err := os.Stat(fullPath)
	return err == nil
}

// ReadFile reads the content of a file
func (ite *IsolatedTestEnv) ReadFile(path string) string {
	fullPath := filepath.Join(ite.tempDir, path)
	content, err := os.ReadFile(fullPath)
	require.NoError(ite.t, err)
	return string(content)
}

// ReadFileBytes reads binary content of a file
func (ite *IsolatedTestEnv) ReadFileBytes(path string) []byte {
	fullPath := filepath.Join(ite.tempDir, path)
	content, err := os.ReadFile(fullPath)
	require.NoError(ite.t, err)
	return content
}

// AppendToFile appends content to an existing file
func (ite *IsolatedTestEnv) AppendToFile(path string, content string) {
	fullPath := filepath.Join(ite.tempDir, path)
	file, err := os.OpenFile(fullPath, os.O_APPEND|os.O_WRONLY|os.O_CREATE, 0644)
	require.NoError(ite.t, err)
	defer file.Close()

	_, err = file.WriteString(content)
	require.NoError(ite.t, err)
}

// SetGitignore creates or updates the .gitignore file
func (ite *IsolatedTestEnv) SetGitignore(patterns ...string) {
	gitignorePath := filepath.Join(ite.tempDir, ".gitignore")
	gitignoreContent := strings.Join(patterns, "\n")
	require.NoError(ite.t, os.WriteFile(gitignorePath, []byte(gitignoreContent), 0644))
}

// AddToGitignore appends patterns to the existing .gitignore file
func (ite *IsolatedTestEnv) AddToGitignore(patterns ...string) {
	gitignorePath := filepath.Join(ite.tempDir, ".gitignore")

	var existingContent string
	if content, err := os.ReadFile(gitignorePath); err == nil {
		existingContent = string(content)
		if !strings.HasSuffix(existingContent, "\n") {
			existingContent += "\n"
		}
	}

	newContent := existingContent + strings.Join(patterns, "\n")
	require.NoError(ite.t, os.WriteFile(gitignorePath, []byte(newContent), 0644))
}

// GetGitignore returns the current .gitignore content
func (ite *IsolatedTestEnv) GetGitignore() string {
	gitignorePath := filepath.Join(ite.tempDir, ".gitignore")
	if content, err := os.ReadFile(gitignorePath); err == nil {
		return string(content)
	}
	return ""
}

// ListFiles returns a list of all files in the test environment (recursively)
func (ite *IsolatedTestEnv) ListFiles() []string {
	var files []string

	err := filepath.Walk(ite.tempDir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}

		// Skip the root directory itself
		if path == ite.tempDir {
			return nil
		}

		// Get relative path
		relPath, err := filepath.Rel(ite.tempDir, path)
		require.NoError(ite.t, err)

		files = append(files, filepath.ToSlash(relPath))
		return nil
	})

	require.NoError(ite.t, err)
	return files
}

// CreateRealNodeProject creates a realistic Node.js project structure
func (ite *IsolatedTestEnv) CreateRealNodeProject() {
	ite.AddToGitignore(
		".next/",
		".DS_Store",
	)

	// Source files
	ite.WriteFile("src/app.js", `
const express = require('express');
const path = require('path');

const app = express();
const PORT = process.env.PORT || 3000;

// Middleware
app.use(express.json());
app.use(express.static('public'));

// Routes
app.get('/', (req, res) => {
    res.sendFile(path.join(__dirname, 'public/index.html'));
});

app.get('/api/health', (req, res) => {
    res.json({ status: 'ok', timestamp: new Date().toISOString() });
});

// Start server
app.listen(PORT, () => {
    console.log("Server running on port " + PORT);
});

module.exports = app;
`)

	ite.WriteFile("src/utils/logger.js", `
class Logger {
    constructor(level = 'info') {
        this.level = level;
    }

    log(message, level = 'info') {
        const timestamp = new Date().toISOString();
        console.log("[" + timestamp + "] [" + level.toUpperCase() + "] " + message);
    }

    info(message) {
        this.log(message, 'info');
    }

    error(message) {
        this.log(message, 'error');
    }

    warn(message) {
        this.log(message, 'warn');
    }
}

module.exports = Logger;
`)

	ite.WriteFile("src/config/database.js", `
const config = {
    development: {
        host: 'localhost',
        port: 5432,
        database: 'app_dev',
        username: 'dev_user',
        password: 'dev_pass'
    },
    production: {
        host: process.env.DB_HOST,
        port: process.env.DB_PORT || 5432,
        database: process.env.DB_NAME,
        username: process.env.DB_USER,
        password: process.env.DB_PASSWORD
    }
};

const env = process.env.NODE_ENV || 'development';
module.exports = config[env];
`)

	// Configuration files
	ite.WriteFile("package.json", `{
  "name": "real-test-project",
  "version": "1.0.0",
  "description": "A realistic Node.js project for testing",
  "main": "src/app.js",
  "scripts": {
    "start": "node src/app.js",
    "dev": "nodemon src/app.js",
    "test": "jest",
    "build": "webpack --mode production",
    "lint": "eslint src/",
    "format": "prettier --write src/"
  },
  "dependencies": {
    "express": "^4.18.2",
    "cors": "^2.8.5",
    "helmet": "^6.0.1",
    "morgan": "^1.10.0"
  },
  "devDependencies": {
    "nodemon": "^2.0.20",
    "jest": "^29.0.0",
    "eslint": "^8.0.0",
    "prettier": "^2.8.0",
    "webpack": "^5.75.0"
  },
  "engines": {
    "node": ">=16.0.0"
  }
}`)

	ite.WriteFile(".eslintrc.json", `{
  "env": {
    "node": true,
    "es2021": true,
    "jest": true
  },
  "extends": ["eslint:recommended"],
  "parserOptions": {
    "ecmaVersion": 12,
    "sourceType": "module"
  },
  "rules": {
    "no-console": "warn",
    "no-unused-vars": "error"
  }
}`)

	// Test files
	ite.MkdirAll("tests/unit")
	ite.WriteFile("tests/unit/app.test.js", `
const request = require('supertest');
const app = require('../../src/app');

describe('App', () => {
    describe('GET /', () => {
        it('should return the index page', async () => {
            const response = await request(app)
                .get('/')
                .expect(200);

            expect(response.headers['content-type']).toMatch(/html/);
        });
    });

    describe('GET /api/health', () => {
        it('should return health status', async () => {
            const response = await request(app)
                .get('/api/health')
                .expect(200);

            expect(response.body).toHaveProperty('status', 'ok');
            expect(response.body).toHaveProperty('timestamp');
        });
    });
});
`)

	ite.WriteFile("tests/utils/logger.test.js", `
const Logger = require('../../src/utils/logger');

describe('Logger', () => {
    let consoleSpy;

    beforeEach(() => {
        consoleSpy = jest.spyOn(console, 'log').mockImplementation();
    });

    afterEach(() => {
        consoleSpy.mockRestore();
    });

    test('should log info messages', () => {
        const logger = new Logger('info');
        logger.info('Test message');

        expect(consoleSpy).toHaveBeenCalledWith(
            expect.stringContaining('[INFO] Test message')
        );
    });

    test('should log error messages', () => {
        const logger = new Logger('error');
        logger.error('Error message');

        expect(consoleSpy).toHaveBeenCalledWith(
            expect.stringContaining('[ERROR] Error message')
        );
    });
});
`)

	// Public files
	ite.MkdirAll("public")
	ite.WriteFile("public/index.html", `<!DOCTYPE html>
<html lang="en">
<head>
    <meta charset="UTF-8">
    <meta name="viewport" content="width=device-width, initial-scale=1.0">
    <title>Real Test Project</title>
    <link rel="stylesheet" href="/styles.css">
</head>
<body>
    <div id="root">
        <h1>Welcome to Real Test Project</h1>
        <p>This is a realistic project for testing LCI functionality.</p>
    </div>
    <script src="/app.js"></script>
</body>
</html>`)

	ite.WriteFile("public/styles.css", `
body {
    font-family: Arial, sans-serif;
    margin: 0;
    padding: 20px;
    background-color: #f5f5f5;
}

#root {
    max-width: 800px;
    margin: 0 auto;
    background: white;
    padding: 20px;
    border-radius: 8px;
    box-shadow: 0 2px 10px rgba(0,0,0,0.1);
}

h1 {
    color: #333;
    text-align: center;
}
`)

	// Environment files
	ite.WriteFile(".env.example", `# Environment variables
NODE_ENV=development
PORT=3000

# Database
DB_HOST=localhost
DB_PORT=5432
DB_NAME=app_dev
DB_USER=dev_user
DB_PASSWORD=dev_pass

# API Keys
API_KEY=your_api_key_here
JWT_SECRET=your_jwt_secret_here
`)

	// Documentation
	ite.WriteFile("README.md", "# Real Test Project\n\n"+
		"This is a realistic Node.js project created for testing Lightning Code Index functionality.\n\n"+
		"## Features\n\n"+
		"- Express.js web server\n"+
		"- RESTful API endpoints\n"+
		"- Static file serving\n"+
		"- Comprehensive logging\n"+
		"- Environment-based configuration\n"+
		"- Unit tests with Jest\n"+
		"- ESLint configuration\n"+
		"- Webpack build system\n\n"+
		"## Getting Started\n\n"+
		"```bash\n"+
		"npm install\n"+
		"npm run dev\n"+
		"```\n\n"+
		"## Testing\n\n"+
		"```bash\n"+
		"npm test\n"+
		"npm run lint\n"+
		"```\n")

	// Create some files that should be excluded by gitignore
	ite.MkdirAll("node_modules/express/lib")
	ite.WriteFile("node_modules/express/index.js", "// Express library content")
	ite.WriteFile("debug.log", "2023-01-01 12:00:00 - Application started")
	ite.WriteFile(".env.local", "SECRET=local_secret_value")
}

// CreateRealGoProject creates a realistic Go project structure
func (ite *IsolatedTestEnv) CreateRealGoProject() {
	ite.SetGitignore(
		"vendor/",
		"*.exe",
		"*.exe~",
		"*.dll",
		"*.so",
		"*.dylib",
		"*.test",
		"*.out",
		"coverage.out",
		"*.prof",
	)

	// Main application
	ite.WriteFile("main.go", `
package main

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"real-go-project/internal/api"
	"real-go-project/internal/config"
	"real-go-project/internal/database"
	"real-go-project/pkg/logger"
)

func main() {
	// Load configuration
	cfg, err := config.Load()
	if err != nil {
		log.Fatalf("Failed to load configuration: %v", err)
	}

	// Initialize logger
	log := logger.New(cfg.LogLevel)
	log.Info("Starting application")

	// Initialize database
	db, err := database.New(cfg.Database)
	if err != nil {
		log.Fatalf("Failed to initialize database: %v", err)
	}
	defer db.Close()

	// Setup HTTP server
	server := api.NewServer(cfg, db, log)

	// Graceful shutdown
	go func() {
		sigChan := make(chan os.Signal, 1)
		signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)
		<-sigChan

		log.Info("Shutting down server...")
		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()

		if err := server.Shutdown(ctx); err != nil {
			log.Printf("Server shutdown error: %v", err)
		}
	}()

	// Start server
	addr := fmt.Sprintf(":%d", cfg.Server.Port)
	log.Printf("Server listening on %s", addr)
	if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
		log.Fatalf("Server error: %v", err)
	}

	log.Info("Application stopped")
}
`)

	// Configuration
	ite.MkdirAll("internal/config")
	ite.WriteFile("internal/config/config.go", `
package config

import (
	"encoding/json"
	"fmt"
	"os"
	"strconv"
)

type Config struct {
	Server   ServerConfig   ` + "`" + `json:"server"` + "`" + `
	Database DatabaseConfig ` + "`" + `json:"database"` + "`" + `
	LogLevel string         ` + "`" + `json:"log_level"` + "`" + `
}

type ServerConfig struct {
	Port int ` + "`" + `json:"port"` + "`" + `
	Host string ` + "`" + `json:"host"` + "`" + `
}

type DatabaseConfig struct {
	Host     string ` + "`" + `json:"host"` + "`" + `
	Port     int    ` + "`" + `json:"port"` + "`" + `
	Database string ` + "`" + `json:"database"` + "`" + `
	Username string ` + "`" + `json:"username"` + "`" + `
	Password string ` + "`" + `json:"password"` + "`" + `
	SSLMode  string ` + "`" + `json:"ssl_mode"` + "`" + `
}

func Load() (*Config, error) {
	// Default configuration
	cfg := &Config{
		Server: ServerConfig{
			Port: 8080,
			Host: "0.0.0.0",
		},
		Database: DatabaseConfig{
			Host:     "localhost",
			Port:     5432,
			Database: "real_go_project",
			Username: "postgres",
			Password: "postgres",
			SSLMode:  "disable",
		},
		LogLevel: "info",
	}

	// Load from file if exists
	if configFile := os.Getenv("CONFIG_FILE"); configFile != "" {
		data, err := os.ReadFile(configFile)
		if err != nil {
			return nil, fmt.Errorf("failed to read config file: %w", err)
		}
		if err := json.Unmarshal(data, cfg); err != nil {
			return nil, fmt.Errorf("failed to parse config file: %w", err)
		}
	}

	// Override with environment variables
	if port := os.Getenv("PORT"); port != "" {
		if p, err := strconv.Atoi(port); err == nil {
			cfg.Server.Port = p
		}
	}

	if logLevel := os.Getenv("LOG_LEVEL"); logLevel != "" {
		cfg.LogLevel = logLevel
	}

	return cfg, nil
}
`)

	// Database layer
	ite.MkdirAll("internal/database")
	ite.WriteFile("internal/database/database.go", `
package database

import (
	"database/sql"
	"fmt"
	"time"

	_ "github.com/lib/pq"
	"real-go-project/internal/config"
)

type Database struct {
	db *sql.DB
}

type User struct {
	ID        int       ` + "`" + `json:"id"` + "`" + `
	Username  string    ` + "`" + `json:"username"` + "`" + `
	Email     string    ` + "`" + `json:"email"` + "`" + `
	CreatedAt time.Time ` + "`" + `json:"created_at"` + "`" + `
	UpdatedAt time.Time ` + "`" + `json:"updated_at"` + "`" + `
}

func New(cfg config.DatabaseConfig) (*Database, error) {
	dsn := fmt.Sprintf("host=%s port=%d user=%s password=%s dbname=%s sslmode=%s",
		cfg.Host, cfg.Port, cfg.Username, cfg.Password, cfg.Database, cfg.SSLMode)

	db, err := sql.Open("postgres", dsn)
	if err != nil {
		return nil, fmt.Errorf("failed to connect to database: %w", err)
	}

	// Test connection
	if err := db.Ping(); err != nil {
		return nil, fmt.Errorf("failed to ping database: %w", err)
	}

	// Set connection pool settings
	db.SetMaxOpenConns(25)
	db.SetMaxIdleConns(25)
	db.SetConnMaxLifetime(5 * time.Minute)

	return &Database{db: db}, nil
}

func (d *Database) Close() error {
	return d.db.Close()
}

func (d *Database) GetUser(id int) (*User, error) {
	var user User
	query := "SELECT id, username, email, created_at, updated_at FROM users WHERE id = $1"

	err := d.db.QueryRow(query, id).Scan(
		&user.ID,
		&user.Username,
		&user.Email,
		&user.CreatedAt,
		&user.UpdatedAt,
	)

	if err != nil {
		return nil, fmt.Errorf("failed to get user: %w", err)
	}

	return &user, nil
}

func (d *Database) CreateUser(user *User) error {
	query := "INSERT INTO users (username, email, created_at, updated_at) VALUES ($1, $2, NOW(), NOW()) RETURNING id"

	err := d.db.QueryRow(query, user.Username, user.Email).Scan(&user.ID)
	if err != nil {
		return fmt.Errorf("failed to create user: %w", err)
	}

	return nil
}
`)

	// Logger
	ite.MkdirAll("pkg/logger")
	ite.WriteFile("pkg/logger/logger.go", `
package logger

import (
	"log"
	"os"
)

type Logger interface {
	Info(msg string)
	Error(msg string)
	Warn(msg string)
	Debug(msg string)
}

type standardLogger struct {
	infoLogger  *log.Logger
	errorLogger *log.Logger
	warnLogger  *log.Logger
	debugLogger *log.Logger
}

func New(level string) Logger {
	flags := log.Ldate | log.Ltime | log.Lshortfile

	return &standardLogger{
		infoLogger:  log.New(os.Stdout, "[INFO] ", flags),
		errorLogger: log.New(os.Stderr, "[ERROR] ", flags),
		warnLogger:  log.New(os.Stdout, "[WARN] ", flags),
		debugLogger: log.New(os.Stdout, "[DEBUG] ", flags),
	}
}

func (l *standardLogger) Info(msg string) {
	l.infoLogger.Println(msg)
}

func (l *standardLogger) Error(msg string) {
	l.errorLogger.Println(msg)
}

func (l *standardLogger) Warn(msg string) {
	l.warnLogger.Println(msg)
}

func (l *standardLogger) Debug(msg string) {
	l.debugLogger.Println(msg)
}
`)

	// API layer
	ite.MkdirAll("internal/api")
	ite.WriteFile("internal/api/server.go", `
package api

import (
	"encoding/json"
	"net/http"
	"strconv"

	"real-go-project/internal/config"
	"real-go-project/internal/database"
	"real-go-project/pkg/logger"
)

type Server struct {
	config  *config.Config
	db      *database.Database
	logger  logger.Logger
	handler *http.ServeMux
}

type UserResponse struct {
	ID       int    ` + "`" + `json:"id"` + "`" + `
	Username string ` + "`" + `json:"username"` + "`" + `
	Email    string ` + "`" + `json:"email"` + "`" + `
}

type CreateUserRequest struct {
	Username string ` + "`" + `json:"username" validate:"required,min=3,max=50"` + "`" + `
	Email    string ` + "`" + `json:"email" validate:"required,email"` + "`" + `
}

func NewServer(cfg *config.Config, db *database.Database, logger logger.Logger) *Server {
	s := &Server{
		config: cfg,
		db:     db,
		logger: logger,
	}

	s.setupRoutes()
	return s
}

func (s *Server) setupRoutes() {
	s.handler = http.NewServeMux()

	s.handler.HandleFunc("/health", s.healthCheck)
	s.handler.HandleFunc("/api/users", s.handleUsers)
	s.handler.HandleFunc("/api/users/", s.handleUserByID)
}

func (s *Server) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	s.handler.ServeHTTP(w, r)
}

func (s *Server) Shutdown(ctx context.Context) error {
	s.logger.Info("Shutting down HTTP server")
	return nil
}

func (s *Server) healthCheck(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{
		"status": "ok",
		"service": "real-go-project",
	})
}

func (s *Server) handleUsers(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		s.listUsers(w, r)
	case http.MethodPost:
		s.createUser(w, r)
	default:
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
	}
}

func (s *Server) handleUserByID(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	idStr := r.URL.Path[len("/api/users/"):]
	id, err := strconv.Atoi(idStr)
	if err != nil {
		http.Error(w, "Invalid user ID", http.StatusBadRequest)
		return
	}

	user, err := s.db.GetUser(id)
	if err != nil {
		http.Error(w, "User not found", http.StatusNotFound)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(UserResponse{
		ID:       user.ID,
		Username: user.Username,
		Email:    user.Email,
	})
}

func (s *Server) listUsers(w http.ResponseWriter, r *http.Request) {
	// Implementation would list users
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode([]UserResponse{})
}

func (s *Server) createUser(w http.ResponseWriter, r *http.Request) {
	var req CreateUserRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	user := &database.User{
		Username: req.Username,
		Email:    req.Email,
	}

	if err := s.db.CreateUser(user); err != nil {
		http.Error(w, "Failed to create user", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	json.NewEncoder(w).Encode(UserResponse{
		ID:       user.ID,
		Username: user.Username,
		Email:    user.Email,
	})
}
`)

	// Go module file
	ite.WriteFile("go.mod", `module real-go-project

go 1.21

require (
	github.com/lib/pq v1.10.9
	github.com/stretchr/testify v1.8.4
)

require (
	github.com/davecgh/go-spew v1.1.1 // indirect
	github.com/pmezard/go-difflib v1.0.0 // indirect
	golang.org/x/xerrors v0.0.0-20220907171357-2be1bf30a55a // indirect
	gopkg.in/yaml.v3 v3.0.1 // indirect
)`)

	// Tests
	ite.MkdirAll("tests/integration")
	ite.WriteFile("tests/integration/api_test.go", `
package integration

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"real-go-project/internal/api"
	"real-go-project/internal/config"
	"real-go-project/internal/database"
	"real-go-project/pkg/logger"
)

func TestAPIEndpoints(t *testing.T) {
	// Setup test configuration
	cfg := &config.Config{
		Server: config.ServerConfig{Port: 0}, // Random port
		Database: config.DatabaseConfig{
			Host:     "localhost",
			Port:     5432,
			Database: "test_db",
			Username: "test_user",
			Password: "test_pass",
			SSLMode:  "disable",
		},
		LogLevel: "error", // Minimize log output in tests
	}

	// Mock database for testing
	db := &database.Database{} // Would use a mock in real tests

	// Create server
	server := api.NewServer(cfg, db, logger.New("error"))

	// Test health check
	t.Run("Health Check", func(t *testing.T) {
		req := httptest.NewRequest("GET", "/health", nil)
		w := httptest.NewRecorder()

		server.ServeHTTP(w, req)

		assert.Equal(t, http.StatusOK, w.Code)

		var response map[string]string
		err := json.Unmarshal(w.Body.Bytes(), &response)
		require.NoError(t, err)
		assert.Equal(t, "ok", response["status"])
		assert.Equal(t, "real-go-project", response["service"])
	})
}
`)

	// Create some files that should be excluded by gitignore
	ite.MkdirAll("vendor/github.com/lib/pq")
	ite.WriteFile("vendor/github.com/lib/pq/conn.go", "// PostgreSQL driver content")
	ite.WriteFile("main.exe", "// Windows executable")
	ite.WriteFile("coverage.out", "mode: atomic")
	ite.WriteFile("test.prof", "// Profile data")
}