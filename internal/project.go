package internal

import (
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/pinpt/agent/sdk"
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

func getProjectIDfromURL(url string) (int, error) {
	i := strings.LastIndex(url, "/")
	if i < 0 {
		return 0, fmt.Errorf("invalid project url: %s", url)
	}
	if i == len(url)-1 {
		return 0, fmt.Errorf("url was missing project id at end: %s", url)
	}
	val := url[i+1:]
	num, err := strconv.ParseInt(val, 10, 32)
	if err != nil {
		return 0, err
	}
	return int(num), nil
}

func (p repoProject) ToModel(logger sdk.Logger, customerID string, integrationInstanceID string, projectID string) (*sdk.AgileBoard, *sdk.AgileKanban) {
	var board sdk.AgileBoard
	board.CustomerID = customerID
	board.IntegrationInstanceID = sdk.StringPointer(integrationInstanceID)
	board.RefType = refType
	board.RefID = p.ID
	board.Name = p.Name
	board.Active = true
	board.UpdatedAt = sdk.TimeToEpoch(p.UpdatedAt)
	board.ID = sdk.NewAgileBoardID(customerID, p.ID, refType)
	board.Columns = make([]sdk.AgileBoardColumns, 0)
	board.Type = sdk.AgileBoardTypeKanban

	var kanban sdk.AgileKanban
	kanban.CustomerID = customerID
	kanban.IntegrationInstanceID = sdk.StringPointer(integrationInstanceID)
	kanban.RefType = refType
	kanban.RefID = p.ID
	kanban.Name = p.Name
	kanban.Active = true
	kanban.URL = sdk.StringPointer(p.URL)
	kanban.BoardID = board.ID
	kanban.UpdatedAt = sdk.TimeToEpoch(p.UpdatedAt)
	kanban.ID = sdk.NewAgileKanbanID(customerID, p.ID, refType)
	kanban.Columns = make([]sdk.AgileKanbanColumns, 0)
	kanban.IssueIds = make([]string, 0)
	kanban.ProjectIds = []string{projectID}

	for _, c := range p.Columns.Nodes {
		var col sdk.AgileKanbanColumns
		col.Name = c.Name
		col.IssueIds = make([]string, 0)
		for _, o := range c.Cards.Nodes {
			id := sdk.NewWorkIssueID(customerID, o.Content.ID, refType)
			col.IssueIds = append(col.IssueIds, id)
			kanban.IssueIds = append(kanban.IssueIds, id)
		}
		kanban.Columns = append(kanban.Columns, col)
		var bcol sdk.AgileBoardColumns
		bcol.Name = c.Name
		board.Columns = append(board.Columns, bcol)
	}
	return &board, &kanban
}

func (r repository) ToProjectModel(repo *sdk.SourceCodeRepo) *sdk.WorkProject {
	if !r.HasIssues {
		return nil
	}
	var project sdk.WorkProject
	project.Active = true
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
	project.IssueTypes = []sdk.WorkProjectIssueTypes{
		{
			Name:  "Epic",
			RefID: "epic",
		},
		{
			Name:  "Task",
			RefID: "task",
		},
		{
			Name:  "Bug",
			RefID: "bug",
		},
		{
			Name:  "Enhancement",
			RefID: "enhancement",
		},
	}
	return &project
}

const projectCapabilityCacheKeyPrefix = "project_capability_"

func (r repository) ToProjectCapabilityModel(state sdk.State, repo *sdk.SourceCodeRepo, historical bool) *sdk.WorkProjectCapability {
	if !r.HasIssues {
		return nil
	}
	var cacheKey = projectCapabilityCacheKeyPrefix + repo.ID
	if !historical && state.Exists(cacheKey) {
		return nil
	}
	var capability sdk.WorkProjectCapability
	capability.CustomerID = repo.CustomerID
	capability.RefID = repo.RefID
	capability.RefType = repo.RefType
	capability.IntegrationInstanceID = repo.IntegrationInstanceID
	capability.ProjectID = sdk.NewWorkProjectID(repo.CustomerID, repo.RefID, refType)
	capability.UpdatedAt = repo.UpdatedAt
	capability.Attachments = false
	capability.ChangeLogs = false
	capability.DueDates = false
	capability.Epics = true
	capability.InProgressStates = false
	capability.KanbanBoards = r.HasProjects
	capability.LinkedIssues = true
	capability.Parents = true
	capability.Priorities = false
	capability.Resolutions = false
	capability.Sprints = false
	capability.StoryPoints = false
	state.SetWithExpires(cacheKey, 1, time.Hour*24*30)
	return &capability
}
