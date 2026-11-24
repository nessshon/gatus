package api

import (
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"slices"
	"testing"
	"time"

	"github.com/TwiN/gatus/v5/config"
	"github.com/TwiN/gatus/v5/config/endpoint"
	"github.com/TwiN/gatus/v5/storage"
	"github.com/TwiN/gatus/v5/storage/store"
	"github.com/TwiN/gatus/v5/watchdog"
)

var (
	timestamp = time.Now()

	testEndpoint = endpoint.Endpoint{
		Name:                    "name",
		Group:                   "group",
		URL:                     "https://example.org/what/ever",
		Method:                  "GET",
		Body:                    "body",
		Interval:                30 * time.Second,
		Conditions:              []endpoint.Condition{endpoint.Condition("[STATUS] == 200"), endpoint.Condition("[RESPONSE_TIME] < 500"), endpoint.Condition("[CERTIFICATE_EXPIRATION] < 72h")},
		Alerts:                  nil,
		NumberOfFailuresInARow:  0,
		NumberOfSuccessesInARow: 0,
	}
	testSuccessfulResult = endpoint.Result{
		Hostname:              "example.org",
		IP:                    "127.0.0.1",
		HTTPStatus:            200,
		Errors:                nil,
		Connected:             true,
		Success:               true,
		Timestamp:             timestamp,
		Duration:              150 * time.Millisecond,
		CertificateExpiration: 10 * time.Hour,
		ConditionResults: []*endpoint.ConditionResult{
			{
				Condition: "[STATUS] == 200",
				Success:   true,
			},
			{
				Condition: "[RESPONSE_TIME] < 500",
				Success:   true,
			},
			{
				Condition: "[CERTIFICATE_EXPIRATION] < 72h",
				Success:   true,
			},
		},
	}
	testUnsuccessfulResult = endpoint.Result{
		Hostname:              "example.org",
		IP:                    "127.0.0.1",
		HTTPStatus:            200,
		Errors:                []string{"error-1", "error-2"},
		Connected:             true,
		Success:               false,
		Timestamp:             timestamp,
		Duration:              750 * time.Millisecond,
		CertificateExpiration: 10 * time.Hour,
		ConditionResults: []*endpoint.ConditionResult{
			{
				Condition: "[STATUS] == 200",
				Success:   true,
			},
			{
				Condition: "[RESPONSE_TIME] < 500",
				Success:   false,
			},
			{
				Condition: "[CERTIFICATE_EXPIRATION] < 72h",
				Success:   false,
			},
		},
	}
)

func TestEndpointStatus(t *testing.T) {
	defer store.Get().Clear()
	defer cache.Clear()
	cfg := &config.Config{
		Metrics: true,
		Endpoints: []*endpoint.Endpoint{
			{
				Name:  "frontend",
				Group: "core",
			},
			{
				Name:  "backend",
				Group: "core",
			},
		},
		Storage: &storage.Config{
			MaximumNumberOfResults: storage.DefaultMaximumNumberOfResults,
			MaximumNumberOfEvents:  storage.DefaultMaximumNumberOfEvents,
		},
	}
	watchdog.UpdateEndpointStatus(cfg.Endpoints[0], &endpoint.Result{Success: true, Duration: time.Millisecond, Timestamp: time.Now()})
	watchdog.UpdateEndpointStatus(cfg.Endpoints[1], &endpoint.Result{Success: false, Duration: time.Second, Timestamp: time.Now()})
	api := New(cfg)
	router := api.Router()
	type Scenario struct {
		Name         string
		Path         string
		ExpectedCode int
		Gzip         bool
	}
	scenarios := []Scenario{
		{
			Name:         "endpoint-status",
			Path:         "/api/v1/endpoints/core_frontend/statuses",
			ExpectedCode: http.StatusOK,
		},
		{
			Name:         "endpoint-status-gzip",
			Path:         "/api/v1/endpoints/core_frontend/statuses",
			ExpectedCode: http.StatusOK,
			Gzip:         true,
		},
		{
			Name:         "endpoint-status-pagination",
			Path:         "/api/v1/endpoints/core_frontend/statuses?page=1&pageSize=20",
			ExpectedCode: http.StatusOK,
		},
		{
			Name:         "endpoint-status-for-invalid-key",
			Path:         "/api/v1/endpoints/invalid_key/statuses",
			ExpectedCode: http.StatusNotFound,
		},
	}
	for _, scenario := range scenarios {
		t.Run(scenario.Name, func(t *testing.T) {
			request := httptest.NewRequest("GET", scenario.Path, http.NoBody)
			if scenario.Gzip {
				request.Header.Set("Accept-Encoding", "gzip")
			}
			response, err := router.Test(request)
			if err != nil {
				return
			}
			if response.StatusCode != scenario.ExpectedCode {
				t.Errorf("%s %s should have returned %d, but returned %d instead", request.Method, request.URL, scenario.ExpectedCode, response.StatusCode)
			}
		})
	}
}

func TestEndpointStatuses(t *testing.T) {
	defer store.Get().Clear()
	defer cache.Clear()
	firstResult := &testSuccessfulResult
	secondResult := &testUnsuccessfulResult
	store.Get().InsertEndpointResult(&testEndpoint, firstResult)
	store.Get().InsertEndpointResult(&testEndpoint, secondResult)
	// Can't be bothered dealing with timezone issues on the worker that runs the automated tests
	firstResult.Timestamp = time.Time{}
	secondResult.Timestamp = time.Time{}
	api := New(&config.Config{
		Metrics: true,
		Storage: &storage.Config{
			MaximumNumberOfResults: storage.DefaultMaximumNumberOfResults,
			MaximumNumberOfEvents:  storage.DefaultMaximumNumberOfEvents,
		},
	})
	router := api.Router()
	type Scenario struct {
		Name         string
		Path         string
		ExpectedCode int
		ExpectedBody string
	}
	scenarios := []Scenario{
		{
			Name:         "no-pagination",
			Path:         "/api/v1/endpoints/statuses",
			ExpectedCode: http.StatusOK,
			ExpectedBody: `[{"name":"name","group":"group","key":"group_name","results":[{"status":200,"hostname":"example.org","duration":150000000,"conditionResults":[{"condition":"[STATUS] == 200","success":true},{"condition":"[RESPONSE_TIME] \u003c 500","success":true},{"condition":"[CERTIFICATE_EXPIRATION] \u003c 72h","success":true}],"success":true,"timestamp":"0001-01-01T00:00:00Z"},{"status":200,"hostname":"example.org","duration":750000000,"errors":["error-1","error-2"],"conditionResults":[{"condition":"[STATUS] == 200","success":true},{"condition":"[RESPONSE_TIME] \u003c 500","success":false},{"condition":"[CERTIFICATE_EXPIRATION] \u003c 72h","success":false}],"success":false,"timestamp":"0001-01-01T00:00:00Z"}]}]`,
		},
		{
			Name:         "pagination-first-result",
			Path:         "/api/v1/endpoints/statuses?page=1&pageSize=1",
			ExpectedCode: http.StatusOK,
			ExpectedBody: `[{"name":"name","group":"group","key":"group_name","results":[{"status":200,"hostname":"example.org","duration":750000000,"errors":["error-1","error-2"],"conditionResults":[{"condition":"[STATUS] == 200","success":true},{"condition":"[RESPONSE_TIME] \u003c 500","success":false},{"condition":"[CERTIFICATE_EXPIRATION] \u003c 72h","success":false}],"success":false,"timestamp":"0001-01-01T00:00:00Z"}]}]`,
		},
		{
			Name:         "pagination-second-result",
			Path:         "/api/v1/endpoints/statuses?page=2&pageSize=1",
			ExpectedCode: http.StatusOK,
			ExpectedBody: `[{"name":"name","group":"group","key":"group_name","results":[{"status":200,"hostname":"example.org","duration":150000000,"conditionResults":[{"condition":"[STATUS] == 200","success":true},{"condition":"[RESPONSE_TIME] \u003c 500","success":true},{"condition":"[CERTIFICATE_EXPIRATION] \u003c 72h","success":true}],"success":true,"timestamp":"0001-01-01T00:00:00Z"}]}]`,
		},
		{
			Name:         "pagination-no-results",
			Path:         "/api/v1/endpoints/statuses?page=5&pageSize=20",
			ExpectedCode: http.StatusOK,
			ExpectedBody: `[{"name":"name","group":"group","key":"group_name","results":[]}]`,
		},
		{
			Name:         "invalid-pagination-should-fall-back-to-default",
			Path:         "/api/v1/endpoints/statuses?page=INVALID&pageSize=INVALID",
			ExpectedCode: http.StatusOK,
			ExpectedBody: `[{"name":"name","group":"group","key":"group_name","results":[{"status":200,"hostname":"example.org","duration":150000000,"conditionResults":[{"condition":"[STATUS] == 200","success":true},{"condition":"[RESPONSE_TIME] \u003c 500","success":true},{"condition":"[CERTIFICATE_EXPIRATION] \u003c 72h","success":true}],"success":true,"timestamp":"0001-01-01T00:00:00Z"},{"status":200,"hostname":"example.org","duration":750000000,"errors":["error-1","error-2"],"conditionResults":[{"condition":"[STATUS] == 200","success":true},{"condition":"[RESPONSE_TIME] \u003c 500","success":false},{"condition":"[CERTIFICATE_EXPIRATION] \u003c 72h","success":false}],"success":false,"timestamp":"0001-01-01T00:00:00Z"}]}]`,
		},
	}

	for _, scenario := range scenarios {
		t.Run(scenario.Name, func(t *testing.T) {
			request := httptest.NewRequest("GET", scenario.Path, http.NoBody)
			response, err := router.Test(request)
			if err != nil {
				return
			}
			defer response.Body.Close()
			if response.StatusCode != scenario.ExpectedCode {
				t.Errorf("%s %s should have returned %d, but returned %d instead", request.Method, request.URL, scenario.ExpectedCode, response.StatusCode)
			}
			body, err := io.ReadAll(response.Body)
			if err != nil {
				t.Error("expected err to be nil, but was", err)
			}
			if string(body) != scenario.ExpectedBody {
				t.Errorf("expected:\n %s\n\ngot:\n %s", scenario.ExpectedBody, string(body))
			}
		})
	}
}

func TestEndpointStatuses_InConfigOrder(t *testing.T) {
	defer store.Get().Clear()
	defer cache.Clear()
	first := &endpoint.Endpoint{Name: "storage", Group: "ton"}
	second := &endpoint.Endpoint{Name: "frontend", Group: "apps"}
	store.Get().InsertEndpointResult(first, &testSuccessfulResult)
	store.Get().InsertEndpointResult(second, &testSuccessfulResult)
	api := New(&config.Config{
		Endpoints: []*endpoint.Endpoint{first, second},
		Storage: &storage.Config{
			MaximumNumberOfResults: storage.DefaultMaximumNumberOfResults,
			MaximumNumberOfEvents:  storage.DefaultMaximumNumberOfEvents,
		},
	})
	router := api.Router()
	for path, expectedKeys := range map[string][]string{
		"/api/v1/endpoints/statuses":             {"apps_frontend", "ton_storage"},
		"/api/v1/endpoints/statuses?sort=config": {"ton_storage", "apps_frontend"},
	} {
		response, err := router.Test(httptest.NewRequest("GET", path, http.NoBody))
		if err != nil {
			t.Fatal(err)
		}
		var statuses []*endpoint.Status
		if err = json.NewDecoder(response.Body).Decode(&statuses); err != nil {
			t.Fatal(err)
		}
		response.Body.Close()
		var keys []string
		for _, status := range statuses {
			keys = append(keys, status.Key)
		}
		if !slices.Equal(keys, expectedKeys) {
			t.Errorf("%s: expected %v, got %v", path, expectedKeys, keys)
		}
	}
}

func TestSortByConfigOrder(t *testing.T) {
	cfg := &config.Config{
		Endpoints:         []*endpoint.Endpoint{{Name: "storage", Group: "ton"}, {Name: "frontend", Group: "apps"}},
		ExternalEndpoints: []*endpoint.ExternalEndpoint{{Name: "bot", Group: "bots"}},
	}
	statuses := []*endpoint.Status{{Key: "apps_frontend"}, {Key: "remote_one"}, {Key: "bots_bot"}, {Key: "ton_storage"}, {Key: "remote_two"}}
	sortByConfigOrder(cfg, statuses)
	var keys []string
	for _, status := range statuses {
		keys = append(keys, status.Key)
	}
	expected := []string{"ton_storage", "apps_frontend", "bots_bot", "remote_one", "remote_two"}
	if !slices.Equal(keys, expected) {
		t.Errorf("expected %v, got %v", expected, keys)
	}
}
