package service

import (
	"context"
	"net/http"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/domain"
)

type Upstream struct {
	ID                    int64                          `json:"id"`
	Name                  string                         `json:"name"`
	SortCode              int                            `json:"sort_code"`
	Kind                  string                         `json:"kind"`
	BaseURL               string                         `json:"base_url"`
	TokenEncrypted        string                         `json:"-"`
	RefreshTokenEncrypted string                         `json:"-"`
	PasswordEncrypted     string                         `json:"-"`
	TokenExpiresAt        *time.Time                     `json:"token_expires_at,omitempty"`
	LoginIdentifier       string                         `json:"login_identifier,omitempty"`
	RemoteUserID          string                         `json:"remote_user_id,omitempty"`
	BalanceSnapshot       domain.UpstreamBalanceSnapshot `json:"balance_snapshot"`
	GroupSnapshot         domain.UpstreamGroupSnapshot   `json:"group_snapshot"`
	Notes                 string                         `json:"notes,omitempty"`
	Enabled               bool                           `json:"enabled"`
	LastCheckedAt         *time.Time                     `json:"last_checked_at,omitempty"`
	LastError             string                         `json:"last_error,omitempty"`
	CreatedBy             int64                          `json:"created_by"`
	CreatedAt             time.Time                      `json:"created_at"`
	UpdatedAt             time.Time                      `json:"updated_at"`
}

type UpstreamResource struct {
	ID                   int64      `json:"id"`
	UpstreamID           int64      `json:"upstream_id"`
	ResourceType         string     `json:"resource_type"`
	RemoteID             string     `json:"remote_id"`
	Name                 string     `json:"name"`
	GroupName            string     `json:"group_name,omitempty"`
	KeyEncrypted         string     `json:"-"`
	ModelsSnapshot       []string   `json:"models_snapshot"`
	ModelsFetchedAt      *time.Time `json:"models_fetched_at,omitempty"`
	SyncedAccountID      *int64     `json:"synced_account_id,omitempty"`
	SyncedRateMultiplier *float64   `json:"synced_rate_multiplier,omitempty"`
	SyncedAt             *time.Time `json:"synced_at,omitempty"`
	Enabled              bool       `json:"enabled"`
	CreatedAt            time.Time  `json:"created_at"`
	UpdatedAt            time.Time  `json:"updated_at"`
}

type UpstreamRepository interface {
	List(context.Context) ([]*Upstream, error)
	ListPaginated(ctx context.Context, page, pageSize int, search string) ([]*Upstream, int64, error)
	CountResources(ctx context.Context) (map[int64]int, error)
	GetByID(context.Context, int64) (*Upstream, error)
	Create(context.Context, *Upstream) error
	Update(context.Context, *Upstream) error
	Delete(context.Context, int64) error
	ListResources(context.Context, int64) ([]*UpstreamResource, error)
	GetResource(context.Context, int64) (*UpstreamResource, error)
	CreateResource(context.Context, *UpstreamResource) error
	SaveGroups(context.Context, int64, domain.UpstreamGroupSnapshot, time.Time) error
	SaveBalance(context.Context, int64, domain.UpstreamBalanceSnapshot, time.Time, error) error
	SaveModels(context.Context, int64, []string, time.Time) error
	UpdateResourceKey(context.Context, int64, string) error
	MarkResourceSynced(context.Context, int64, int64, float64, time.Time) error
}

type UpstreamService interface {
	List(context.Context) ([]*UpstreamView, error)
	ListPaginated(ctx context.Context, page, pageSize int, search string) ([]*UpstreamListView, int64, error)
	GetByID(context.Context, int64) (*UpstreamView, error)
	Create(context.Context, *CreateUpstreamInput) (*UpstreamView, error)
	Update(context.Context, int64, *UpdateUpstreamInput) (*UpstreamView, error)
	Delete(context.Context, int64) error
	TestConnection(context.Context, int64) (*UpstreamView, error)
	RefreshBalance(context.Context, int64) (*UpstreamView, error)
	FetchGroups(context.Context, int64, bool) ([]domain.UpstreamGroupItem, error)
	ListResources(context.Context, int64) ([]*UpstreamResourceView, error)
	CreateKey(context.Context, int64, string, string) (*UpstreamResourceView, error)
	FetchModels(context.Context, int64, bool) ([]string, error)
	ChatCompletion(context.Context, int64, map[string]any) (*http.Response, error)
	SyncToAccount(context.Context, int64, []int64) (*UpstreamSyncResult, error)
	GetBalanceNotifySettings(context.Context) (*UpstreamBalanceNotifySettings, error)
	UpdateBalanceNotifySettings(context.Context, *UpstreamBalanceNotifySettings) error
	RefreshAllBalances(context.Context) ([]*UpstreamView, error)
}

// UpstreamBalanceNotifySettings controls the optional periodic upstream balance probe.
type UpstreamBalanceNotifySettings struct {
	Enabled   bool     `json:"enabled"`
	Threshold float64  `json:"threshold"`
	Emails    []string `json:"emails"`
}

type UpstreamView struct {
	Upstream
	TokenMasked string `json:"token_masked,omitempty"`
}

type UpstreamListView struct {
	UpstreamView
	ResourceCount int `json:"resource_count"`
}
type UpstreamResourceView struct {
	UpstreamResource
	KeyMasked string `json:"key_masked"`
}

type CreateUpstreamInput struct {
	Name            string `json:"name" binding:"required"`
	SortCode        int    `json:"sort_code"`
	Kind            string `json:"kind" binding:"required"`
	BaseURL         string `json:"base_url" binding:"required"`
	Token           string `json:"token"`
	RefreshToken    string `json:"refresh_token"`
	LoginIdentifier string `json:"login_identifier"`
	RemoteUserID    string `json:"remote_user_id"`
	Password        string `json:"password"`
	Notes           string `json:"notes"`
	Enabled         *bool  `json:"enabled"`
}

type UpdateUpstreamInput struct {
	Name            string `json:"name" binding:"required"`
	SortCode        int    `json:"sort_code"`
	Kind            string `json:"kind" binding:"required"`
	BaseURL         string `json:"base_url" binding:"required"`
	Token           string `json:"token"`
	RefreshToken    string `json:"refresh_token"`
	LoginIdentifier string `json:"login_identifier"`
	RemoteUserID    string `json:"remote_user_id"`
	Password        string `json:"password"`
	Notes           string `json:"notes"`
	Enabled         *bool  `json:"enabled"`
}

type UpstreamSyncItem struct {
	ResourceID int64  `json:"resource_id"`
	Status     string `json:"status"`
	AccountID  int64  `json:"account_id,omitempty"`
	Message    string `json:"message,omitempty"`
}
type UpstreamSyncResult struct {
	Created int                `json:"created"`
	Updated int                `json:"updated"`
	Skipped int                `json:"skipped"`
	Failed  int                `json:"failed"`
	Items   []UpstreamSyncItem `json:"items"`
}
