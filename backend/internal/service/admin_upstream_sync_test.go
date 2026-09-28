package service

import (
	"context"
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/domain"
	"github.com/stretchr/testify/require"
)

type upstreamSyncRepositoryStub struct {
	UpstreamRepository
	resource *UpstreamResource
	upstream *Upstream
}

func (s *upstreamSyncRepositoryStub) GetResource(_ context.Context, _ int64) (*UpstreamResource, error) {
	return s.resource, nil
}

func (s *upstreamSyncRepositoryStub) GetByID(_ context.Context, _ int64) (*Upstream, error) {
	return s.upstream, nil
}

func (s *upstreamSyncRepositoryStub) MarkResourceSynced(_ context.Context, _ int64, platform string, accountID int64, rate float64, syncedAt time.Time) error {
	for i := range s.resource.SyncedAccounts {
		if s.resource.SyncedAccounts[i].Platform == platform {
			s.resource.SyncedAccounts[i] = UpstreamSyncedAccount{Platform: platform, AccountID: accountID, RateMultiplier: rate, SyncedAt: syncedAt}
			return nil
		}
	}
	s.resource.SyncedAccounts = append(s.resource.SyncedAccounts, UpstreamSyncedAccount{Platform: platform, AccountID: accountID, RateMultiplier: rate, SyncedAt: syncedAt})
	return nil
}

type upstreamSyncAdminStub struct {
	AdminService
	groups   map[int64]*Group
	accounts map[int64]*Account
	created  []*CreateAccountInput
	updated  []*UpdateAccountInput
}

func (s *upstreamSyncAdminStub) ValidateAccountGroupBindings(context.Context, []int64) error {
	return nil
}

func (s *upstreamSyncAdminStub) GetGroup(_ context.Context, id int64) (*Group, error) {
	group, ok := s.groups[id]
	if !ok {
		return nil, ErrGroupNotFound
	}
	return group, nil
}

func (s *upstreamSyncAdminStub) GetAccount(_ context.Context, id int64) (*Account, error) {
	account, ok := s.accounts[id]
	if !ok {
		return nil, ErrAccountNotFound
	}
	return account, nil
}

func (s *upstreamSyncAdminStub) CreateAccount(_ context.Context, input *CreateAccountInput) (*Account, error) {
	s.created = append(s.created, input)
	id := int64(100 + len(s.created))
	account := &Account{ID: id, Platform: input.Platform, Type: input.Type, Credentials: input.Credentials}
	s.accounts[id] = account
	return account, nil
}

func (s *upstreamSyncAdminStub) UpdateAccount(_ context.Context, id int64, input *UpdateAccountInput) (*Account, error) {
	s.updated = append(s.updated, input)
	account := s.accounts[id]
	account.Credentials = input.Credentials
	return account, nil
}

func newUpstreamSyncTestService() (*adminUpstreamService, *upstreamSyncRepositoryStub, *upstreamSyncAdminStub) {
	rate := 1.25
	repo := &upstreamSyncRepositoryStub{
		resource: &UpstreamResource{ID: 7, UpstreamID: 3, GroupName: "premium", KeyEncrypted: "encrypted:remote-key", ModelsSnapshot: []string{"model-a"}},
		upstream: &Upstream{ID: 3, Name: "relay", BaseURL: "https://relay.example.com", GroupSnapshot: domain.UpstreamGroupSnapshot{
			Items: []domain.UpstreamGroupItem{{Name: "premium", Ratio: &rate}},
		}},
	}
	admin := &upstreamSyncAdminStub{groups: map[int64]*Group{
		1: {ID: 1, Name: "openai-a", Platform: PlatformOpenAI, Status: StatusActive},
		2: {ID: 2, Name: "anthropic-a", Platform: PlatformAnthropic, Status: StatusActive},
		3: {ID: 3, Name: "openai-b", Platform: PlatformOpenAI, Status: StatusActive},
		4: {ID: 4, Name: "composite", Platform: PlatformComposite, Status: StatusActive},
		5: {ID: 5, Name: "oauth-only", Platform: PlatformGemini, Status: StatusActive, RequireOAuthOnly: true},
		6: {ID: 6, Name: "disabled", Platform: PlatformGrok, Status: StatusDisabled},
	}, accounts: make(map[int64]*Account)}
	return &adminUpstreamService{repo: repo, admin: admin, encryptor: adminUpstreamTestEncryptor{}}, repo, admin
}

func TestUpstreamSyncCreatesOneAccountPerSelectedPlatform(t *testing.T) {
	svc, repo, admin := newUpstreamSyncTestService()
	result, err := svc.SyncToAccount(context.Background(), 7, []int64{1, 2, 3, 1})
	require.NoError(t, err)
	require.Equal(t, 2, result.Created)
	require.Zero(t, result.Failed)
	require.Len(t, repo.resource.SyncedAccounts, 2)
	require.Len(t, admin.created, 2)
	for _, input := range admin.created {
		require.Equal(t, AccountTypeAPIKey, input.Type)
		require.Equal(t, "remote-key", input.Credentials["api_key"])
		require.Equal(t, "https://relay.example.com", input.Credentials["base_url"])
		require.Equal(t, 1.25, *input.RateMultiplier)
		switch input.Platform {
		case PlatformOpenAI:
			require.ElementsMatch(t, []int64{1, 3}, input.GroupIDs)
		case PlatformAnthropic:
			require.Equal(t, []int64{2}, input.GroupIDs)
		default:
			t.Fatalf("unexpected platform %q", input.Platform)
		}
	}
	require.Equal(t, "relay-anthropic-1.25", admin.created[0].Name)

	result, err = svc.SyncToAccount(context.Background(), 7, []int64{1, 2, 3})
	require.NoError(t, err)
	require.Equal(t, 2, result.Skipped)
	require.Len(t, admin.created, 2)

	repo.resource.ModelsSnapshot = []string{"model-b"}
	result, err = svc.SyncToAccount(context.Background(), 7, []int64{1, 2, 3})
	require.NoError(t, err)
	require.Equal(t, 2, result.Updated)
	require.Len(t, admin.updated, 2)
	for _, input := range admin.updated {
		require.Nil(t, input.GroupIDs)
	}
	require.Len(t, admin.created, 2)
}

func TestUpstreamSyncAccountNameFitsAccountLimit(t *testing.T) {
	name := upstreamSyncAccountName("", PlatformOpenAI, "1.25")
	require.Equal(t, "-openai-1.25", name)
	longName := upstreamSyncAccountName(strings.Repeat("中", 100), PlatformAnthropic, "1.25")
	require.LessOrEqual(t, len([]rune(longName)), 100)
	require.Contains(t, longName, "-anthropic-1.25")
	extremeRate := upstreamSyncAccountName("relay", PlatformOpenAI, strings.Repeat("9", 120))
	require.LessOrEqual(t, len([]rune(extremeRate)), 100)
}

func TestUpstreamSyncRejectsUnsupportedAndOAuthOnlyGroupsBeforeCreatingAccounts(t *testing.T) {
	for _, groupID := range []int64{4, 5, 6} {
		t.Run(fmt.Sprint(groupID), func(t *testing.T) {
			svc, _, admin := newUpstreamSyncTestService()
			_, err := svc.SyncToAccount(context.Background(), 7, []int64{1, groupID})
			require.Error(t, err)
			require.Empty(t, admin.created)
		})
	}
}
