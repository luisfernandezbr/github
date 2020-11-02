package main

import (
	"github.com/pinpt/agent/v4/runner"
	"github.com/pinpt/github/internal"
)

// Integration is used to export the integration
var Integration internal.GithubIntegration

func main() {
	runner.Main(&Integration)
}
