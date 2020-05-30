package internal

type author struct {
	ID     string  `json:"id"`
	Email  string  `json:"email"`
	Name   string  `json:"name"`
	Avatar string  `json:"avatarUrl"`
	Login  string  `json:"login"`
	BotURL *string `json:"boturl"`
}

func (a author) RefID() string {
	// FIXME: this doesn't seem right but tried to following current agent
	// https://github.com/pinpt/agent/blob/afcc3e5b585a1902eeeaec89e37424f651818e6f/integrations/github/user.go#L199
	if a.Name == "GitHub" && a.Email == "noreply@github.com" {
		return "github-noreply"
	}
	if a.Login == "" {
		return ""
	}
	return a.Login
}

type gitUser struct {
	Name   string `json:"name"`
	Email  string `json:"email"`
	Avatar string `json:"avatarUrl"`
	User   author `json:"user"`
}

func (a gitUser) RefID() string {
	// FIXME
	return a.User.Login
}
