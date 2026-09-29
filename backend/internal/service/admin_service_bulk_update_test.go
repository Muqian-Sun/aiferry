//go:build unit

package service

import (
	"context"
	"errors"
	"reflect"
	"testing"

	"github.com/Wei-Shaw/sub2api/internal/pkg/pagination"
	"github.com/stretchr/testify/require"
)

type accountRepoStubForBulkUpdate struct {
	accountRepoStub
	bulkUpdateErr       error
	bulkUpdateIDs       []int64
	bulkUpdateCalls     int
	lastBulkUpdate      AccountBulkUpdate
	bindGroupErrByID    map[int64]error
	bindGroupsCalls     []int64
	bindGroupsByAccount map[int64][]int64
	createAccount       *Account
	createID            int64
	createErr           error
	updatedAccounts     []*Account
	updateErr           error
	getByIDsAccounts    []*Account
	getByIDsErr         error
	getByIDsCalled      bool
	getByIDsIDs         []int64
	getByIDAccounts     map[int64]*Account
	getByIDErrByID      map[int64]error
	getByIDCalled       []int64
	listByGroupData     map[int64][]Account
	listByGroupErr      map[int64]error
	listData            []Account
	listResult          *pagination.PaginationResult
	listErr             error
	listCalled          bool
	lastListParams      pagination.PaginationParams
	lastListFilters     struct {
		platform    string
		accountType string
		status      string
		search      string
		privacyMode string
	}
}

func (s *accountRepoStubForBulkUpdate) BulkUpdate(_ context.Context, ids []int64, updates AccountBulkUpdate) (int64, error) {
	s.bulkUpdateCalls++
	s.bulkUpdateIDs = append([]int64{}, ids...)
	s.lastBulkUpdate = updates
	if s.bulkUpdateErr != nil {
		return 0, s.bulkUpdateErr
	}
	return int64(len(ids)), nil
}

func (s *accountRepoStubForBulkUpdate) Create(_ context.Context, account *Account) error {
	s.createAccount = account
	if s.createID > 0 {
		account.ID = s.createID
	}
	return s.createErr
}

func (s *accountRepoStubForBulkUpdate) Update(_ context.Context, account *Account) error {
	s.updatedAccounts = append(s.updatedAccounts, account)
	return s.updateErr
}

func (s *accountRepoStubForBulkUpdate) GetByIDs(_ context.Context, ids []int64) ([]*Account, error) {
	s.getByIDsCalled = true
	s.getByIDsIDs = append([]int64{}, ids...)
	if s.getByIDsErr != nil {
		return nil, s.getByIDsErr
	}
	return s.getByIDsAccounts, nil
}

func (s *accountRepoStubForBulkUpdate) GetByID(_ context.Context, id int64) (*Account, error) {
	s.getByIDCalled = append(s.getByIDCalled, id)
	if err, ok := s.getByIDErrByID[id]; ok {
		return nil, err
	}
	if account, ok := s.getByIDAccounts[id]; ok {
		return account, nil
	}
	return nil, errors.New("account not found")
}

func (s *accountRepoStubForBulkUpdate) ListAllWithFilters(context.Context, string, string, string, string, string) ([]Account, error) {
	return nil, nil
}

func (s *accountRepoStubForBulkUpdate) ListWithFilters(_ context.Context, params pagination.PaginationParams, platform, accountType, status, search string, privacyMode string) ([]Account, *pagination.PaginationResult, error) {
	s.listCalled = true
	s.lastListParams = params
	s.lastListFilters.platform = platform
	s.lastListFilters.accountType = accountType
	s.lastListFilters.status = status
	s.lastListFilters.search = search
	s.lastListFilters.privacyMode = privacyMode
	if s.listErr != nil {
		return nil, nil, s.listErr
	}
	if s.listResult != nil {
		return s.listData, s.listResult, nil
	}
	return s.listData, &pagination.PaginationResult{Total: int64(len(s.listData))}, nil
}

// TestAdminService_BulkUpdateAccounts_AllSuccessIDs 验证批量更新成功时返回 success_ids/failed_ids。
func TestAdminService_BulkUpdateAccounts_AllSuccessIDs(t *testing.T) {
	repo := &accountRepoStubForBulkUpdate{}
	svc := &adminServiceImpl{accountRepo: repo}

	schedulable := true
	input := &BulkUpdateAccountsInput{
		AccountIDs:  []int64{1, 2, 3},
		Schedulable: &schedulable,
	}

	result, err := svc.BulkUpdateAccounts(context.Background(), input)
	require.NoError(t, err)
	require.Equal(t, 3, result.Success)
	require.Equal(t, 0, result.Failed)
	require.ElementsMatch(t, []int64{1, 2, 3}, result.SuccessIDs)
	require.Empty(t, result.FailedIDs)
	require.Len(t, result.Results, 3)
}

func TestAdminServiceBulkUpdateAccounts_ResolvesIDsFromFilters(t *testing.T) {
	repo := &accountRepoStubForBulkUpdate{
		listData: []Account{
			{ID: 7},
			{ID: 11},
		},
		listResult: &pagination.PaginationResult{Total: 2},
	}
	svc := &adminServiceImpl{accountRepo: repo}

	schedulable := true
	input := &BulkUpdateAccountsInput{
		Schedulable: &schedulable,
	}

	filtersField := reflect.ValueOf(input).Elem().FieldByName("Filters")
	require.True(t, filtersField.IsValid(), "BulkUpdateAccountsInput should expose Filters for filter-target bulk update")
	require.Equal(t, reflect.Ptr, filtersField.Kind(), "BulkUpdateAccountsInput.Filters should be a pointer field")

	filtersValue := reflect.New(filtersField.Type().Elem())
	filtersValue.Elem().FieldByName("Platform").SetString(PlatformOpenAI)
	filtersValue.Elem().FieldByName("Type").SetString(AccountTypeOAuth)
	filtersValue.Elem().FieldByName("Status").SetString(StatusActive)
	filtersValue.Elem().FieldByName("Group").SetString("12")
	filtersValue.Elem().FieldByName("PrivacyMode").SetString(PrivacyModeCFBlocked)
	filtersValue.Elem().FieldByName("Search").SetString("bulk-target")
	filtersField.Set(filtersValue)

	result, err := svc.BulkUpdateAccounts(context.Background(), input)
	require.NoError(t, err)
	require.True(t, repo.listCalled, "expected filter-target bulk update to resolve matching IDs via account list filters")
	require.Equal(t, PlatformOpenAI, repo.lastListFilters.platform)
	require.Equal(t, AccountTypeOAuth, repo.lastListFilters.accountType)
	require.Equal(t, StatusActive, repo.lastListFilters.status)
	require.Equal(t, "bulk-target", repo.lastListFilters.search)
	require.Equal(t, PrivacyModeCFBlocked, repo.lastListFilters.privacyMode)
	require.Equal(t, []int64{7, 11}, repo.bulkUpdateIDs)
	require.Equal(t, 2, result.Success)
	require.Equal(t, 0, result.Failed)
	require.Equal(t, []int64{7, 11}, result.SuccessIDs)
}
