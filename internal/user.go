package internal

import (
	"sync"

	"github.com/pinpt/agent.next/sdk"
)

var getUserOrgsQuery = `
query GetUser($id: ID!) {
	node(id: $id) {
		... on User {
			organizations(first: 100) {
				nodes {
					login
				}
			}
		}
	}
	rateLimit {
		limit
		cost
		remaining
		resetAt
	}
}
`

type userOrg struct {
	Login string `json:"login"`
}

type userOrgNode struct {
	Nodes []userOrg `json:"nodes"`
}

type userOrgs struct {
	Organizations userOrgNode `json:"organizations"`
}

type userOrgResult struct {
	Node      userOrgs  `json:"node"`
	RateLimit rateLimit `json:"rateLimit"`
}

// UserManager is a manager for users
type UserManager struct {
	customerID  string
	orgs        []string
	users       map[string]bool
	export      sdk.Export
	pipe        sdk.Pipe
	integration *GithubIntegration
	mu          sync.Mutex
}

func (u *UserManager) isMemberOfOrg(orgName string) bool {
	for _, login := range u.orgs {
		if login == orgName {
			return true
		}
	}
	return false
}

func (u *UserManager) emitAuthor(author author) error {
	refID := author.RefID(u.customerID)
	if refID == "" {
		return nil
	}
	// hold lock while we determine if this is a user we need to lookup
	u.mu.Lock()
	if u.users[refID] {
		u.mu.Unlock()
		return nil
	}
	user := author.ToModel(u.customerID)
	if user.Type == sdk.SourceCodeUserTypeHuman {
		for {
			// go to GitHub and find out if this user is a current member of our organization
			var result userOrgResult
			sdk.LogDebug(u.integration.logger, "need to fetch user org details", "ref_id", refID)
			if err := u.integration.client.Query(getUserOrgsQuery, map[string]interface{}{"id": author.ID}, &result); err != nil {
				u.mu.Unlock()
				if u.integration.checkForAbuseDetection(u.export, err) {
					u.mu.Lock()
					continue
				}
				if u.integration.checkForRetryableError(u.export, err) {
					u.mu.Lock()
					continue
				}
				sdk.LogError(u.integration.logger, "error fetching user", "err", err, "ref_id", refID)
				return err
			}
			var ismember bool
			for _, node := range result.Node.Organizations.Nodes {
				if u.isMemberOfOrg(node.Login) {
					ismember = true
					break
				}
			}
			user.Member = ismember
			if err := u.integration.checkForRateLimit(u.export, result.RateLimit); err != nil {
				u.mu.Unlock()
				return err
			}
			break
		}
	}
	u.users[refID] = true
	u.mu.Unlock()
	return u.pipe.Write(user)
}

func (u *UserManager) emitGitUser(author gitUser) error {
	refID := author.RefID(u.customerID)
	if refID == "" {
		return nil
	}
	// hold lock while we determine if this is a user we need to lookup
	u.mu.Lock()
	if u.users[refID] {
		u.mu.Unlock()
		return nil
	}
	user := author.ToModel(u.customerID)
	if user.Type == sdk.SourceCodeUserTypeHuman && author.User.ID != "" {
		for {
			// go to GitHub and find out if this user is a current member of our organization
			var result userOrgResult
			sdk.LogDebug(u.integration.logger, "need to fetch user org details", "ref_id", refID)
			if err := u.integration.client.Query(getUserOrgsQuery, map[string]interface{}{"id": author.User.ID}, &result); err != nil {
				u.mu.Unlock()
				if u.integration.checkForAbuseDetection(u.export, err) {
					u.mu.Lock()
					continue
				}
				if u.integration.checkForRetryableError(u.export, err) {
					u.mu.Lock()
					continue
				}
				sdk.LogError(u.integration.logger, "error fetching user", "err", err, "ref_id", refID)
				return err
			}
			var ismember bool
			for _, node := range result.Node.Organizations.Nodes {
				if u.isMemberOfOrg(node.Login) {
					ismember = true
					break
				}
			}
			user.Member = ismember
			if err := u.integration.checkForRateLimit(u.export, result.RateLimit); err != nil {
				u.mu.Unlock()
				return err
			}
			break
		}
	}
	u.users[refID] = true
	u.mu.Unlock()
	return u.pipe.Write(user)
}

// NewUserManager returns a new instance
func NewUserManager(customerID string, orgs []string, export sdk.Export, pipe sdk.Pipe, integration *GithubIntegration) *UserManager {
	return &UserManager{
		customerID:  customerID,
		orgs:        orgs,
		users:       make(map[string]bool),
		export:      export,
		pipe:        pipe,
		integration: integration,
	}
}
