package internal

import (
	"sync"

	"github.com/pinpt/agent.next/sdk"
)

// UserManager is a manager for users
// easyjson:skip
type UserManager struct {
	customerID  string
	orgs        []string
	users       map[string]bool
	control     sdk.Control
	pipe        sdk.Pipe
	integration *GithubIntegration
	mu          sync.Mutex
	instanceid  string
}

func (u *UserManager) emitAuthor(logger sdk.Logger, author author) error {
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
	u.users[refID] = true
	u.mu.Unlock()
	return u.pipe.Write(user)
}

func (u *UserManager) emitGitUser(logger sdk.Logger, author gitUser) error {
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
	user := author.ToModel(u.customerID, u.instanceid)
	u.users[refID] = true
	u.mu.Unlock()
	return u.pipe.Write(user)
}

// NewUserManager returns a new instance
func NewUserManager(customerID string, orgs []string, control sdk.Control, pipe sdk.Pipe, integration *GithubIntegration, instanceid string) *UserManager {
	return &UserManager{
		customerID:  customerID,
		orgs:        orgs,
		users:       make(map[string]bool),
		control:     control,
		pipe:        pipe,
		integration: integration,
		instanceid:  instanceid,
	}
}
