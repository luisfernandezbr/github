package internal

import "github.com/pinpt/go-common/hash"

type author struct {
	ID     string `json:"id"`
	Email  string `json:"email"`
	Name   string `json:"name"`
	Avatar string `json:"avatarUrl"`
	Login  string `json:"login"`
	URL    string `json:"url"`
	Type   string `json:"type"`
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
	return a.User.Login
}
