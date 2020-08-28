package internal

import (
	"fmt"

	"github.com/pinpt/agent.next/sdk"
)

func (g *GithubIntegration) fetchOrgAccounts(logger sdk.Logger, client sdk.GraphQLClient, control sdk.Control) ([]*sdk.ConfigAccount, error) {
	orgs, err := g.fetchOrgs(logger, client, control)
	if err != nil {
		return nil, fmt.Errorf("error fetching orgs: %w", err)
	}
	var accounts []*sdk.ConfigAccount
	for _, org := range orgs {
		accounts = append(accounts, &sdk.ConfigAccount{
			ID:     org.Login,
			Type:   sdk.ConfigAccountTypeOrg,
			Public: false,
		})
	}
	return accounts, nil
}

func (g *GithubIntegration) fetchViewerAccount(logger sdk.Logger, client sdk.GraphQLClient, control sdk.Control) (*sdk.ConfigAccount, error) {
	viewer, err := g.fetchViewer(logger, client, control)
	if err != nil {
		return nil, err
	}
	var acct sdk.ConfigAccount
	acct.ID = viewer
	acct.Public = false
	acct.Type = sdk.ConfigAccountTypeUser
	return &acct, nil
}

func toConfigAccounts(accounts []*sdk.ConfigAccount) *sdk.ConfigAccounts {
	res := make(sdk.ConfigAccounts)
	for _, account := range accounts {
		res[account.ID] = account
	}
	return &res
}

// Validate the github integration
func (g *GithubIntegration) Validate(validate sdk.Validate) (map[string]interface{}, error) {
	logger := g.logger
	_, client, err := g.newGraphClient(logger, validate.Config())
	if err != nil {
		return nil, fmt.Errorf("error creating client: %w", err)
	}
	accounts, err := g.fetchOrgAccounts(logger, client, validate)
	if err != nil {
		return nil, fmt.Errorf("error fetching org accounts: %w", err)
	}
	account, err := g.fetchViewerAccount(logger, client, validate)
	if err != nil {
		return nil, fmt.Errorf("error fetching viewer accounts: %w", err)
	}
	accounts = append(accounts, account)
	res := map[string]interface{}{
		"accounts": accounts,
	}
	return res, nil
}
