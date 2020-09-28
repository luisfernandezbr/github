package internal

import (
	"time"

	"github.com/google/go-github/v32/github"
	"github.com/pinpt/agent/sdk"
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
	HasProjects   bool         `json:"hasProjectsEnabled"`
	HasIssues     bool         `json:"hasIssuesEnabled"`
	Labels        labelNode    `json:"labels"`
	Pullrequests  pullrequests `json:"pullRequests"`
	Owner         struct {
		Login string `json:"login"`
	} `json:"owner"`
}

func (g *GithubIntegration) fromRepositoryEvent(logger sdk.Logger, state sdk.State, integrationInstanceID string, customerID string, event *github.RepositoryEvent) (*sdk.SourceCodeRepo, *sdk.WorkProject, *sdk.WorkProjectCapability) {
	var repo repository
	theRepo := event.GetRepo()
	login := getRepoOwnerLogin(theRepo)
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
	repo.HasIssues = theRepo.GetHasIssues()
	repo.HasProjects = theRepo.GetHasProjects()
	repo.Owner.Login = login
	isPrivate := theRepo.GetPrivate()
	return repo.ToModel(state, false, customerID, integrationInstanceID, login, isPrivate, scope)
}

func (r repository) ToModel(state sdk.State, historical bool, customerID string, integrationInstanceID string, login string, isPrivate bool, scope sdk.ConfigAccountType) (*sdk.SourceCodeRepo, *sdk.WorkProject, *sdk.WorkProjectCapability) {
	repo := &sdk.SourceCodeRepo{}
	repo.ID = sdk.NewSourceCodeRepoID(customerID, r.ID, refType)
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
			repo.Affiliation = sdk.SourceCodeRepoAffiliationUser
		} else if scope == sdk.ConfigAccountTypeOrg {
			repo.Affiliation = sdk.SourceCodeRepoAffiliationOrganization
		}
	}

	// since a repo can also possibly be a work project, try and create it too
	return repo, r.ToProjectModel(repo), r.ToProjectCapabilityModel(state, repo, historical)
}

func getRepoOwnerLogin(repo *github.Repository) string {
	if repo.Organization != nil {
		return repo.Organization.GetLogin()
	}
	return repo.Owner.GetLogin()
}

func getPushRepoOwnerLogin(repo *github.PushEventRepository) string {
	if repo.Organization != nil {
		return repo.GetOrganization()
	}
	return repo.Owner.GetLogin()
}
