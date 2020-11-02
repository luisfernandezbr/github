package internal

import (
	"strconv"
	"strings"

	"github.com/google/go-github/v32/github"
	"github.com/pinpt/agent/v4/sdk"
)

func isBot(name string) bool {
	return strings.HasSuffix(name, "[bot]") || strings.HasSuffix(name, "-bot") || strings.HasSuffix(name, " Bot") || name == "GitHub"
}

type authorCommon struct {
	Email  string `json:"email"`
	Name   string `json:"name"`
	Avatar string `json:"avatarUrl"`
	Login  string `json:"login"`
	URL    string `json:"url"`
	Type   string `json:"type"`
}

func (a authorCommon) RefID(customerID string) string {
	// FIXME: review how we do this in current agent to match
	switch a.Type {
	case "Bot":
		return ""
	case "Mannequin":
	}
	if a.Email != "" {
		return sdk.Hash(customerID, a.Email)
	}
	return "" // FIXME
}

func (a authorCommon) ToModel(customerID string, integrationInstanceID string) *sdk.SourceCodeUser {
	user := &sdk.SourceCodeUser{}
	user.CustomerID = customerID
	user.RefType = refType
	user.ID = sdk.NewSourceCodeUserID(customerID, refType, user.RefID)
	user.IntegrationInstanceID = sdk.StringPointer(integrationInstanceID)
	user.URL = sdk.StringPointer(a.URL)
	user.AvatarURL = sdk.StringPointer(a.Avatar)
	user.Email = sdk.StringPointer(a.Email)
	if user.Email != nil {
		id := sdk.Hash(customerID, a.Email)
		if id != user.RefID {
			user.AssociatedRefID = sdk.StringPointer(id)
		}
	}
	user.Name = a.Name
	switch a.Type {
	case "Bot":
		user.Type = sdk.SourceCodeUserTypeBot
	case "User":
		user.Type = sdk.SourceCodeUserTypeHuman
		user.Username = sdk.StringPointer(a.Login)
	case "Mannequin":
	}
	if user.RefID == "" || isBot(a.Name) {
		user.Type = sdk.SourceCodeUserTypeBot
	}
	return user
}

type author struct {
	authorCommon
	ID string `json:"id"`
}

type author2 struct {
	authorCommon
	ID int64 `json:"id"`
}

func (a author) ToModel(customerID string, integrationInstanceID string) *sdk.SourceCodeUser {
	u := a.authorCommon.ToModel(customerID, integrationInstanceID)

	u.RefID = a.RefID(customerID)

	return u
}

func (a author2) ToModel(customerID string, integrationInstanceID string) *sdk.SourceCodeUser {
	u := a.authorCommon.ToModel(customerID, integrationInstanceID)

	u.RefID = a.RefID(customerID)

	return u
}

func (a author) RefID(customerID string) string {

	// FIXME: review how we do this in current agent to match
	if a.Type == "User" {
		return a.ID
	}
	return a.authorCommon.RefID(customerID)
}

func (a author2) RefID(customerID string) string {
	// FIXME: review how we do this in current agent to match
	if a.Type == "User" {
		return strconv.FormatInt(a.ID, 10)
	}
	return a.authorCommon.RefID(customerID)

}

type gitUser struct {
	Name   string `json:"name"`
	Email  string `json:"email"`
	Avatar string `json:"avatarUrl"`
	User   author `json:"user"`
}

func (a gitUser) RefID(customerID string) string {
	if a.User.ID != "" {
		return a.User.ID
	}
	if a.Email != "" {
		return sdk.Hash(customerID, a.Email)
	}
	return ""
}

func (a gitUser) ToModel(customerID string, integrationInstanceID string) *sdk.SourceCodeUser {
	user := &sdk.SourceCodeUser{}
	user.CustomerID = customerID
	user.RefID = a.RefID(customerID)
	user.RefType = refType
	if a.Email != "" {
		id := sdk.Hash(customerID, a.Email)
		if id != user.RefID {
			user.AssociatedRefID = sdk.StringPointer(id)
		}
	}
	user.IntegrationInstanceID = sdk.StringPointer(integrationInstanceID)
	user.URL = sdk.StringPointer(a.User.URL)
	user.AvatarURL = sdk.StringPointer(a.Avatar)
	user.Email = sdk.StringPointer(a.Email)
	user.Name = a.Name
	switch a.User.Type {
	case "Bot":
		user.Type = sdk.SourceCodeUserTypeBot
		user.Username = sdk.StringPointer(a.User.Login)
	case "User":
		user.Type = sdk.SourceCodeUserTypeHuman
		user.Username = sdk.StringPointer(a.User.Login)
	case "Mannequin":
	}
	if user.RefID == "" || isBot(a.Name) {
		user.Type = sdk.SourceCodeUserTypeBot
	}
	return user
}

func userToAuthor(user *github.User) author {
	var author author
	if user != nil && user.ID != nil {
		author.ID = user.GetNodeID()
	}
	author.Avatar = user.GetAvatarURL()
	author.Email = user.GetEmail()
	author.Login = user.GetLogin()
	author.Name = user.GetName()
	author.URL = user.GetHTMLURL()
	author.Type = "User"
	return author
}

func commitUserToAuthor(user *github.CommitAuthor) gitUser {
	var author gitUser
	author.Email = user.GetEmail()
	author.Name = user.GetName()
	return author
}
