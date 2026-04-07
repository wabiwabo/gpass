package store

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"strings"
	"time"
)

// PostgresAuditStore is a PostgreSQL-backed implementation of AuditStore.
// It satisfies 12factor Factor VI (stateless processes) and PP 71/2019
// 5-year audit retention requirement via append-only audit_events table.
type PostgresAuditStore struct {
	db *sql.DB
}

// NewPostgresAuditStore creates a new PostgreSQL-backed audit store.
func NewPostgresAuditStore(db *sql.DB) *PostgresAuditStore {
	return &PostgresAuditStore{db: db}
}

// Append inserts a new audit event. Append-only: no UPDATE or DELETE.
func (s *PostgresAuditStore) Append(event *AuditEvent) error {
	if err := ValidateEvent(event); err != nil {
		return err
	}

	if event.ActorType == "" {
		event.ActorType = "USER"
	}
	if event.Status == "" {
		event.Status = "SUCCESS"
	}
	if event.Metadata == nil {
		event.Metadata = make(map[string]string)
	}

	metadataJSON, err := json.Marshal(event.Metadata)
	if err != nil {
		return fmt.Errorf("marshal metadata: %w", err)
	}

	query := `
		INSERT INTO audit_events (
			event_type, actor_id, actor_type, resource_id, resource_type,
			action, metadata, ip_address, user_agent, service_name,
			request_id, status
		) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12)
		RETURNING id, created_at`

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	return s.db.QueryRowContext(ctx, query,
		event.EventType,
		event.ActorID,
		event.ActorType,
		nullString(event.ResourceID),
		nullString(event.ResourceType),
		event.Action,
		metadataJSON,
		nullString(event.IPAddress),
		nullString(event.UserAgent),
		event.ServiceName,
		nullString(event.RequestID),
		event.Status,
	).Scan(&event.ID, &event.CreatedAt)
}

// GetByID retrieves a single event by its ID.
func (s *PostgresAuditStore) GetByID(id string) (*AuditEvent, error) {
	query := `
		SELECT id, event_type, actor_id, actor_type,
			COALESCE(resource_id, ''), COALESCE(resource_type, ''),
			action, metadata,
			COALESCE(ip_address, ''), COALESCE(user_agent, ''),
			service_name, COALESCE(request_id, ''), status, created_at
		FROM audit_events
		WHERE id = $1`

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	var e AuditEvent
	var metadataJSON []byte

	err := s.db.QueryRowContext(ctx, query, id).Scan(
		&e.ID, &e.EventType, &e.ActorID, &e.ActorType,
		&e.ResourceID, &e.ResourceType, &e.Action, &metadataJSON,
		&e.IPAddress, &e.UserAgent, &e.ServiceName, &e.RequestID,
		&e.Status, &e.CreatedAt,
	)
	if err == sql.ErrNoRows {
		return nil, fmt.Errorf("audit event not found: %s", id)
	}
	if err != nil {
		return nil, fmt.Errorf("get audit event: %w", err)
	}

	if err := json.Unmarshal(metadataJSON, &e.Metadata); err != nil {
		return nil, fmt.Errorf("unmarshal metadata: %w", err)
	}

	return &e, nil
}

// Query retrieves events matching the filter criteria with pagination.
func (s *PostgresAuditStore) Query(filter AuditFilter) ([]*AuditEvent, error) {
	limit := filter.Limit
	if limit <= 0 {
		limit = 100
	}
	if limit > 1000 {
		limit = 1000
	}

	conditions, args := buildFilterConditions(filter)
	whereClause := ""
	if len(conditions) > 0 {
		whereClause = "WHERE " + strings.Join(conditions, " AND ")
	}

	query := fmt.Sprintf(`
		SELECT id, event_type, actor_id, actor_type,
			COALESCE(resource_id, ''), COALESCE(resource_type, ''),
			action, metadata,
			COALESCE(ip_address, ''), COALESCE(user_agent, ''),
			service_name, COALESCE(request_id, ''), status, created_at
		FROM audit_events
		%s
		ORDER BY created_at DESC
		LIMIT $%d OFFSET $%d`, whereClause, len(args)+1, len(args)+2)

	args = append(args, limit, filter.Offset)

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	rows, err := s.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("query audit events: %w", err)
	}
	defer rows.Close()

	var events []*AuditEvent
	for rows.Next() {
		var e AuditEvent
		var metadataJSON []byte
		if err := rows.Scan(
			&e.ID, &e.EventType, &e.ActorID, &e.ActorType,
			&e.ResourceID, &e.ResourceType, &e.Action, &metadataJSON,
			&e.IPAddress, &e.UserAgent, &e.ServiceName, &e.RequestID,
			&e.Status, &e.CreatedAt,
		); err != nil {
			return nil, fmt.Errorf("scan audit event: %w", err)
		}
		if err := json.Unmarshal(metadataJSON, &e.Metadata); err != nil {
			return nil, fmt.Errorf("unmarshal metadata: %w", err)
		}
		events = append(events, &e)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate audit events: %w", err)
	}

	return events, nil
}

// Count returns the number of events matching the filter.
func (s *PostgresAuditStore) Count(filter AuditFilter) (int64, error) {
	conditions, args := buildFilterConditions(filter)
	whereClause := ""
	if len(conditions) > 0 {
		whereClause = "WHERE " + strings.Join(conditions, " AND ")
	}

	query := "SELECT COUNT(*) FROM audit_events " + whereClause

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	var count int64
	if err := s.db.QueryRowContext(ctx, query, args...).Scan(&count); err != nil {
		return 0, fmt.Errorf("count audit events: %w", err)
	}
	return count, nil
}

// buildFilterConditions translates an AuditFilter into parameterized SQL conditions.
func buildFilterConditions(f AuditFilter) ([]string, []interface{}) {
	var conditions []string
	var args []interface{}
	idx := 1

	addEq := func(column string, value string) {
		if value != "" {
			conditions = append(conditions, fmt.Sprintf("%s = $%d", column, idx))
			args = append(args, value)
			idx++
		}
	}

	addEq("actor_id", f.ActorID)
	addEq("resource_id", f.ResourceID)
	addEq("resource_type", f.ResourceType)
	addEq("event_type", f.EventType)
	addEq("action", f.Action)
	addEq("service_name", f.ServiceName)
	addEq("status", f.Status)

	if !f.From.IsZero() {
		conditions = append(conditions, fmt.Sprintf("created_at >= $%d", idx))
		args = append(args, f.From)
		idx++
	}
	if !f.To.IsZero() {
		conditions = append(conditions, fmt.Sprintf("created_at <= $%d", idx))
		args = append(args, f.To)
		idx++
	}

	return conditions, args
}

func nullString(s string) interface{} {
	if s == "" {
		return nil
	}
	return s
}
