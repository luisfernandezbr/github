package internal

import (
	"strings"

	"github.com/pinpt/go-common/hash"
	ps "github.com/pinpt/go-common/strings"
	"github.com/pinpt/integration-sdk/sourcecode"
)

func isBot(name string) bool {
	return strings.HasSuffix(name, "[bot]") || strings.HasSuffix(name, "-bot") || strings.HasSuffix(name, " Bot") || name == "GitHub"
}

type author struct {
	ID     string `json:"id"`
	Email  string `json:"email"`
	Name   string `json:"name"`
	Avatar string `json:"avatarUrl"`
	Login  string `json:"login"`
	URL    string `json:"url"`
	Type   string `json:"type"`
}

func (a author) ToModel(customerID string) *sourcecode.User {
	user := &sourcecode.User{}
	user.CustomerID = customerID
	user.RefID = a.RefID(customerID)
	user.RefType = refType
	user.ID = sourcecode.NewUserID(customerID, refType, user.RefID)
	user.URL = ps.Pointer(a.URL)
	user.AvatarURL = ps.Pointer(a.Avatar)
	user.Email = ps.Pointer(a.Email)
	if user.Email != nil {
		id := hash.Values(customerID, a.Email)
		if id != user.RefID {
			user.AssociatedRefID = ps.Pointer(id)
		}
	}
	user.Name = a.Name
	switch a.Type {
	case "Bot":
		user.Type = sourcecode.UserTypeBot
	case "User":
		user.Type = sourcecode.UserTypeHuman
		user.Username = ps.Pointer(a.Login)
	case "Mannequin":
	}
	if user.RefID == "" || isBot(a.Name) {
		user.Type = sourcecode.UserTypeBot
	}
	return user
}

func (a author) RefID(customerID string) string {
	// FIXME: review how we do this in current agent to match
	switch a.Type {
	case "Bot":
		return ""
	case "User":
		return a.ID
	case "Mannequin":
	}
	if a.Email != "" {
		return hash.Values(customerID, a.Email)
	}
	return "" // FIXME
}

type gitUser struct {
	Name   string `json:"name"`
	Email  string `json:"email"`
	Avatar string `json:"avatarUrl"`
	User   author `json:"user"`
}

func (a gitUser) RefID(customerID string) string {
	// FIXME
	if a.User.ID != "" {
		return a.User.ID
	}
	if a.Email != "" {
		return hash.Values(customerID, a.Email)
	}
	return ""
}

func (a gitUser) ToModel(customerID string) *sourcecode.User {
	user := &sourcecode.User{}
	user.CustomerID = customerID
	user.RefID = a.RefID(customerID)
	user.RefType = refType
	if a.Email != "" {
		id := hash.Values(customerID, a.Email)
		if id != user.RefID {
			user.AssociatedRefID = ps.Pointer(id)
		}
	}
	user.URL = ps.Pointer(a.User.URL)
	user.AvatarURL = ps.Pointer(a.Avatar)
	user.Email = ps.Pointer(a.Email)
	user.Name = a.Name
	switch a.User.Type {
	case "Bot":
		user.Type = sourcecode.UserTypeBot
	case "User":
		user.Type = sourcecode.UserTypeHuman
		user.Username = ps.Pointer(a.User.Login)
	case "Mannequin":
	}
	if user.RefID == "" || isBot(a.Name) {
		user.Type = sourcecode.UserTypeBot
	}
	return user
}
