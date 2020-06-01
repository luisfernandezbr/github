package internal

import (
	"time"

	"github.com/pinpt/agent.next/sdk"
)

type repositoryNode struct {
	Cursor string
	Node   repository
}

type repositories struct {
	TotalCount int
	PageInfo   pageInfo
	Edges      []repositoryNode
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

func (r repository) ToModel(customerID string) *sdk.SourceCodeRepo {
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
	return repo
}
