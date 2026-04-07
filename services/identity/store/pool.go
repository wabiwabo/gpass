package store

import (
	"database/sql"
	"os"
	"strconv"
	"time"
)

// Connection-pool defaults tuned for enterprise workloads. Each value can be
// overridden via environment variables to allow per-deployment tuning without
// recompilation (12factor Factor III).
const (
	defaultMaxOpenConns    = 25
	defaultMaxIdleConns    = 10
	defaultConnMaxLifetime = 30 * time.Minute
	defaultConnMaxIdleTime = 5 * time.Minute
)

// configurePool applies enterprise pool defaults to db, with optional
// per-service overrides via DB_MAX_OPEN_CONNS / DB_MAX_IDLE_CONNS /
// DB_CONN_MAX_LIFETIME_SEC / DB_CONN_MAX_IDLE_TIME_SEC.
func configurePool(db *sql.DB) {
	db.SetMaxOpenConns(envInt("DB_MAX_OPEN_CONNS", defaultMaxOpenConns))
	db.SetMaxIdleConns(envInt("DB_MAX_IDLE_CONNS", defaultMaxIdleConns))
	db.SetConnMaxLifetime(envDuration("DB_CONN_MAX_LIFETIME_SEC", defaultConnMaxLifetime))
	db.SetConnMaxIdleTime(envDuration("DB_CONN_MAX_IDLE_TIME_SEC", defaultConnMaxIdleTime))
}

func envInt(key string, def int) int {
	if v := os.Getenv(key); v != "" {
		if n, err := strconv.Atoi(v); err == nil && n > 0 {
			return n
		}
	}
	return def
}

func envDuration(key string, def time.Duration) time.Duration {
	if v := os.Getenv(key); v != "" {
		if n, err := strconv.Atoi(v); err == nil && n > 0 {
			return time.Duration(n) * time.Second
		}
	}
	return def
}
