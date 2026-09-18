package repository

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/domain"
	"github.com/Wei-Shaw/sub2api/internal/service"
)

type upstreamRepository struct{ db *sql.DB }

func NewUpstreamRepository(db *sql.DB) service.UpstreamRepository { return &upstreamRepository{db: db} }

func (r *upstreamRepository) List(ctx context.Context) ([]*service.Upstream, error) {
	rows, err := r.db.QueryContext(ctx, upstreamSelect+" WHERE deleted_at IS NULL ORDER BY sort_code, id")
	if err != nil {
		return nil, fmt.Errorf("list upstreams: %w", err)
	}
	defer func() { _ = rows.Close() }()
	var result []*service.Upstream
	for rows.Next() {
		item, err := scanUpstream(rows)
		if err != nil {
			return nil, err
		}
		result = append(result, item)
	}
	return result, rows.Err()
}

func (r *upstreamRepository) GetByID(ctx context.Context, id int64) (*service.Upstream, error) {
	item, err := scanUpstream(r.db.QueryRowContext(ctx, upstreamSelect+" WHERE id = $1 AND deleted_at IS NULL", id))
	if errors.Is(err, sql.ErrNoRows) {
		return nil, service.ErrUpstreamNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("get upstream: %w", err)
	}
	return item, nil
}

func (r *upstreamRepository) Create(ctx context.Context, u *service.Upstream) error {
	return r.db.QueryRowContext(ctx, `INSERT INTO upstreams
			(name, sort_code, kind, base_url, token_encrypted, refresh_token_encrypted, password_encrypted, token_expires_at, login_identifier, remote_user_id, balance_snapshot, group_snapshot, notes, enabled, created_by)
			VALUES ($1,$2,$3,$4,$5,NULLIF($6,''),NULLIF($7,''),$8,NULLIF($9,''),NULLIF($10,''),$11,$12,NULLIF($13,''),$14,$15) RETURNING id, created_at, updated_at`,
		u.Name, u.SortCode, u.Kind, u.BaseURL, u.TokenEncrypted, u.RefreshTokenEncrypted, u.PasswordEncrypted, u.TokenExpiresAt, u.LoginIdentifier,
		u.RemoteUserID, jsonValue(u.BalanceSnapshot), jsonValue(u.GroupSnapshot), u.Notes, u.Enabled, u.CreatedBy).Scan(&u.ID, &u.CreatedAt, &u.UpdatedAt)
}

func (r *upstreamRepository) Update(ctx context.Context, u *service.Upstream) error {
	result, err := r.db.ExecContext(ctx, `UPDATE upstreams SET name=$1, sort_code=$2, kind=$3, base_url=$4, token_encrypted=$5,
			refresh_token_encrypted=NULLIF($6,''), password_encrypted=NULLIF($7,''), token_expires_at=$8, login_identifier=NULLIF($9,''), remote_user_id=NULLIF($10,''), notes=NULLIF($11,''), enabled=$12, updated_at=NOW()
			WHERE id=$13 AND deleted_at IS NULL`, u.Name, u.SortCode, u.Kind, u.BaseURL, u.TokenEncrypted, u.RefreshTokenEncrypted, u.PasswordEncrypted, u.TokenExpiresAt, u.LoginIdentifier, u.RemoteUserID, u.Notes, u.Enabled, u.ID)
	if err != nil {
		return fmt.Errorf("update upstream: %w", err)
	}
	if n, _ := result.RowsAffected(); n == 0 {
		return service.ErrUpstreamNotFound
	}
	return nil
}

func (r *upstreamRepository) Delete(ctx context.Context, id int64) error {
	result, err := r.db.ExecContext(ctx, `UPDATE upstreams SET deleted_at=NOW(), updated_at=NOW() WHERE id=$1 AND deleted_at IS NULL`, id)
	if err != nil {
		return fmt.Errorf("delete upstream: %w", err)
	}
	if n, _ := result.RowsAffected(); n == 0 {
		return service.ErrUpstreamNotFound
	}
	_, _ = r.db.ExecContext(ctx, `UPDATE upstream_resources SET deleted_at=NOW(), updated_at=NOW() WHERE upstream_id=$1 AND deleted_at IS NULL`, id)
	return nil
}

func (r *upstreamRepository) ListResources(ctx context.Context, upstreamID int64) ([]*service.UpstreamResource, error) {
	rows, err := r.db.QueryContext(ctx, resourceSelect+" WHERE upstream_id=$1 AND deleted_at IS NULL ORDER BY id DESC", upstreamID)
	if err != nil {
		return nil, fmt.Errorf("list upstream resources: %w", err)
	}
	defer func() { _ = rows.Close() }()
	var result []*service.UpstreamResource
	for rows.Next() {
		item, err := scanResource(rows)
		if err != nil {
			return nil, err
		}
		result = append(result, item)
	}
	return result, rows.Err()
}

func (r *upstreamRepository) GetResource(ctx context.Context, id int64) (*service.UpstreamResource, error) {
	item, err := scanResource(r.db.QueryRowContext(ctx, resourceSelect+" WHERE id=$1 AND deleted_at IS NULL", id))
	if errors.Is(err, sql.ErrNoRows) {
		return nil, service.ErrUpstreamResourceNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("get upstream resource: %w", err)
	}
	return item, nil
}

func (r *upstreamRepository) CreateResource(ctx context.Context, item *service.UpstreamResource) error {
	return r.db.QueryRowContext(ctx, `INSERT INTO upstream_resources (upstream_id, resource_type, remote_id, name, group_name, key_encrypted, models_snapshot, enabled)
		VALUES ($1,$2,$3,$4,NULLIF($5,''),$6,$7,$8)
		ON CONFLICT (upstream_id, resource_type, remote_id) DO UPDATE SET name=EXCLUDED.name, group_name=EXCLUDED.group_name, key_encrypted=EXCLUDED.key_encrypted, deleted_at=NULL, updated_at=NOW()
		RETURNING id, created_at, updated_at`, item.UpstreamID, item.ResourceType, item.RemoteID, item.Name, item.GroupName, item.KeyEncrypted, jsonValue(item.ModelsSnapshot), item.Enabled).Scan(&item.ID, &item.CreatedAt, &item.UpdatedAt)
}

func (r *upstreamRepository) SaveGroups(ctx context.Context, id int64, snapshot domain.UpstreamGroupSnapshot, checkedAt time.Time) error {
	_, err := r.db.ExecContext(ctx, `UPDATE upstreams SET group_snapshot=$1, last_checked_at=$2, last_error=NULL, updated_at=NOW() WHERE id=$3 AND deleted_at IS NULL`, jsonValue(snapshot), checkedAt, id)
	return err
}

func (r *upstreamRepository) SaveBalance(ctx context.Context, id int64, snapshot domain.UpstreamBalanceSnapshot, checkedAt time.Time, probeErr error) error {
	var message any
	if probeErr != nil {
		message = probeErr.Error()
	}
	_, err := r.db.ExecContext(ctx, `UPDATE upstreams SET balance_snapshot=$1, last_checked_at=$2, last_error=$3, updated_at=NOW() WHERE id=$4 AND deleted_at IS NULL`, jsonValue(snapshot), checkedAt, message, id)
	return err
}

func (r *upstreamRepository) SaveModels(ctx context.Context, id int64, models []string, fetchedAt time.Time) error {
	_, err := r.db.ExecContext(ctx, `UPDATE upstream_resources SET models_snapshot=$1, models_fetched_at=$2, updated_at=NOW() WHERE id=$3 AND deleted_at IS NULL`, jsonValue(models), fetchedAt, id)
	return err
}

func (r *upstreamRepository) MarkResourceSynced(ctx context.Context, resourceID, accountID int64, rate float64, syncedAt time.Time) error {
	_, err := r.db.ExecContext(ctx, `UPDATE upstream_resources SET synced_account_id=$1, synced_rate_multiplier=$2, synced_at=$3, updated_at=NOW() WHERE id=$4 AND deleted_at IS NULL`, accountID, rate, syncedAt, resourceID)
	return err
}

func (r *upstreamRepository) ListPaginated(ctx context.Context, page, pageSize int, search string) ([]*service.Upstream, int64, error) {
	whereClause := "WHERE deleted_at IS NULL"
	args := []any{}
	argIdx := 1

	if search != "" {
		whereClause += fmt.Sprintf(" AND (name ILIKE $%d OR base_url ILIKE $%d)", argIdx, argIdx)
		args = append(args, "%"+search+"%")
		argIdx++
	}

	var total int64
	countQuery := "SELECT COUNT(*) FROM upstreams " + whereClause
	if err := r.db.QueryRowContext(ctx, countQuery, args...).Scan(&total); err != nil {
		return nil, 0, fmt.Errorf("upstream.listPaginated count: %w", err)
	}

	offset := (page - 1) * pageSize
	selectQuery := fmt.Sprintf("%s %s ORDER BY sort_code, id LIMIT $%d OFFSET $%d",
		upstreamSelect, whereClause, argIdx, argIdx+1)
	selectArgs := append(args, pageSize, offset)

	rows, err := r.db.QueryContext(ctx, selectQuery, selectArgs...)
	if err != nil {
		return nil, 0, fmt.Errorf("upstream.listPaginated: %w", err)
	}
	defer func() { _ = rows.Close() }()

	var out []*service.Upstream
	for rows.Next() {
		item, err := scanUpstream(rows)
		if err != nil {
			return nil, 0, err
		}
		out = append(out, item)
	}
	return out, total, rows.Err()
}

func (r *upstreamRepository) CountResources(ctx context.Context) (map[int64]int, error) {
	rows, err := r.db.QueryContext(ctx,
		`SELECT upstream_id, COUNT(*) FROM upstream_resources WHERE deleted_at IS NULL GROUP BY upstream_id`)
	if err != nil {
		return nil, fmt.Errorf("upstream.countResources: %w", err)
	}
	defer func() { _ = rows.Close() }()

	result := make(map[int64]int)
	for rows.Next() {
		var uid int64
		var cnt int
		if err := rows.Scan(&uid, &cnt); err != nil {
			return nil, err
		}
		result[uid] = cnt
	}
	return result, rows.Err()
}

const upstreamSelect = `SELECT id,name,sort_code,kind,base_url,token_encrypted,COALESCE(refresh_token_encrypted,''),COALESCE(password_encrypted,''),token_expires_at,COALESCE(login_identifier,''),COALESCE(remote_user_id,''),balance_snapshot,group_snapshot,COALESCE(notes,''),enabled,last_checked_at,COALESCE(last_error,''),created_by,created_at,updated_at FROM upstreams`
const resourceSelect = `SELECT id,upstream_id,resource_type,remote_id,COALESCE(name,''),COALESCE(group_name,''),key_encrypted,models_snapshot,models_fetched_at,synced_account_id,synced_rate_multiplier,synced_at,enabled,created_at,updated_at FROM upstream_resources`

type scanner interface{ Scan(...any) error }

func scanUpstream(s scanner) (*service.Upstream, error) {
	var item service.Upstream
	var balance, groups []byte
	var refresh, password, login, remoteUserID, notes, lastError string
	if err := s.Scan(&item.ID, &item.Name, &item.SortCode, &item.Kind, &item.BaseURL, &item.TokenEncrypted, &refresh, &password, &item.TokenExpiresAt, &login, &remoteUserID, &balance, &groups, &notes, &item.Enabled, &item.LastCheckedAt, &lastError, &item.CreatedBy, &item.CreatedAt, &item.UpdatedAt); err != nil {
		return nil, err
	}
	item.RefreshTokenEncrypted, item.PasswordEncrypted, item.LoginIdentifier, item.RemoteUserID, item.Notes, item.LastError = refresh, password, login, remoteUserID, notes, lastError
	if err := json.Unmarshal(balance, &item.BalanceSnapshot); err != nil {
		return nil, fmt.Errorf("decode upstream balance snapshot: %w", err)
	}
	if err := json.Unmarshal(groups, &item.GroupSnapshot); err != nil {
		return nil, fmt.Errorf("decode upstream group snapshot: %w", err)
	}
	return &item, nil
}
func scanResource(s scanner) (*service.UpstreamResource, error) {
	var item service.UpstreamResource
	var group string
	var models []byte
	if err := s.Scan(&item.ID, &item.UpstreamID, &item.ResourceType, &item.RemoteID, &item.Name, &group, &item.KeyEncrypted, &models, &item.ModelsFetchedAt, &item.SyncedAccountID, &item.SyncedRateMultiplier, &item.SyncedAt, &item.Enabled, &item.CreatedAt, &item.UpdatedAt); err != nil {
		return nil, err
	}
	item.GroupName = group
	if err := json.Unmarshal(models, &item.ModelsSnapshot); err != nil {
		return nil, fmt.Errorf("decode upstream models snapshot: %w", err)
	}
	return &item, nil
}
func jsonValue(value any) []byte { data, _ := json.Marshal(value); return data }
