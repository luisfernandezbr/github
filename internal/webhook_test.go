package internal

import (
	"io/ioutil"
	"reflect"
	"testing"
	"time"

	"github.com/pinpt/agent.next/sdk"
	"github.com/stretchr/testify/assert"
)

type testPipe struct {
	objects []sdk.Model
	closed  bool
	flushed bool
}

var _ sdk.Pipe = (*testPipe)(nil)

// Write a model back to the output system
func (p *testPipe) Write(object sdk.Model) error {
	if p.objects == nil {
		p.objects = make([]sdk.Model, 0)
	}
	p.objects = append(p.objects, object)
	return nil
}

// Flush will tell the pipe to flush any pending data
func (p *testPipe) Flush() error {
	p.flushed = true
	return nil
}

// Close is called when the integration has completed and no more data will be sent
func (p *testPipe) Close() error {
	p.closed = true
	return nil
}

type testWebhook struct {
	data    []byte
	headers map[string]string
	pipe    testPipe
}

var _ sdk.WebHook = (*testWebhook)(nil)

// Config is any customer specific configuration for this customer
func (w *testWebhook) Config() sdk.Config {
	return sdk.Config{}
}

// State is a customer specific state object for this integration and customer
func (w *testWebhook) State() sdk.State {
	return nil
}

// CustomerID will return the customer id for the web hook
func (w *testWebhook) CustomerID() string {
	return "1234"
}

// IntegrationInstanceID will return the unique instance id for this integration for a customer
func (w *testWebhook) IntegrationInstanceID() string {
	return "999"
}

// RefID will return the ref id from when the hook was created
func (w *testWebhook) RefID() string {
	return "0"
}

// Pipe returns a pipe for sending data back to pinpoint from the web hook data
func (w *testWebhook) Pipe() sdk.Pipe {
	return &w.pipe
}

// Data is the data payload for the web hook
func (w *testWebhook) Data() (map[string]interface{}, error) {
	return nil, nil
}

// Bytes will return the underlying data as bytes
func (w *testWebhook) Bytes() []byte {
	return w.data
}

// Headers are the headers that came from the web hook
func (w *testWebhook) Headers() map[string]string {
	return w.headers
}

// Paused must be called when the integration is paused for any reason such as rate limiting
func (w *testWebhook) Paused(resetAt time.Time) error {
	return nil
}

// Resumed must be called when a paused integration is resumed
func (w *testWebhook) Resumed() error {
	return nil
}

// Scope is the registered webhook scope
func (w *testWebhook) Scope() sdk.WebHookScope {
	return sdk.WebHookScopeOrg
}

type testGraphqlClient struct {
	res interface{}
}

var _ sdk.GraphQLClient = (*testGraphqlClient)(nil)

func (c *testGraphqlClient) Query(query string, variables map[string]interface{}, out interface{}, options ...sdk.WithGraphQLOption) error {
	if c.res != nil {
		reflect.ValueOf(out).Elem().Set(reflect.ValueOf(c.res))
	}
	return nil
}

func TestWebhookPullRequestCreated(t *testing.T) {
	assert := assert.New(t)
	buf, err := ioutil.ReadFile("./data/pullrequest_created.json")
	assert.NoError(err)
	webhook := &testWebhook{
		data:    buf,
		headers: map[string]string{"x-github-event": "pull_request"},
	}
	c := &testGraphqlClient{}
	g := &GithubIntegration{
		logger:     sdk.NewNoOpTestLogger(),
		testClient: c,
	}
	var commits pullrequestPagedCommitsResult
	commits.Node = pullrequestPagedCommits{
		Commits: pullrequestPagedCommitEdges{
			Edges: []pullrequestPagedCommitNode{
				{
					Node: pullrequestPagedCommit{
						Commit: pullrequestCommit{
							Sha: "sha1",
						},
					},
				},
			},
		},
	}
	c.res = commits
	assert.NoError(g.WebHook(webhook))
	assert.NotEmpty(webhook.pipe.objects)
	assert.NotEmpty(webhook.pipe.objects[0])
}
