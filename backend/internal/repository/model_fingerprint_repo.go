package repository

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"

	"github.com/Wei-Shaw/sub2api/internal/service"
)

type modelFingerprintRepository struct{ db *sql.DB }

func NewModelFingerprintRepository(db *sql.DB) service.ModelFingerprintRepository {
	return &modelFingerprintRepository{db: db}
}
func (r *modelFingerprintRepository) Save(ctx context.Context, record *service.ModelFingerprintRecord) error {
	samples, err := json.Marshal(record.Samples)
	if err != nil {
		return err
	}
	analysis, err := json.Marshal(record.Analysis)
	if err != nil {
		return err
	}
	if record.ID == 0 {
		return r.db.QueryRowContext(ctx, `INSERT INTO model_fingerprint_records (account_id,account_name,platform,model,status,score,samples,upstream_model,endpoint,analysis) VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10) RETURNING id,created_at`, record.AccountID, record.AccountName, record.Platform, record.Model, record.Status, record.Score, string(samples), record.UpstreamModel, record.Endpoint, string(analysis)).Scan(&record.ID, &record.CreatedAt)
	}
	_, err = r.db.ExecContext(ctx, `UPDATE model_fingerprint_records SET status=$2,score=$3,samples=$4,analysis=$5 WHERE id=$1`, record.ID, record.Status, record.Score, string(samples), string(analysis))
	return err
}
func (r *modelFingerprintRepository) List(ctx context.Context, f service.FingerprintFilter) ([]service.ModelFingerprintRecord, int64, error) {
	var total int64
	where := ` FROM model_fingerprint_records WHERE ($1::bigint=0 OR account_id=$1) AND ($2::text='' OR model=$2)`
	if err := r.db.QueryRowContext(ctx, `SELECT COUNT(*)`+where, f.AccountID, f.Model).Scan(&total); err != nil {
		return nil, 0, err
	}
	rows, err := r.db.QueryContext(ctx, `SELECT id,account_id,account_name,platform,model,status,score,samples,created_at,upstream_model,endpoint,analysis`+where+` ORDER BY created_at DESC,id DESC LIMIT $3 OFFSET $4`, f.AccountID, f.Model, f.PageSize, (f.Page-1)*f.PageSize)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()
	items := make([]service.ModelFingerprintRecord, 0)
	for rows.Next() {
		var item service.ModelFingerprintRecord
		var samples, analysis []byte
		if err := rows.Scan(&item.ID, &item.AccountID, &item.AccountName, &item.Platform, &item.Model, &item.Status, &item.Score, &samples, &item.CreatedAt, &item.UpstreamModel, &item.Endpoint, &analysis); err != nil {
			return nil, 0, err
		}
		if err := json.Unmarshal(samples, &item.Samples); err != nil {
			return nil, 0, err
		}
		if len(analysis) > 0 {
			if err := json.Unmarshal(analysis, &item.Analysis); err != nil {
				return nil, 0, err
			}
		}
		items = append(items, item)
	}
	return items, total, rows.Err()
}
func (r *modelFingerprintRepository) LoadReference(ctx context.Context) ([]byte, []byte, error) {
	var bank, detector []byte
	err := r.db.QueryRowContext(ctx, `SELECT bank,detector FROM model_fingerprint_reference WHERE id=1`).Scan(&bank, &detector)
	if errors.Is(err, sql.ErrNoRows) {
		err = nil
	}
	return bank, detector, err
}
func (r *modelFingerprintRepository) SaveReference(ctx context.Context, bank, detector []byte) error {
	_, err := r.db.ExecContext(ctx, `INSERT INTO model_fingerprint_reference(id,bank,detector) VALUES(1,$1,$2) ON CONFLICT(id) DO UPDATE SET bank=EXCLUDED.bank,detector=EXCLUDED.detector,updated_at=NOW()`, string(bank), detector)
	return err
}
