package internal

import "github.com/pinpt/agent/v4/sdk"

// Mutation is called when a mutation request is received on behalf of the integration
func (g *GithubIntegration) Mutation(mutation sdk.Mutation) (*sdk.MutationResponse, error) {
	logger := sdk.LogWith(g.logger, "customer_id", mutation.CustomerID(), "integration_instance_id", mutation.IntegrationInstanceID())
	sdk.LogInfo(logger, "mutation request received", "action", mutation.Action(), "id", mutation.ID(), "customer_id", mutation.CustomerID(), "model", mutation.Model())
	switch mutation.Action() {
	case sdk.CreateAction:
		break
	case sdk.UpdateAction:
		switch v := mutation.Payload().(type) {
		case *sdk.SourcecodePullRequestUpdateMutation:
			return nil, g.updatePullrequest(logger, mutation.Config(), mutation.ID(), v, mutation.User())
		}
	case sdk.DeleteAction:
		break
	}
	return nil, nil
}
