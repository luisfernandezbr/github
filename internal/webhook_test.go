package internal

import (
	"crypto/hmac"
	"crypto/sha1"
	"encoding/json"
	"fmt"
	"io"
	"io/ioutil"
	"net/http"
	"net/url"
	"strings"
	"testing"

	"github.com/pinpt/agent/v4/sdk"
	"github.com/stretchr/testify/assert"
)

type mockWebhookManager struct {
	URL                       string
	isPinpointWebhookCallback func(url string) bool
}

var _ sdk.WebHookManager = (*mockWebhookManager)(nil)

func (m *mockWebhookManager) Create(customerID string, integrationInstanceID string, refType string, refID string, scope sdk.WebHookScope, params ...string) (string, error) {
	return m.URL, nil
}
func (m *mockWebhookManager) Delete(customerID string, integrationInstanceID string, refType string, refID string, scope sdk.WebHookScope) error {
	return nil
}
func (m *mockWebhookManager) Exists(customerID string, integrationInstanceID string, refType string, refID string, scope sdk.WebHookScope) bool {
	return false
}
func (m *mockWebhookManager) Errored(customerID string, integrationInstanceID string, refType string, refID string, scope sdk.WebHookScope, err error) {
}
func (m *mockWebhookManager) HookURL(customerID string, integrationInstanceID string, refType string, refID string, scope sdk.WebHookScope) (string, error) {
	return m.URL, nil
}
func (m *mockWebhookManager) Secret() string { return "pinpoint" }
func (m *mockWebhookManager) CreateSharedWebhook(customerID string, integrationInstanceID string, refType string, refID string, scope sdk.WebHookScope) (string, error) {
	return m.URL, nil
}
func (m *mockWebhookManager) IsPinpointWebhook(url string) bool {
	if m.isPinpointWebhookCallback != nil {
		return m.isPinpointWebhookCallback(url)
	}
	return false
}

type ClientCallback func(method string, data io.Reader, out interface{}, options ...sdk.WithHTTPOption) (*sdk.HTTPResponse, error)

type MockHTTPClient struct {
	Callback ClientCallback
}

func (c *MockHTTPClient) Get(out interface{}, options ...sdk.WithHTTPOption) (*sdk.HTTPResponse, error) {
	return c.Callback(http.MethodGet, nil, out, options...)
}
func (c *MockHTTPClient) Post(data io.Reader, out interface{}, options ...sdk.WithHTTPOption) (*sdk.HTTPResponse, error) {
	return c.Callback(http.MethodPost, data, out, options...)
}
func (c *MockHTTPClient) Put(data io.Reader, out interface{}, options ...sdk.WithHTTPOption) (*sdk.HTTPResponse, error) {
	return c.Callback(http.MethodPut, data, out, options...)
}
func (c *MockHTTPClient) Patch(data io.Reader, out interface{}, options ...sdk.WithHTTPOption) (*sdk.HTTPResponse, error) {
	return c.Callback(http.MethodPatch, data, out, options...)
}
func (c *MockHTTPClient) Delete(out interface{}, options ...sdk.WithHTTPOption) (*sdk.HTTPResponse, error) {
	return c.Callback(http.MethodDelete, nil, out, options...)
}

func returnJSONFromFile(filename string, withStatus int, toOut interface{}) (*sdk.HTTPResponse, error) {
	buf, err := ioutil.ReadFile(filename)
	if err != nil {
		panic(err)
	}
	if err := json.Unmarshal(buf, toOut); err != nil {
		return nil, err
	}
	return &sdk.HTTPResponse{
		StatusCode: withStatus,
	}, nil
}

// getEndpoint will parse out the url that would be set in the request
func getEndpoint(options []sdk.WithHTTPOption) string {
	opts := sdk.HTTPOptions{
		Request: &http.Request{
			URL: &url.URL{},
		},
	}
	for _, opt := range options {
		opt(&opts)
	}
	return opts.Request.URL.Path
}

func TestInstallRepoWebhookIfRequiredMigrateToNew(t *testing.T) {
	assert := assert.New(t)
	g := GithubIntegration{}
	logger := sdk.NewNoOpTestLogger()
	manager := &mockWebhookManager{
		isPinpointWebhookCallback: func(url string) bool {
			return strings.Contains(url, "event.api")
		},
		URL: "https://testURL.com",
	}
	client := &MockHTTPClient{}
	customerID := "abc"
	integrationInstanceID := "1"
	login := "pinpt"
	repoName := "pinpt/pipeline"
	repoRefID := "base64thing=="
	var listedWebhooks bool
	var deletedWebhooks []string
	var createdWebhook bool
	client.Callback = func(method string, data io.Reader, out interface{}, options ...sdk.WithHTTPOption) (*sdk.HTTPResponse, error) {
		if method == http.MethodGet {
			path := getEndpoint(options)
			assert.EqualValues("/repos/pinpt/pipeline/hooks", path)
			listedWebhooks = true
			return returnJSONFromFile("testdata/list_repo_webhooks_legacy.json", http.StatusOK, out)
		}
		if method == http.MethodDelete {
			path := getEndpoint(options)
			sl := strings.Split(path, "/")
			id := sl[len(sl)-1]
			deletedWebhooks = append(deletedWebhooks, id)
			return &sdk.HTTPResponse{
				StatusCode: http.StatusNoContent,
			}, nil
		}
		if method == http.MethodPost {
			path := getEndpoint(options)
			assert.EqualValues("/repos/pinpt/pipeline/hooks", path)
			buf, _ := ioutil.ReadAll(data)
			assert.EqualValues(fmt.Sprintf("{\"active\":true,\"config\":{\"content_type\":\"json\",\"insecure_ssl\":\"0\",\"secret\":\"pinpoint\",\"url\":\"https://testURL.com?version=%s\"},\"events\":[\"push\",\"pull_request\",\"commit_comment\",\"issue_comment\",\"issues\",\"project_card\",\"project_column\",\"project\",\"pull_request_review\",\"pull_request_review_comment\",\"repository\",\"milestone\"],\"name\":\"web\"}", hookVersion), string(buf))
			createdWebhook = true
			return returnJSONFromFile("testdata/create_repo_webhook_response.json", http.StatusCreated, out)
		}
		return nil, fmt.Errorf("error unhandled request")
	}
	installed, err := g.installRepoWebhookIfRequired(manager, logger, client, customerID, integrationInstanceID, login, repoName, repoRefID)
	assert.NoError(err)
	assert.True(installed)
	assert.True(listedWebhooks)
	assert.True(createdWebhook)
	assert.Len(deletedWebhooks, 2)
	assert.Contains(deletedWebhooks, "228888199")
	assert.Contains(deletedWebhooks, "232672340")
}

func TestVerifyWebhookSignature(t *testing.T) {
	assert := assert.New(t)
	secret := []byte("aaaaa")
	body := []byte("{\"a\":\"b\"")
	mac := hmac.New(sha1.New, secret)
	sig := mac.Sum(body)
	assert.True(verifyWebhookSignature(string(sig), string(secret), body))
}
