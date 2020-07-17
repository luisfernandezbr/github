module github.com/pinpt/agent.next.github

go 1.14

require (
	github.com/JohannesKaufmann/html-to-markdown v0.0.0-20200716201554-b46a9fbd5b11 // indirect
	github.com/go-redis/redis/v8 v8.0.0-beta.6 // indirect
	github.com/google/go-github/v32 v32.0.0
	github.com/mailru/easyjson v0.7.1
	github.com/pinpt/agent.next v0.0.0-20200717211020-a80e2326049f
	github.com/pinpt/go-common/v10 v10.0.16
	github.com/stretchr/testify v1.6.1
	go.opentelemetry.io/otel v0.8.0 // indirect
	golang.org/x/net v0.0.0-20200707034311-ab3426394381 // indirect
	golang.org/x/text v0.3.3 // indirect
)

replace github.com/pinpt/agent.next => ../agent.next
