package service

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/config"
	"github.com/Wei-Shaw/sub2api/internal/domain"
	"github.com/Wei-Shaw/sub2api/internal/util/urlvalidator"
)

var (
	ErrUpstreamNotFound         = errors.New("upstream not found")
	ErrUpstreamResourceNotFound = errors.New("upstream resource not found")
)

type adminUpstreamService struct {
	repo      UpstreamRepository
	admin     AdminService
	encryptor SecretEncryptor
	cfg       *config.Config
}

func NewAdminUpstreamService(repo UpstreamRepository, admin AdminService, encryptor SecretEncryptor, cfg *config.Config) UpstreamService {
	return &adminUpstreamService{repo: repo, admin: admin, encryptor: encryptor, cfg: cfg}
}

func (s *adminUpstreamService) List(ctx context.Context) ([]*UpstreamView, error) {
	items, err := s.repo.List(ctx)
	if err != nil {
		return nil, err
	}
	result := make([]*UpstreamView, 0, len(items))
	for _, item := range items {
		view, err := s.view(item)
		if err != nil {
			return nil, err
		}
		result = append(result, view)
	}
	return result, nil
}

func (s *adminUpstreamService) GetByID(ctx context.Context, id int64) (*UpstreamView, error) {
	item, err := s.repo.GetByID(ctx, id)
	if err != nil {
		return nil, err
	}
	return s.view(item)
}

func (s *adminUpstreamService) Create(ctx context.Context, input *CreateUpstreamInput) (*UpstreamView, error) {
	u, err := s.buildUpstream(ctx, input.Name, input.SortCode, input.Kind, input.BaseURL, input.Token, input.LoginIdentifier, input.RemoteUserID, input.Password, input.Notes, input.Enabled, 0)
	if err != nil {
		return nil, err
	}
	if err := s.repo.Create(ctx, u); err != nil {
		return nil, err
	}
	// 创建成功后探测不阻断保存，失败信息会留在 last_error。
	if _, err := s.testConnection(ctx, u); err != nil {
		_ = s.repo.SaveBalance(ctx, u.ID, u.BalanceSnapshot, time.Now().UTC(), err)
	}
	return s.GetByID(ctx, u.ID)
}

func (s *adminUpstreamService) Update(ctx context.Context, id int64, input *UpdateUpstreamInput) (*UpstreamView, error) {
	existing, err := s.repo.GetByID(ctx, id)
	if err != nil {
		return nil, err
	}

	// An empty credential field means "keep the existing credential" on edit.
	// This is required for password-based Sub2API entries because the list API
	// never sends either the password or the access token back to the browser.
	if strings.TrimSpace(input.Token) == "" && strings.TrimSpace(input.Password) == "" {
		if strings.TrimSpace(input.Name) == "" {
			return nil, fmt.Errorf("upstream name is required")
		}
		if input.Kind != existing.Kind {
			return nil, fmt.Errorf("credentials are required when changing upstream kind")
		}
		normalized, validateErr := s.validateURL(input.BaseURL)
		if validateErr != nil {
			return nil, validateErr
		}
		if normalized != existing.BaseURL {
			return nil, fmt.Errorf("credentials are required when changing upstream URL")
		}
		identifier := strings.TrimSpace(input.LoginIdentifier)
		if identifier != "" && identifier != existing.LoginIdentifier {
			return nil, fmt.Errorf("password is required when changing upstream login identifier")
		}
		remoteUserID := strings.TrimSpace(input.RemoteUserID)
		u := *existing
		u.Name = strings.TrimSpace(input.Name)
		u.SortCode = input.SortCode
		u.LoginIdentifier = identifier
		if u.LoginIdentifier == "" {
			u.LoginIdentifier = existing.LoginIdentifier
		}
		if remoteUserID != "" {
			u.RemoteUserID = remoteUserID
		}
		u.Notes = input.Notes
		if input.Enabled != nil {
			u.Enabled = *input.Enabled
		}
		if err := s.repo.Update(ctx, &u); err != nil {
			return nil, err
		}
		if _, err := s.testConnection(ctx, &u); err != nil {
			_ = s.repo.SaveBalance(ctx, u.ID, u.BalanceSnapshot, time.Now().UTC(), err)
		}
		return s.GetByID(ctx, id)
	}
	u, err := s.buildUpstream(ctx, input.Name, input.SortCode, input.Kind, input.BaseURL, input.Token, input.LoginIdentifier, input.RemoteUserID, input.Password, input.Notes, input.Enabled, existing.CreatedBy)
	if err != nil {
		return nil, err
	}
	u.ID = id
	u.CreatedAt, u.UpdatedAt = existing.CreatedAt, existing.UpdatedAt
	if input.Password == "" && input.Kind == existing.Kind {
		u.PasswordEncrypted = existing.PasswordEncrypted
	}
	if u.RemoteUserID == "" && input.Kind == existing.Kind {
		u.RemoteUserID = existing.RemoteUserID
	}
	if err := s.repo.Update(ctx, u); err != nil {
		return nil, err
	}
	if _, err := s.testConnection(ctx, u); err != nil {
		_ = s.repo.SaveBalance(ctx, u.ID, u.BalanceSnapshot, time.Now().UTC(), err)
	}
	return s.GetByID(ctx, id)
}

func (s *adminUpstreamService) Delete(ctx context.Context, id int64) error {
	return s.repo.Delete(ctx, id)
}

func (s *adminUpstreamService) TestConnection(ctx context.Context, id int64) (*UpstreamView, error) {
	u, err := s.repo.GetByID(ctx, id)
	if err != nil {
		return nil, err
	}
	if _, err := s.testConnection(ctx, u); err != nil {
		return nil, err
	}
	return s.GetByID(ctx, id)
}

func (s *adminUpstreamService) RefreshBalance(ctx context.Context, id int64) (*UpstreamView, error) {
	u, err := s.repo.GetByID(ctx, id)
	if err != nil {
		return nil, err
	}
	var usage *domain.UpstreamBalanceSnapshot
	err = s.withUpstreamAccess(ctx, u, func(access *UpstreamAccess, client UpstreamClient) error {
		var usageErr error
		usage, usageErr = client.FetchAccountUsage(ctx, access)
		return usageErr
	})
	now := time.Now().UTC()
	if err != nil {
		_ = s.repo.SaveBalance(ctx, id, u.BalanceSnapshot, now, err)
		return nil, err
	}
	if err := s.repo.SaveBalance(ctx, id, *usage, now, nil); err != nil {
		return nil, err
	}
	return s.GetByID(ctx, id)
}

func (s *adminUpstreamService) FetchGroups(ctx context.Context, id int64, refresh bool) ([]domain.UpstreamGroupItem, error) {
	u, err := s.repo.GetByID(ctx, id)
	if err != nil {
		return nil, err
	}
	if !refresh && len(u.GroupSnapshot.Items) > 0 {
		return u.GroupSnapshot.Items, nil
	}
	var items []domain.UpstreamGroupItem
	err = s.withUpstreamAccess(ctx, u, func(access *UpstreamAccess, client UpstreamClient) error {
		var groupsErr error
		items, groupsErr = client.FetchGroups(ctx, access)
		return groupsErr
	})
	if err != nil {
		return nil, err
	}
	now := time.Now().UTC()
	if err := s.repo.SaveGroups(ctx, id, domain.UpstreamGroupSnapshot{Items: items, FetchedAt: now.Format(time.RFC3339)}, now); err != nil {
		return nil, err
	}
	return items, nil
}

func (s *adminUpstreamService) ListResources(ctx context.Context, upstreamID int64) ([]*UpstreamResourceView, error) {
	items, err := s.repo.ListResources(ctx, upstreamID)
	if err != nil {
		return nil, err
	}
	result := make([]*UpstreamResourceView, 0, len(items))
	for _, item := range items {
		secret, decryptErr := s.encryptor.Decrypt(item.KeyEncrypted)
		if decryptErr != nil {
			return nil, decryptErr
		}
		result = append(result, &UpstreamResourceView{UpstreamResource: *item, KeyMasked: maskSecret(secret)})
	}
	return result, nil
}

func (s *adminUpstreamService) CreateKey(ctx context.Context, upstreamID int64, groupName, keyName string) (*UpstreamResourceView, error) {
	u, err := s.repo.GetByID(ctx, upstreamID)
	if err != nil {
		return nil, err
	}
	groups, err := s.FetchGroups(ctx, upstreamID, false)
	if err != nil {
		return nil, err
	}
	groupArg := strings.TrimSpace(groupName)
	var group *domain.UpstreamGroupItem
	for i := range groups {
		if groups[i].Name == groupName {
			group = &groups[i]
			break
		}
	}
	if group == nil {
		return nil, fmt.Errorf("upstream group not found: %s", groupName)
	}
	if u.Kind == domain.UpstreamKindSub2API {
		groupArg = group.ID
	}
	var created *UpstreamCreatedKey
	err = s.withUpstreamAccess(ctx, u, func(access *UpstreamAccess, upstreamClient UpstreamClient) error {
		var createErr error
		created, createErr = upstreamClient.CreateKey(ctx, access, groupArg, keyName)
		return createErr
	})
	if err != nil {
		return nil, err
	}
	encrypted, err := s.encryptor.Encrypt(created.Secret)
	if err != nil {
		return nil, err
	}
	resourceType := domain.UpstreamResourceTypeToken
	if u.Kind == domain.UpstreamKindSub2API {
		resourceType = domain.UpstreamResourceTypeKey
	}
	resource := &UpstreamResource{UpstreamID: upstreamID, ResourceType: resourceType, RemoteID: created.RemoteID, Name: keyName, GroupName: groupName, KeyEncrypted: encrypted, ModelsSnapshot: []string{}, Enabled: true}
	if err := s.repo.CreateResource(ctx, resource); err != nil {
		return nil, err
	}
	var models []string
	modelErr := s.withUpstreamAccess(ctx, u, func(access *UpstreamAccess, upstreamClient UpstreamClient) error {
		var fetchErr error
		models, fetchErr = upstreamClient.FetchModels(ctx, access, created.Secret)
		return fetchErr
	})
	if modelErr == nil {
		resource.ModelsSnapshot = models
		_ = s.repo.SaveModels(ctx, resource.ID, models, time.Now().UTC())
	}
	view := &UpstreamResourceView{UpstreamResource: *resource, KeyMasked: maskSecret(created.Secret)}
	if modelErr != nil {
		return view, fmt.Errorf("key created but model fetch failed: %w", modelErr)
	}
	return view, nil
}

func (s *adminUpstreamService) FetchModels(ctx context.Context, resourceID int64, refresh bool) ([]string, error) {
	resource, err := s.repo.GetResource(ctx, resourceID)
	if err != nil {
		return nil, err
	}
	if !refresh && len(resource.ModelsSnapshot) > 0 {
		return resource.ModelsSnapshot, nil
	}
	u, err := s.repo.GetByID(ctx, resource.UpstreamID)
	if err != nil {
		return nil, err
	}
	secret, err := s.encryptor.Decrypt(resource.KeyEncrypted)
	if err != nil {
		return nil, err
	}
	var models []string
	err = s.withUpstreamAccess(ctx, u, func(access *UpstreamAccess, client UpstreamClient) error {
		var fetchErr error
		models, fetchErr = client.FetchModels(ctx, access, secret)
		return fetchErr
	})
	if err != nil {
		return nil, err
	}
	if err := s.repo.SaveModels(ctx, resourceID, models, time.Now().UTC()); err != nil {
		return nil, err
	}
	return models, nil
}

func (s *adminUpstreamService) ChatCompletion(ctx context.Context, resourceID int64, body map[string]any) (*http.Response, error) {
	resource, err := s.repo.GetResource(ctx, resourceID)
	if err != nil {
		return nil, err
	}
	u, err := s.repo.GetByID(ctx, resource.UpstreamID)
	if err != nil {
		return nil, err
	}
	secret, err := s.encryptor.Decrypt(resource.KeyEncrypted)
	if err != nil {
		return nil, err
	}
	var response *http.Response
	err = s.withUpstreamAccess(ctx, u, func(access *UpstreamAccess, client UpstreamClient) error {
		var requestErr error
		response, requestErr = client.ChatCompletion(ctx, access, secret, body)
		return requestErr
	})
	return response, err
}

func (s *adminUpstreamService) SyncToAccount(ctx context.Context, resourceID int64, groupIDs []int64) (*UpstreamSyncResult, error) {
	if len(groupIDs) == 0 {
		return nil, fmt.Errorf("at least one local group is required")
	}
	if err := s.admin.ValidateAccountGroupBindings(ctx, groupIDs); err != nil {
		return nil, err
	}
	resource, err := s.repo.GetResource(ctx, resourceID)
	if err != nil {
		return nil, err
	}
	u, err := s.repo.GetByID(ctx, resource.UpstreamID)
	if err != nil {
		return nil, err
	}
	result := &UpstreamSyncResult{Items: []UpstreamSyncItem{}}
	if resource.SyncedAccountID != nil {
		account, getErr := s.admin.GetAccount(ctx, *resource.SyncedAccountID)
		if getErr != nil {
			result.Failed++
			result.Items = append(result.Items, UpstreamSyncItem{ResourceID: resourceID, Status: "failed", Message: getErr.Error()})
			return result, nil
		}
		// Once linked, subsequent syncs are intentionally limited to the model
		// whitelist. Scheduling, groups, rate and other account settings remain
		// under local account management instead of being overwritten here.
		desired := modelWhitelistMapping(resource.ModelsSnapshot)
		current := accountModelMapping(account)
		if len(desired) == 0 || modelMappingsEqual(current, desired) {
			result.Skipped++
			result.Items = append(result.Items, UpstreamSyncItem{ResourceID: resourceID, Status: "skipped", AccountID: account.ID, Message: "model whitelist unchanged"})
			return result, nil
		}
		updatedCredentials := cloneStringAnyMap(account.Credentials)
		updatedCredentials["model_mapping"] = desired
		updated, updateErr := s.admin.UpdateAccount(ctx, account.ID, &UpdateAccountInput{Credentials: updatedCredentials})
		if updateErr != nil {
			result.Failed++
			result.Items = append(result.Items, UpstreamSyncItem{ResourceID: resourceID, Status: "failed", AccountID: account.ID, Message: updateErr.Error()})
			return result, nil
		}
		_ = updated
		syncedRate := 1.0
		if resource.SyncedRateMultiplier != nil {
			syncedRate = *resource.SyncedRateMultiplier
		}
		_ = s.repo.MarkResourceSynced(ctx, resourceID, account.ID, syncedRate, time.Now().UTC())
		result.Updated++
		result.Items = append(result.Items, UpstreamSyncItem{ResourceID: resourceID, Status: "updated", AccountID: account.ID})
		return result, nil
	}
	secret, err := s.encryptor.Decrypt(resource.KeyEncrypted)
	if err != nil {
		return nil, err
	}
	rate, rateText, err := s.resolveRate(ctx, u, resource.GroupName)
	if err != nil {
		return nil, err
	}
	name := u.Name + "-" + rateText
	extra := map[string]any{"upstream_sync": map[string]any{"upstream_id": u.ID, "resource_id": resource.ID, "remote_id": resource.RemoteID, "group_name": resource.GroupName}}
	credentials := map[string]any{"api_key": secret, "base_url": u.BaseURL, "pool_mode": true}
	if modelMapping := modelWhitelistMapping(resource.ModelsSnapshot); len(modelMapping) > 0 {
		credentials["model_mapping"] = modelMapping
	}
	account, createErr := s.admin.CreateAccount(ctx, &CreateAccountInput{Name: name, Platform: domain.PlatformOpenAI, Type: AccountTypeAPIKey, Credentials: credentials, Extra: extra, Concurrency: 5, Priority: 1, RateMultiplier: &rate, GroupIDs: groupIDs, SkipDefaultGroupBind: true})
	if createErr != nil {
		result.Failed++
		result.Items = append(result.Items, UpstreamSyncItem{ResourceID: resourceID, Status: "failed", Message: createErr.Error()})
		return result, nil
	}
	if err := s.repo.MarkResourceSynced(ctx, resourceID, account.ID, rate, time.Now().UTC()); err != nil {
		return nil, err
	}
	result.Created++
	result.Items = append(result.Items, UpstreamSyncItem{ResourceID: resourceID, Status: "created", AccountID: account.ID})
	return result, nil
}

func (s *adminUpstreamService) buildUpstream(ctx context.Context, name string, sortCode int, kind, baseURL, token, identifier, remoteUserID, password, notes string, enabled *bool, createdBy int64) (*Upstream, error) {
	if strings.TrimSpace(name) == "" {
		return nil, fmt.Errorf("upstream name is required")
	}
	if kind != domain.UpstreamKindNewAPI && kind != domain.UpstreamKindSub2API {
		return nil, fmt.Errorf("unsupported upstream kind: %s", kind)
	}
	normalized, err := s.validateURL(baseURL)
	if err != nil {
		return nil, err
	}
	u := &Upstream{Name: strings.TrimSpace(name), SortCode: sortCode, Kind: kind, BaseURL: normalized, LoginIdentifier: strings.TrimSpace(identifier), RemoteUserID: strings.TrimSpace(remoteUserID), Notes: notes, Enabled: enabled == nil || *enabled, CreatedBy: createdBy, BalanceSnapshot: domain.UpstreamBalanceSnapshot{}, GroupSnapshot: domain.UpstreamGroupSnapshot{Items: []domain.UpstreamGroupItem{}}}
	if kind == domain.UpstreamKindSub2API {
		if password == "" && token == "" {
			return nil, fmt.Errorf("sub2api requires login credentials")
		}
		if password != "" {
			u.PasswordEncrypted, err = s.encryptor.Encrypt(password)
			if err != nil {
				return nil, err
			}
			login, loginErr := loginSub2API(ctx, normalized, identifier, password, s.cfg)
			if loginErr != nil {
				return nil, loginErr
			}
			u.TokenEncrypted, err = s.encryptor.Encrypt(login.AccessToken)
			if err != nil {
				return nil, err
			}
			u.RefreshTokenEncrypted, err = s.encryptor.Encrypt(login.RefreshToken)
			if err != nil {
				return nil, err
			}
			expires := time.Now().UTC().Add(time.Duration(login.ExpiresIn) * time.Second)
			u.TokenExpiresAt = &expires
		} else {
			u.TokenEncrypted, err = s.encryptor.Encrypt(token)
		}
	} else {
		if password != "" {
			if u.LoginIdentifier == "" {
				return nil, fmt.Errorf("new-api login identifier is required")
			}
			u.PasswordEncrypted, err = s.encryptor.Encrypt(password)
			if err != nil {
				return nil, err
			}
			login, loginErr := loginNewAPI(ctx, normalized, u.LoginIdentifier, password, s.cfg)
			if loginErr != nil {
				return nil, loginErr
			}
			u.RemoteUserID = login.UserID
			token = login.AccessToken
		}
		if token == "" {
			return nil, fmt.Errorf("new-api requires login credentials or an access token")
		}
		if u.RemoteUserID == "" {
			return nil, fmt.Errorf("new-api user ID is required with an access token")
		}
		u.TokenEncrypted, err = s.encryptor.Encrypt(token)
	}
	if err != nil {
		return nil, err
	}
	return u, nil
}

func (s *adminUpstreamService) testConnection(ctx context.Context, u *Upstream) (bool, error) {
	var groups []domain.UpstreamGroupItem
	var usage *domain.UpstreamBalanceSnapshot
	err := s.withUpstreamAccess(ctx, u, func(access *UpstreamAccess, client UpstreamClient) error {
		var groupErr error
		groups, groupErr = client.FetchGroups(ctx, access)
		if groupErr != nil {
			return groupErr
		}
		var usageErr error
		usage, usageErr = client.FetchAccountUsage(ctx, access)
		return usageErr
	})
	now := time.Now().UTC()
	if err == nil {
		_ = s.repo.SaveGroups(ctx, u.ID, domain.UpstreamGroupSnapshot{Items: groups, FetchedAt: now.Format(time.RFC3339)}, now)
		u.BalanceSnapshot = *usage
	}
	if err != nil {
		return false, fmt.Errorf("connection test failed: %w", err)
	}
	_ = s.repo.SaveBalance(ctx, u.ID, *usage, now, nil)
	return true, nil
}

func (s *adminUpstreamService) accessAndClient(ctx context.Context, u *Upstream) (*UpstreamAccess, UpstreamClient, error) {
	if err := s.validateURLOnly(u.BaseURL); err != nil {
		return nil, nil, err
	}
	var token string
	if u.TokenEncrypted != "" {
		var decryptErr error
		token, decryptErr = s.encryptor.Decrypt(u.TokenEncrypted)
		if decryptErr != nil && u.PasswordEncrypted == "" {
			return nil, nil, fmt.Errorf("decrypt upstream credential: %w", decryptErr)
		}
	}
	if u.Kind == domain.UpstreamKindNewAPI && (token == "" || (u.TokenExpiresAt != nil && time.Until(*u.TokenExpiresAt) < 5*time.Minute)) {
		login, loginErr := s.loginWithStoredPassword(ctx, u)
		if loginErr != nil {
			if token == "" {
				return nil, nil, fmt.Errorf("需要重新登录: %w", loginErr)
			}
		} else {
			if err := s.applyNewAPILogin(ctx, u, login); err != nil {
				return nil, nil, err
			}
			token = login.AccessToken
		}
	}
	if u.Kind == domain.UpstreamKindSub2API && (token == "" || (u.TokenExpiresAt != nil && time.Until(*u.TokenExpiresAt) < 5*time.Minute)) {
		var login *UpstreamLogin
		var loginErr error
		if u.RefreshTokenEncrypted != "" {
			refresh, decryptErr := s.encryptor.Decrypt(u.RefreshTokenEncrypted)
			if decryptErr == nil {
				login, loginErr = refreshSub2API(ctx, u.BaseURL, refresh, s.cfg)
			} else {
				loginErr = decryptErr
			}
		}
		if loginErr != nil || login == nil || login.AccessToken == "" {
			login, loginErr = s.loginWithStoredPassword(ctx, u)
		}
		if loginErr != nil {
			_ = s.repo.SaveBalance(ctx, u.ID, u.BalanceSnapshot, time.Now().UTC(), fmt.Errorf("需要重新登录: %w", loginErr))
			return nil, nil, fmt.Errorf("需要重新登录: %w", loginErr)
		}
		if err := s.applySub2APILogin(ctx, u, login); err != nil {
			return nil, nil, err
		}
		token = login.AccessToken
	}
	if token == "" {
		return nil, nil, fmt.Errorf("upstream credential is empty")
	}
	httpClient, err := newUpstreamHTTPClient(s.cfg.Security.URLAllowlist.Enabled, s.cfg.Security.URLAllowlist.AllowPrivateHosts)
	if err != nil {
		return nil, nil, err
	}
	client, err := NewUpstreamClient(u.Kind, httpClient)
	if err != nil {
		return nil, nil, err
	}
	return &UpstreamAccess{BaseURL: u.BaseURL, Kind: u.Kind, AccessToken: token, UserID: u.RemoteUserID}, client, nil
}

// withUpstreamAccess retries one upstream operation after a New API
// authentication failure. Newer New API releases expire access tokens
// independently of the local record, while older releases keep the same
// login shape; refreshing only on HTTP 401 preserves both behaviours.
func (s *adminUpstreamService) withUpstreamAccess(ctx context.Context, u *Upstream, operation func(*UpstreamAccess, UpstreamClient) error) error {
	access, client, err := s.accessAndClient(ctx, u)
	if err != nil {
		return err
	}
	err = operation(access, client)
	if u.Kind != domain.UpstreamKindNewAPI || !isUpstreamAuthError(err) {
		return err
	}

	login, loginErr := s.loginWithStoredPassword(ctx, u)
	if loginErr != nil {
		return fmt.Errorf("upstream authentication expired; re-login failed: %w (original error: %v)", loginErr, err)
	}
	if applyErr := s.applyNewAPILogin(ctx, u, login); applyErr != nil {
		return fmt.Errorf("refresh upstream authentication: %w", applyErr)
	}
	access, client, err = s.accessAndClient(ctx, u)
	if err != nil {
		return err
	}
	return operation(access, client)
}

func isUpstreamAuthError(err error) bool {
	if err == nil {
		return false
	}
	var httpErr *upstreamHTTPError
	return errors.As(err, &httpErr) && httpErr.StatusCode == http.StatusUnauthorized
}

func (s *adminUpstreamService) loginWithStoredPassword(ctx context.Context, u *Upstream) (*UpstreamLogin, error) {
	if u.PasswordEncrypted == "" || u.LoginIdentifier == "" {
		return nil, fmt.Errorf("stored upstream login credentials are incomplete")
	}
	password, err := s.encryptor.Decrypt(u.PasswordEncrypted)
	if err != nil {
		return nil, fmt.Errorf("decrypt upstream password: %w", err)
	}
	if u.Kind == domain.UpstreamKindNewAPI {
		return loginNewAPI(ctx, u.BaseURL, u.LoginIdentifier, password, s.cfg)
	}
	return loginSub2API(ctx, u.BaseURL, u.LoginIdentifier, password, s.cfg)
}

func (s *adminUpstreamService) applyNewAPILogin(ctx context.Context, u *Upstream, login *UpstreamLogin) error {
	if login == nil || login.AccessToken == "" || login.UserID == "" {
		return fmt.Errorf("new-api login returned incomplete credentials")
	}
	access, err := s.encryptor.Encrypt(login.AccessToken)
	if err != nil {
		return err
	}
	u.TokenEncrypted = access
	u.RemoteUserID = login.UserID
	if login.ExpiresIn > 0 {
		expires := time.Now().UTC().Add(time.Duration(login.ExpiresIn) * time.Second)
		u.TokenExpiresAt = &expires
	} else {
		u.TokenExpiresAt = nil
	}
	return s.repo.Update(ctx, u)
}

func (s *adminUpstreamService) applySub2APILogin(ctx context.Context, u *Upstream, login *UpstreamLogin) error {
	if login == nil || login.AccessToken == "" {
		return fmt.Errorf("Sub2API login returned an empty access token")
	}
	access, err := s.encryptor.Encrypt(login.AccessToken)
	if err != nil {
		return err
	}
	u.TokenEncrypted = access
	u.RefreshTokenEncrypted = ""
	if login.RefreshToken != "" {
		u.RefreshTokenEncrypted, err = s.encryptor.Encrypt(login.RefreshToken)
		if err != nil {
			return err
		}
	}
	if login.ExpiresIn > 0 {
		expires := time.Now().UTC().Add(time.Duration(login.ExpiresIn) * time.Second)
		u.TokenExpiresAt = &expires
	} else {
		u.TokenExpiresAt = nil
	}
	return s.repo.Update(ctx, u)
}

func (s *adminUpstreamService) view(u *Upstream) (*UpstreamView, error) {
	token := ""
	if u.TokenEncrypted != "" {
		var err error
		token, err = s.encryptor.Decrypt(u.TokenEncrypted)
		if err != nil {
			return nil, err
		}
	}
	copy := *u
	copy.TokenEncrypted = ""
	copy.RefreshTokenEncrypted = ""
	copy.PasswordEncrypted = ""
	return &UpstreamView{Upstream: copy, TokenMasked: maskSecret(token)}, nil
}

func (s *adminUpstreamService) resolveRate(ctx context.Context, u *Upstream, groupName string) (float64, string, error) {
	groups, err := s.FetchGroups(ctx, u.ID, false)
	if err != nil {
		return 1, "auto", err
	}
	for _, group := range groups {
		if group.Name == groupName {
			if group.Ratio == nil {
				return 1, "auto", nil
			}
			return *group.Ratio, formatRate(*group.Ratio), nil
		}
	}
	return 1, "auto", nil
}

func (s *adminUpstreamService) validateURL(raw string) (string, error) {
	allowlist := s.cfg.Security.URLAllowlist
	allowedHosts := []string(nil)
	if allowlist.Enabled {
		allowedHosts = allowlist.UpstreamHosts
	}
	return urlvalidator.ValidateHTTPURL(raw, allowlist.AllowInsecureHTTP, urlvalidator.ValidationOptions{AllowedHosts: allowedHosts, RequireAllowlist: allowlist.Enabled, AllowPrivate: allowlist.AllowPrivateHosts})
}
func (s *adminUpstreamService) validateURLOnly(raw string) error {
	_, err := s.validateURL(raw)
	return err
}

func loginSub2API(ctx context.Context, baseURL, email, password string, cfg *config.Config) (*UpstreamLogin, error) {
	client, err := newUpstreamHTTPClient(cfg.Security.URLAllowlist.Enabled, cfg.Security.URLAllowlist.AllowPrivateHosts)
	if err != nil {
		return nil, err
	}
	return doSub2Login(ctx, client, baseURL, "/api/v1/auth/login", map[string]any{"email": email, "password": password})
}

func loginNewAPI(ctx context.Context, baseURL, identifier, password string, cfg *config.Config) (*UpstreamLogin, error) {
	client, err := newUpstreamHTTPClient(cfg.Security.URLAllowlist.Enabled, cfg.Security.URLAllowlist.AllowPrivateHosts)
	if err != nil {
		return nil, err
	}
	data, _ := json.Marshal(map[string]any{"username": identifier, "password": password})
	loginRequest, err := http.NewRequestWithContext(ctx, http.MethodPost, strings.TrimRight(baseURL, "/")+"/api/user/login", bytes.NewReader(data))
	if err != nil {
		return nil, err
	}
	loginRequest.Header.Set("Content-Type", "application/json")
	loginResponse, err := client.Do(loginRequest)
	if err != nil {
		return nil, err
	}
	defer func() { _ = loginResponse.Body.Close() }()
	loginPayload, err := io.ReadAll(io.LimitReader(loginResponse.Body, upstreamResponseLimit))
	if err != nil {
		return nil, err
	}
	if loginResponse.StatusCode < 200 || loginResponse.StatusCode >= 300 {
		return nil, fmt.Errorf("new-api login returned HTTP %d", loginResponse.StatusCode)
	}
	var loginEnvelope struct {
		Success *bool  `json:"success"`
		Message string `json:"message"`
		Data    struct {
			ID          any    `json:"id"`
			AccessToken string `json:"access_token"`
			ExpiresIn   int    `json:"expires_in"`
			User        struct {
				ID any `json:"id"`
			} `json:"user"`
		} `json:"data"`
	}
	if err := json.Unmarshal(loginPayload, &loginEnvelope); err != nil {
		return nil, fmt.Errorf("decode new-api login: %w", err)
	}
	if loginEnvelope.Success != nil && !*loginEnvelope.Success {
		return nil, fmt.Errorf("new-api login failed: %s", strings.TrimSpace(loginEnvelope.Message))
	}
	userID := stringify(loginEnvelope.Data.User.ID)
	if userID == "" {
		userID = stringify(loginEnvelope.Data.ID)
	}
	if userID == "" {
		return nil, fmt.Errorf("new-api login returned an empty user ID")
	}
	if accessToken := strings.TrimSpace(loginEnvelope.Data.AccessToken); accessToken != "" {
		return &UpstreamLogin{AccessToken: accessToken, UserID: userID, ExpiresIn: loginEnvelope.Data.ExpiresIn}, nil
	}

	tokenRequest, err := http.NewRequestWithContext(ctx, http.MethodGet, strings.TrimRight(baseURL, "/")+"/api/user/token", nil)
	if err != nil {
		return nil, err
	}
	tokenRequest.Header.Set("New-Api-User", userID)
	for _, cookie := range loginResponse.Cookies() {
		tokenRequest.AddCookie(cookie)
	}
	tokenResponse, err := client.Do(tokenRequest)
	if err != nil {
		return nil, err
	}
	defer func() { _ = tokenResponse.Body.Close() }()
	tokenPayload, err := io.ReadAll(io.LimitReader(tokenResponse.Body, upstreamResponseLimit))
	if err != nil {
		return nil, err
	}
	if tokenResponse.StatusCode < 200 || tokenResponse.StatusCode >= 300 {
		return nil, fmt.Errorf("new-api access token returned HTTP %d", tokenResponse.StatusCode)
	}
	var tokenEnvelope struct {
		Success *bool  `json:"success"`
		Message string `json:"message"`
		Data    string `json:"data"`
	}
	if err := json.Unmarshal(tokenPayload, &tokenEnvelope); err != nil {
		return nil, fmt.Errorf("decode new-api access token: %w", err)
	}
	if (tokenEnvelope.Success != nil && !*tokenEnvelope.Success) || strings.TrimSpace(tokenEnvelope.Data) == "" {
		return nil, fmt.Errorf("new-api access token failed: %s", strings.TrimSpace(tokenEnvelope.Message))
	}
	return &UpstreamLogin{AccessToken: tokenEnvelope.Data, UserID: userID}, nil
}
func refreshSub2API(ctx context.Context, baseURL, refresh string, cfg *config.Config) (*UpstreamLogin, error) {
	client, err := newUpstreamHTTPClient(cfg.Security.URLAllowlist.Enabled, cfg.Security.URLAllowlist.AllowPrivateHosts)
	if err != nil {
		return nil, err
	}
	return doSub2Login(ctx, client, baseURL, "/api/v1/auth/refresh", map[string]any{"refresh_token": refresh})
}
func doSub2Login(ctx context.Context, client *http.Client, baseURL, path string, body map[string]any) (*UpstreamLogin, error) {
	data, _ := json.Marshal(body)
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, strings.TrimRight(baseURL, "/")+path, strings.NewReader(string(data)))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/json")
	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer func() { _ = resp.Body.Close() }()
	payload, readErr := io.ReadAll(io.LimitReader(resp.Body, upstreamResponseLimit))
	if readErr != nil {
		return nil, readErr
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, fmt.Errorf("sub2api login returned HTTP %d: %s", resp.StatusCode, strings.TrimSpace(string(payload)))
	}
	var envelope struct {
		Data struct {
			AccessToken  string `json:"access_token"`
			RefreshToken string `json:"refresh_token"`
			ExpiresIn    int    `json:"expires_in"`
		} `json:"data"`
		AccessToken  string `json:"access_token"`
		RefreshToken string `json:"refresh_token"`
		ExpiresIn    int    `json:"expires_in"`
	}
	if err := json.Unmarshal(payload, &envelope); err != nil {
		return nil, fmt.Errorf("decode sub2api login: %w", err)
	}
	login := &UpstreamLogin{AccessToken: envelope.Data.AccessToken, RefreshToken: envelope.Data.RefreshToken, ExpiresIn: envelope.Data.ExpiresIn}
	if login.AccessToken == "" {
		login.AccessToken = envelope.AccessToken
	}
	if login.RefreshToken == "" {
		login.RefreshToken = envelope.RefreshToken
	}
	if login.ExpiresIn == 0 {
		login.ExpiresIn = envelope.ExpiresIn
	}
	if login.AccessToken == "" {
		return nil, fmt.Errorf("sub2api login returned empty access token")
	}
	if login.ExpiresIn == 0 {
		login.ExpiresIn = 24 * 60 * 60
	}
	return login, nil
}

func maskSecret(secret string) string {
	if secret == "" {
		return ""
	}
	if len(secret) <= 10 {
		return secret[:minInt(3, len(secret))] + "..."
	}
	return secret[:6] + "..." + secret[len(secret)-4:]
}
func formatRate(rate float64) string {
	return strings.TrimRight(strings.TrimRight(fmt.Sprintf("%.4f", rate), "0"), ".")
}

func modelWhitelistMapping(models []string) map[string]string {
	mapping := make(map[string]string, len(models))
	for _, model := range models {
		model = strings.TrimSpace(model)
		if model != "" {
			mapping[model] = model
		}
	}
	return mapping
}

func accountModelMapping(account *Account) map[string]string {
	if account == nil {
		return nil
	}
	mapping := account.GetModelMapping()
	if len(mapping) > 0 {
		return mapping
	}
	raw, ok := account.Credentials["model_mapping"]
	if !ok {
		return nil
	}
	result := map[string]string{}
	switch values := raw.(type) {
	case map[string]any:
		for key, value := range values {
			if mapped := strings.TrimSpace(stringify(value)); strings.TrimSpace(key) != "" && mapped != "" {
				result[key] = mapped
			}
		}
	case map[string]string:
		for key, value := range values {
			if strings.TrimSpace(key) != "" && strings.TrimSpace(value) != "" {
				result[key] = value
			}
		}
	}
	return result
}

func modelMappingsEqual(left, right map[string]string) bool {
	if len(left) != len(right) {
		return false
	}
	for key, value := range right {
		if left[key] != value {
			return false
		}
	}
	return true
}

func cloneStringAnyMap(input map[string]any) map[string]any {
	output := make(map[string]any, len(input)+1)
	for key, value := range input {
		output[key] = value
	}
	return output
}
func minInt(a, b int) int {
	if a < b {
		return a
	}
	return b
}
