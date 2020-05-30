package internal

import (
	"time"

	"github.com/pinpt/go-common/datetime"
	"github.com/pinpt/integration-sdk/sourcecode"
)

type repositories struct {
	TotalCount int
	PageInfo   pageInfo
	Nodes      []repository
}

type repository struct {
	ID            string       `json:"id"`
	Name          string       `json:"nameWithOwner"`
	URL           string       `json:"url"`
	UpdatedAt     time.Time    `json:"updatedAt"`
	Description   string       `json:"description"`
	Language      nameProp     `json:"primaryLanguage"`
	DefaultBranch nameProp     `json:"defaultBranchRef"`
	IsArchived    bool         `json:"isArchived"`
	Pullrequests  pullrequests `json:"pullRequests"`
}

func (r repository) ToModel(customerID string) *sourcecode.Repo {
	repo := &sourcecode.Repo{}
	repo.ID = sourcecode.NewRepoID(customerID, refType, repo.ID)
	repo.CustomerID = customerID
	repo.Name = r.Name
	repo.Description = r.Description
	repo.RefID = r.ID
	repo.RefType = refType
	repo.Language = r.Language.Name
	repo.DefaultBranch = r.DefaultBranch.Name
	repo.URL = r.URL
	repo.UpdatedAt = datetime.TimeToEpoch(r.UpdatedAt)
	repo.Active = !r.IsArchived
	return repo
}
