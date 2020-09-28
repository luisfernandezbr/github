package internal

import (
	"fmt"

	"github.com/pinpt/agent/v4/sdk"
)

const cacheKeyWorkConfig = "work_config"

func (i *GithubIntegration) processWorkConfig(config sdk.Config, pipe sdk.Pipe, istate sdk.State, customerID string, integrationInstanceID string, historical bool) error {
	if historical || !istate.Exists(cacheKeyWorkConfig) {
		var wc sdk.WorkConfig
		wc.ID = sdk.NewWorkConfigID(customerID, refType, integrationInstanceID)
		wc.IntegrationInstanceID = integrationInstanceID
		wc.CustomerID = customerID
		wc.RefType = refType
		wc.Statuses = sdk.WorkConfigStatuses{
			OpenStatus:       []string{"OPEN"},
			InProgressStatus: []string{},
			ClosedStatus:     []string{"CLOSED"},
		}
		wc.UpdatedAt = sdk.EpochNow()
		if err := pipe.Write(&wc); err != nil {
			return err
		}
		if err := istate.Set(cacheKeyWorkConfig, wc.Hash()); err != nil {
			return fmt.Errorf("error writing work status config key to cache: %w", err)
		}
	}
	return nil
}
