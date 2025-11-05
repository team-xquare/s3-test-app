package db

import (
	"crypto/sha256"
	"database/sql"
	"fmt"
	"sync"

	_ "github.com/mattn/go-sqlite3"
	"s3-test-app/internal/auth"
)

// Database handles all database operations
type Database struct {
	conn *sql.DB
	mu   sync.RWMutex
}

// User represents a user record
type User struct {
	ID       string
	Username string
	Email    string
	Password string // hashed
	Role     auth.Role
}

// New creates a new database connection
func New(dbPath string) (*Database, error) {
	conn, err := sql.Open("sqlite3", dbPath)
	if err != nil {
		return nil, fmt.Errorf("failed to open database: %w", err)
	}

	if err := conn.Ping(); err != nil {
		return nil, fmt.Errorf("failed to ping database: %w", err)
	}

	db := &Database{conn: conn}

	// Initialize schema
	if err := db.initSchema(); err != nil {
		return nil, fmt.Errorf("failed to initialize schema: %w", err)
	}

	return db, nil
}

// Close closes the database connection
func (d *Database) Close() error {
	return d.conn.Close()
}

// initSchema creates tables if they don't exist
func (d *Database) initSchema() error {
	schema := `
	CREATE TABLE IF NOT EXISTS users (
		id TEXT PRIMARY KEY,
		username TEXT UNIQUE NOT NULL,
		email TEXT UNIQUE NOT NULL,
		password TEXT NOT NULL,
		role TEXT NOT NULL,
		created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
		updated_at DATETIME DEFAULT CURRENT_TIMESTAMP
	);

	CREATE UNIQUE INDEX IF NOT EXISTS idx_users_username ON users(username);
	CREATE UNIQUE INDEX IF NOT EXISTS idx_users_email ON users(email);
	`

	if _, err := d.conn.Exec(schema); err != nil {
		return fmt.Errorf("failed to execute schema: %w", err)
	}

	return nil
}

// GetUserByUsername retrieves a user by username
func (d *Database) GetUserByUsername(username string) (*User, error) {
	d.mu.RLock()
	defer d.mu.RUnlock()

	var user User
	err := d.conn.QueryRow(
		`SELECT id, username, email, password, role FROM users WHERE username = ?`,
		username,
	).Scan(&user.ID, &user.Username, &user.Email, &user.Password, &user.Role)

	if err != nil {
		if err == sql.ErrNoRows {
			return nil, fmt.Errorf("user not found")
		}
		return nil, fmt.Errorf("failed to get user: %w", err)
	}

	return &user, nil
}

// GetUserByID retrieves a user by ID
func (d *Database) GetUserByID(id string) (*User, error) {
	d.mu.RLock()
	defer d.mu.RUnlock()

	var user User
	err := d.conn.QueryRow(
		`SELECT id, username, email, password, role FROM users WHERE id = ?`,
		id,
	).Scan(&user.ID, &user.Username, &user.Email, &user.Password, &user.Role)

	if err != nil {
		if err == sql.ErrNoRows {
			return nil, fmt.Errorf("user not found")
		}
		return nil, fmt.Errorf("failed to get user: %w", err)
	}

	return &user, nil
}

// GetAllUsers retrieves all users
func (d *Database) GetAllUsers() ([]*User, error) {
	d.mu.RLock()
	defer d.mu.RUnlock()

	rows, err := d.conn.Query(
		`SELECT id, username, email, password, role FROM users ORDER BY created_at DESC`,
	)
	if err != nil {
		return nil, fmt.Errorf("failed to query users: %w", err)
	}
	defer rows.Close()

	var users []*User
	for rows.Next() {
		var user User
		if err := rows.Scan(&user.ID, &user.Username, &user.Email, &user.Password, &user.Role); err != nil {
			return nil, fmt.Errorf("failed to scan user: %w", err)
		}
		users = append(users, &user)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("error iterating users: %w", err)
	}

	return users, nil
}

// CreateUser creates a new user
func (d *Database) CreateUser(id, username, email, password string, role auth.Role) error {
	d.mu.Lock()
	defer d.mu.Unlock()

	// Hash password
	hashedPassword := hashPassword(password)

	_, err := d.conn.Exec(
		`INSERT INTO users (id, username, email, password, role) VALUES (?, ?, ?, ?, ?)`,
		id, username, email, hashedPassword, role,
	)

	if err != nil {
		return fmt.Errorf("failed to create user: %w", err)
	}

	return nil
}

// DeleteUser deletes a user
func (d *Database) DeleteUser(id string) error {
	d.mu.Lock()
	defer d.mu.Unlock()

	result, err := d.conn.Exec(`DELETE FROM users WHERE id = ?`, id)
	if err != nil {
		return fmt.Errorf("failed to delete user: %w", err)
	}

	rowsAffected, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("failed to get rows affected: %w", err)
	}

	if rowsAffected == 0 {
		return fmt.Errorf("user not found")
	}

	return nil
}

// UpdateUserRole updates a user's role
func (d *Database) UpdateUserRole(id string, role auth.Role) error {
	d.mu.Lock()
	defer d.mu.Unlock()

	result, err := d.conn.Exec(
		`UPDATE users SET role = ?, updated_at = CURRENT_TIMESTAMP WHERE id = ?`,
		role, id,
	)

	if err != nil {
		return fmt.Errorf("failed to update user: %w", err)
	}

	rowsAffected, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("failed to get rows affected: %w", err)
	}

	if rowsAffected == 0 {
		return fmt.Errorf("user not found")
	}

	return nil
}

// VerifyPassword checks if the provided password matches the user's password
func VerifyPassword(hashedPassword, plainPassword string) bool {
	return hashPassword(plainPassword) == hashedPassword
}

// hashPassword hashes a password using SHA256
func hashPassword(password string) string {
	hash := sha256.Sum256([]byte(password))
	return fmt.Sprintf("%x", hash)
}