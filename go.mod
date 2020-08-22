module github.com/pinpt/agent.next.github

go 1.14

require (
	github.com/google/go-github/v32 v32.0.0
	github.com/mailru/easyjson v0.7.1
	github.com/pinpt/agent.next v0.0.0-20200719023419-46571aef39f6
	go.opentelemetry.io/otel v0.8.0 // indirect
	golang.org/x/net v0.0.0-20200707034311-ab3426394381 // indirect
)

replace github.com/pinpt/agent.next => ../agent.next
