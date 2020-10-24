package internal

import (
	"github.com/pinpt/agent/v4/sdk"
)

// Mutation is called when a mutation request is received on behalf of the integration
func (g *GithubIntegration) Mutation(mutation sdk.Mutation) (*sdk.MutationResponse, error) {
	logger := mutation.Logger()
	sdk.LogInfo(logger, "mutation request received", "action", mutation.Action(), "id", mutation.ID(), "model", mutation.Model())
	userManager := NewUserManager(mutation.CustomerID(), []string{""}, mutation, mutation.State(), mutation.Pipe(), g, mutation.IntegrationInstanceID(), false)
	switch mutation.Action() {
	case sdk.CreateAction:
		switch v := mutation.Payload().(type) {
		case *sdk.WorkIssueCreateMutation:
			switch *v.Type.Name {
			case "Bug":
				return g.createIssue(logger, userManager, v, mutation.User())
			}
		}
		break
	case sdk.UpdateAction:
		switch v := mutation.Payload().(type) {
		case *sdk.SourcecodePullRequestUpdateMutation:
			return nil, g.updatePullrequest(logger, mutation.Config(), mutation.ID(), v, mutation.User())
		case *sdk.WorkIssueUpdateMutation:
			return g.UpdateIssue(logger, userManager, mutation.ID(), v, mutation.User())
		}
	case sdk.DeleteAction:
		break
	}
	return nil, nil
}
