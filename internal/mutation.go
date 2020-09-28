package internal

import "github.com/pinpt/agent/sdk"

// Mutation is called when a mutation request is received on behalf of the integration
func (g *GithubIntegration) Mutation(mutation sdk.Mutation) error {
	sdk.LogInfo(g.logger, "mutation request received", "action", mutation.Action(), "id", mutation.ID(), "customer_id", mutation.CustomerID(), "model", mutation.Model())
	switch mutation.Action() {
	case sdk.CreateAction:
		break
	case sdk.UpdateAction:
		switch v := mutation.Payload().(type) {
		case *sdk.SourcecodePullRequestUpdateMutation:
			return g.updatePullrequest(g.logger, mutation.Config(), mutation.ID(), v, mutation.User())
		}
	case sdk.DeleteAction:
		break
	}
	return nil
}
