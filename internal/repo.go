package internal

import (
	"time"

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

func (r repository) ToModel(customerID string, integrationID string, login string, isPrivate bool, scope accountType) *sdk.SourceCodeRepo {
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
	repo.IntegrationInstanceID = sdk.StringPointer(integrationID)
	if isPrivate {
		repo.Visibility = sdk.SourceCodeRepoVisibilityPrivate
	} else {
		repo.Visibility = sdk.SourceCodeRepoVisibilityPublic
	}
	if r.IsFork || r.Owner.Login != login {
		repo.Affiliation = sdk.SourceCodeRepoAffiliationThirdparty
	} else {
		if scope == userAccountType {
			repo.Affiliation = sdk.SourceCodeRepoAffiliationUser
		} else if scope == orgAccountType {
			repo.Affiliation = sdk.SourceCodeRepoAffiliationOrganization
		}
	}
	return repo
}
