package storage

import (
	"context"
	"database/sql"
	"fmt"
	"strings"

	"github.com/google/uuid"
)

// AuditStore handles append-only audit log writes and tenant-scoped queries.
type AuditStore struct {
	db Querier
}

// NewAuditStore creates a new AuditStore backed by the given connection pool.
func NewAuditStore(db Querier) *AuditStore {
	return &AuditStore{
		db: db,
	}
}

// Insert appends an immutable audit log entry. Generates an ID if not set.
func (s *AuditStore) Insert(ctx context.Context, entry *AuditLogEntry) error {
	const query = `
		INSERT INTO audit_log (
			id,
			tenant_id,
			actor_id,
			actor_type,
			action,
			resource_type,
			resource_id,
			changes,
			ip_address,
			user_agent
		)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10)
		RETURNING created_at
	`
	if entry.ID == uuid.Nil {
		entry.ID = uuid.New()
	}
	var ip any
	if entry.IPAddress != nil {
		ip = *entry.IPAddress
	}
	err := s.db.QueryRow(
		ctx,
		query,
		entry.ID,
		entry.TenantID,
		entry.ActorID,
		entry.ActorType,
		entry.Action,
		entry.ResourceType,
		entry.ResourceID,
		entry.Changes,
		ip,
		entry.UserAgent,
	).Scan(&entry.CreatedAt)
	if err != nil {
		return fmt.Errorf("storage.AuditStore.Insert: %w", err)
	}
	return nil
}

// Query returns a paginated tenant-scoped audit log result set.
func (s *AuditStore) Query(ctx context.Context, filter AuditLogFilter) (*AuditLogPage, error) {
	const countQuery = `
		SELECT COUNT(*)
		FROM audit_log
		WHERE tenant_id = $1
		  AND ($2::uuid IS NULL OR actor_id = $2)
		  AND ($3::varchar IS NULL OR actor_type = $3)
		  AND ($4::varchar IS NULL OR action = $4)
		  AND ($5::varchar IS NULL OR resource_type = $5)
		  AND ($6::uuid IS NULL OR resource_id = $6)
		  AND ($7::timestamptz IS NULL OR created_at >= $7)
		  AND ($8::timestamptz IS NULL OR created_at <= $8)
	`
	const listQuery = `
		SELECT
			id,
			tenant_id,
			actor_id,
			actor_type,
			action,
			resource_type,
			resource_id,
			changes,
			host(ip_address),
			user_agent,
			created_at
		FROM audit_log
		WHERE tenant_id = $1
		  AND ($2::uuid IS NULL OR actor_id = $2)
		  AND ($3::varchar IS NULL OR actor_type = $3)
		  AND ($4::varchar IS NULL OR action = $4)
		  AND ($5::varchar IS NULL OR resource_type = $5)
		  AND ($6::uuid IS NULL OR resource_id = $6)
		  AND ($7::timestamptz IS NULL OR created_at >= $7)
		  AND ($8::timestamptz IS NULL OR created_at <= $8)
		ORDER BY created_at DESC
		LIMIT $9 OFFSET $10
	`
	limit := filter.Limit
	if limit <= 0 {
		limit = 20
	}
	if limit > 100 {
		limit = 100
	}
	offset := filter.Offset
	if offset < 0 {
		offset = 0
	}
	actorType := trimmedStringOrNil(filter.ActorType)
	action := trimmedStringOrNil(filter.Action)
	resourceType := trimmedStringOrNil(filter.ResourceType)
	args := []any{
		filter.TenantID,
		filter.ActorID,
		actorType,
		action,
		resourceType,
		filter.ResourceID,
		filter.Since,
		filter.Until,
	}
	var total int64
	if err := s.db.QueryRow(ctx, countQuery, args...).Scan(&total); err != nil {
		return nil, fmt.Errorf("storage.AuditStore.Query: count: %w", err)
	}
	rows, err := s.db.Query(ctx, listQuery, append(args, limit, offset)...)
	if err != nil {
		return nil, fmt.Errorf("storage.AuditStore.Query: list: %w", err)
	}
	defer rows.Close()
	entries := make([]AuditLogEntry, 0)
	for rows.Next() {
		var (
			entry      AuditLogEntry
			actorID    uuid.NullUUID
			resourceID uuid.NullUUID
			changes    []byte
			ipText     sql.NullString
			userAgent  sql.NullString
		)
		if err := rows.Scan(
			&entry.ID,
			&entry.TenantID,
			&actorID,
			&entry.ActorType,
			&entry.Action,
			&entry.ResourceType,
			&resourceID,
			&changes,
			&ipText,
			&userAgent,
			&entry.CreatedAt,
		); err != nil {
			return nil, fmt.Errorf("storage.AuditStore.Query: scan row: %w", err)
		}
		entry.ActorID = uuidPtrFromNull(actorID)
		entry.ResourceID = uuidPtrFromNull(resourceID)
		entry.Changes = rawMessagePtrFromBytes(changes)
		entry.IPAddress = ipPtrFromNull(ipText)
		entry.UserAgent = stringPtrFromNull(userAgent)
		entries = append(entries, entry)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("storage.AuditStore.Query: iterate rows: %w", err)
	}
	return &AuditLogPage{
		Entries: entries,
		Total:   total,
		Limit:   limit,
		Offset:  offset,
	}, nil
}
func trimmedStringOrNil(v *string) any {
	if v == nil {
		return nil
	}
	trimmed := strings.TrimSpace(*v)
	if trimmed == "" {
		return nil
	}
	return trimmed
}
