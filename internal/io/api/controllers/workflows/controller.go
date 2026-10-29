package workflows

import "github.com/doxanocap/blotato/internal/io/api/controllers/workflows/contracts"

type Controller struct {
	service   contracts.Service
	execution contracts.ExecutionService
	sender    contracts.FakeSender
	accountID string
}

func New(
	service contracts.Service,
	execution contracts.ExecutionService,
	sender contracts.FakeSender,
	accountID string,
) Controller {
	return Controller{
		service:   service,
		execution: execution,
		sender:    sender,
		accountID: accountID,
	}
}
