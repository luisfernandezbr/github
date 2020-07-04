package internal

import (
	"sync"
	"time"

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
	state       sdk.State
	historical  bool
}

const userStatecacheKey = "user_"

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
	user := author.ToModel(u.customerID, u.instanceid)
	hash := user.Hash()
	cachekey := userStatecacheKey + refID
	u.users[refID] = true
	if !u.historical {
		var cacheValue string
		found, _ := u.state.Get(cachekey, &cacheValue)
		if found && cacheValue == hash {
			// already cached with the same hashcode so we can skip emit
			u.mu.Unlock()
			return nil
		}
	}
	u.mu.Unlock()
	if err := u.pipe.Write(user); err != nil {
		return err
	}
	return u.state.SetWithExpires(cachekey, hash, time.Hour)
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
	hash := user.Hash()
	cachekey := userStatecacheKey + refID
	u.users[refID] = true
	if !u.historical {
		var cacheValue string
		found, _ := u.state.Get(cachekey, &cacheValue)
		if found && cacheValue == hash {
			// already cached with the same hashcode so we can skip emit
			u.mu.Unlock()
			return nil
		}
	}
	u.mu.Unlock()
	if err := u.pipe.Write(user); err != nil {
		return err
	}
	return u.state.SetWithExpires(cachekey, hash, time.Hour)
}

// NewUserManager returns a new instance
func NewUserManager(customerID string, orgs []string, control sdk.Control, state sdk.State, pipe sdk.Pipe, integration *GithubIntegration, instanceid string, historical bool) *UserManager {
	return &UserManager{
		customerID:  customerID,
		orgs:        orgs,
		users:       make(map[string]bool),
		control:     control,
		pipe:        pipe,
		state:       state,
		integration: integration,
		instanceid:  instanceid,
		historical:  historical,
	}
}
