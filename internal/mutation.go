package internal

import (
	"fmt"

	"github.com/pinpt/agent.next/sdk"
)

// Mutation is called when a mutation request is received on behalf of the integration
func (g *GithubIntegration) Mutation(mutation sdk.Mutation) error {
	sdk.LogInfo(g.logger, "mutation request received", "mutation", mutation)
	switch mutation.Action() {
	case sdk.CreateAction:
		switch v := mutation.Payload().(type) {
		case *sdk.WorkIssue:
			fmt.Println(v)
		}
	}
	return nil
}
