package main

import (
	"github.com/pinpt/agent.next.github/internal"
	"github.com/pinpt/agent.next/runner"
)

// Integration is used to export the integration
var Integration internal.GithubIntegration

func main() {
	runner.Main(&Integration)
}
