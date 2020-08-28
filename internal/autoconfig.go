package internal

import (
	"fmt"

	"github.com/pinpt/agent.next/sdk"
)

// AutoConfigure is called when a cloud integration has requested to be auto configured
func (g *GithubIntegration) AutoConfigure(autoconfig sdk.AutoConfigure) (*sdk.Config, error) {
	config := autoconfig.Config()
	logger := g.logger
	_, client, err := g.newGraphClient(logger, config)
	if err != nil {
		return nil, fmt.Errorf("error creating graphql client: %w", err)
	}
	var accounts []*sdk.ConfigAccount
	if config.Scope != nil && *config.Scope == sdk.OrgScope {
		accounts, err = g.fetchOrgAccounts(logger, client, autoconfig)
		if err != nil {
			return nil, err
		}
	} else {
		account, err := g.fetchViewerAccount(logger, client, autoconfig)
		if err != nil {
			return nil, err
		}
		accounts = []*sdk.ConfigAccount{account}
	}
	config.Accounts = toConfigAccounts(accounts)
	return &config, nil
}
