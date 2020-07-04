package internal

import (
	"time"

	"github.com/google/go-github/v32/github"
	"github.com/pinpt/agent.next/sdk"
)

type repository struct {
	ID            string       `json:"id"`
	Name          string       `json:"nameWithOwner"`
	URL           string       `json:"url"`
	UpdatedAt     time.Time    `json:"updatedAt"`
	Description   string       `json:"description"`
	Language      nameProp     `json:"primaryLanguage"`
	DefaultBranch nameProp     `json:"defaultBranchRef"`
	IsArchived    bool         `json:"isArchived"`
	IsFork        bool         `json:"isFork"`
	Pullrequests  pullrequests `json:"pullRequests"`
	Owner         struct {
		Login string `json:"login"`
	} `json:"owner"`
}

func (g *GithubIntegration) fromRepositoryEvent(logger sdk.Logger, integrationInstanceID string, customerID string, event *github.RepositoryEvent) *sdk.SourceCodeRepo {
	var repo repository
	theRepo := event.GetRepo()
	login := theRepo.Owner.GetLogin()
	var scope sdk.ConfigAccountType
	if theRepo.Owner.GetType() == "Organization" {
		scope = sdk.ConfigAccountTypeOrg
	} else {
		scope = sdk.ConfigAccountTypeUser
	}
	repo.ID = theRepo.GetNodeID()
	repo.Name = theRepo.GetFullName()
	repo.URL = theRepo.GetHTMLURL()
	repo.UpdatedAt = theRepo.UpdatedAt.Time
	repo.Description = theRepo.GetDescription()
	repo.Language = nameProp{theRepo.GetLanguage()}
	repo.DefaultBranch = nameProp{theRepo.GetDefaultBranch()}
	repo.IsArchived = theRepo.GetArchived()
	repo.IsFork = theRepo.GetFork()
	repo.Owner.Login = login
	isPrivate := theRepo.GetPrivate()
	return repo.ToModel(customerID, integrationInstanceID, login, isPrivate, scope)
}

func (r repository) ToModel(customerID string, integrationInstanceID string, login string, isPrivate bool, scope sdk.ConfigAccountType) *sdk.SourceCodeRepo {
	repo := &sdk.SourceCodeRepo{}
	repo.ID = sdk.NewSourceCodeRepoID(customerID, repo.ID, refType)
	repo.CustomerID = customerID
	repo.Name = r.Name
	repo.Description = r.Description
	repo.RefID = r.ID
	repo.RefType = refType
	repo.Language = r.Language.Name
	repo.DefaultBranch = r.DefaultBranch.Name
	repo.URL = r.URL
	repo.UpdatedAt = sdk.TimeToEpoch(r.UpdatedAt)
	repo.Active = !r.IsArchived
	repo.IntegrationInstanceID = sdk.StringPointer(integrationInstanceID)
	if isPrivate {
		repo.Visibility = sdk.SourceCodeRepoVisibilityPrivate
	} else {
		repo.Visibility = sdk.SourceCodeRepoVisibilityPublic
	}
	if r.IsFork || r.Owner.Login != login {
		repo.Affiliation = sdk.SourceCodeRepoAffiliationThirdparty
	} else {
		if scope == sdk.ConfigAccountTypeUser {
			// TODO: need to check the user and determine if they are member of the org using the userManager
			repo.Affiliation = sdk.SourceCodeRepoAffiliationUser
		} else if scope == sdk.ConfigAccountTypeOrg {
			repo.Affiliation = sdk.SourceCodeRepoAffiliationOrganization
		}
	}
	return repo
}
