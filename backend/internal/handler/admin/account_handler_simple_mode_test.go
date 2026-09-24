package admin

import (
	"context"

	"github.com/Wei-Shaw/sub2api/internal/service"
)

type simpleModeAccountService struct {
	*stubAdminService
	account     service.Account
	createCalls int
	updateCalls int
	bulkCalls   int
}

func (s *simpleModeAccountService) GetAccount(context.Context, int64) (*service.Account, error) {
	return &s.account, nil
}

func (s *simpleModeAccountService) CreateAccount(ctx context.Context, input *service.CreateAccountInput) (*service.Account, error) {
	s.createCalls++
	return &s.account, nil
}

func (s *simpleModeAccountService) UpdateAccount(ctx context.Context, _ int64, input *service.UpdateAccountInput) (*service.Account, error) {
	s.updateCalls++
	return &s.account, nil
}

func (s *simpleModeAccountService) BulkUpdateAccounts(ctx context.Context, input *service.BulkUpdateAccountsInput) (*service.BulkUpdateAccountsResult, error) {
	s.bulkCalls++
	return &service.BulkUpdateAccountsResult{}, nil
}
