package internal

import (
	"strconv"
	"strings"
	"time"

	"github.com/pinpt/agent.next/sdk"
)

// In GitHub:
// - a repo is a project
// - a project is a kanban board

type repoProjectCardContent struct {
	Type string `json:"__typename"`
	ID   string `json:"id"`
}

type repoProjectCard struct {
	ID      string                 `json:"id"`
	State   string                 `json:"state"`
	Note    string                 `json:"note"`
	Content repoProjectCardContent `json:"content"`
}

type repoProjectCardNodes struct {
	Nodes []repoProjectCard `json:"nodes"`
}

type repoProjectColumn struct {
	ID      string               `json:"id"`
	Name    string               `json:"name"`
	Purpose string               `json:"purpose"`
	Cards   repoProjectCardNodes `json:"cards"`
}

type repoProjectColumnNodes struct {
	Nodes []repoProjectColumn `json:"nodes"`
}

type repoProject struct {
	Name      string                 `json:"name"`
	ID        string                 `json:"id"`
	URL       string                 `json:"url"`
	UpdatedAt time.Time              `json:"updatedAt"`
	Columns   repoProjectColumnNodes `json:"columns"`
}

type repoProjectNode struct {
	Nodes []repoProject `json:"nodes"`
}

type repoProjectsResult struct {
	Repository struct {
		Projects repoProjectNode `json:"projects"`
	} `json:"repository"`
}

type repoProjectResult struct {
	Repository struct {
		Project repoProject `json:"project"`
	} `json:"repository"`
}

func getProjectIDfromURL(url string) int {
	i := strings.LastIndex(url, "/")
	val := url[i:1]
	num, _ := strconv.ParseInt(val, 10, 32)
	return int(num)
}

func (p repoProject) ToModel(logger sdk.Logger, customerID string, integrationInstanceID string, projectID string) *sdk.WorkKanbanBoard {
	var board sdk.WorkKanbanBoard
	board.CustomerID = customerID
	board.IntegrationInstanceID = sdk.StringPointer(integrationInstanceID)
	board.RefType = refType
	board.RefID = p.ID
	board.Name = p.Name
	board.URL = p.URL
	board.ProjectIds = []string{projectID}
	board.UpdatedAt = sdk.TimeToEpoch(p.UpdatedAt)
	board.ID = sdk.NewWorkKanbanBoardID(customerID, board.RefID, refType)
	board.Columns = make([]sdk.WorkKanbanBoardColumns, 0)
	board.IssueIds = make([]string, 0)
	// TODO: should the backlog just be all issues that aren't on the board?
	for _, c := range p.Columns.Nodes {
		var col sdk.WorkKanbanBoardColumns
		col.Name = c.Name
		col.IssueIds = make([]string, 0)
		for _, o := range c.Cards.Nodes {
			id := sdk.NewWorkIssueID(customerID, o.Content.ID, refType)
			col.IssueIds = append(col.IssueIds, id)
			board.IssueIds = append(board.IssueIds, id)
		}
		board.Columns = append(board.Columns, col)
	}
	return &board
}

func (r repository) ToProjectModel(repo *sdk.SourceCodeRepo) *sdk.WorkProject {
	if !r.HasIssues {
		return nil
	}
	var project sdk.WorkProject
	project.ID = sdk.NewWorkProjectID(repo.CustomerID, repo.RefID, refType)
	project.RefType = refType
	project.RefID = repo.RefID
	project.URL = repo.URL
	project.Name = repo.Name
	project.Description = sdk.StringPointer(repo.Description)
	project.Identifier = repo.Name
	project.Visibility = sdk.WorkProjectVisibility(repo.Visibility)
	project.Affiliation = sdk.WorkProjectAffiliation(repo.Affiliation)
	project.IntegrationInstanceID = repo.IntegrationInstanceID
	project.CustomerID = repo.CustomerID
	project.UpdatedAt = repo.UpdatedAt
	return &project
}
