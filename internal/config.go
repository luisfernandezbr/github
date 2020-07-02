package internal

import "github.com/pinpt/agent.next/sdk"

type accountType string

const (
	orgAccountType  accountType = "ORG"
	userAccountType accountType = "USER"
)

type configAccount struct {
	ID     string      `json:"id"`
	Type   accountType `json:"type"`
	Public bool        `json:"public"`
}

type accounts map[string]*configAccount

func parseAccounts(config sdk.Config) (accounts, error) {
	var a accounts
	err := config.From("accounts", &a)
	return a, err
}
