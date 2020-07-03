module github.com/pinpt/agent.next.github

go 1.14

require (
	github.com/dgryski/go-rendezvous v0.0.0-20200624174652-8d2f3be8b2d9 // indirect
	github.com/google/go-github/v32 v32.0.0
	github.com/mailru/easyjson v0.7.1
	github.com/mattn/go-colorable v0.1.7 // indirect
	github.com/pinpt/agent.next v0.0.0-20200703224529-1fbcbb17e940
	github.com/pinpt/go-common/v10 v10.0.14
	github.com/pinpt/httpclient v0.0.0-20200627153820-d374c2f15648 // indirect
	github.com/stretchr/testify v1.6.1
	go.opentelemetry.io/otel v0.7.0 // indirect
	golang.org/x/text v0.3.3 // indirect
	google.golang.org/grpc v1.30.0 // indirect
)

replace github.com/pinpt/agent.next => ../agent.next
