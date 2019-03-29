package jobrunner

import (
	"bytes"
	"fmt"
	"io/ioutil"
	"net/http"
	"testing"

	cl "github.com/automationcloud/client-go"
)

func TestJobRun(t *testing.T) {
	t.Run("happy case", func(t *testing.T) {
		requestsMade := make(map[string]*http.Request)
		responses := map[string]string{
			"POST http://jib":      `{"url":"http://ubio.air/"}`,
			"POST http://api/jobs": `{"id": "job-id"}`,
			"GET https://protocol.automationcloud.net/schema.json": `{}`,
		}
		client := newTestClient(func(req *http.Request) *http.Response {
			// Test request parameters
			request := req.Method + " " + req.URL.String()
			requestsMade[request] = req
			response, ok := responses[request]
			if !ok {
				panic("undeclared request: " + request)
			}
			return &http.Response{
				StatusCode: 200,
				Body:       ioutil.NopCloser(bytes.NewBufferString(response)),
				Header:     make(http.Header),
			}
		})

		jr := NewRunner(client, "apikey", "http://api", "http://jib")
		job, err := jr.RunJob(JobRun{ServiceId: "service-id", HowMany: 1, OversupplyInputs: true})
		if err != nil {
			t.Errorf("unexpected error %v", err)
		}

		jobCreationRequestBody := fmt.Sprint(requestsMade["POST http://api/jobs"].Body)
		expectedJobCreationRequestBody := `{{"serviceId":"service-id","input":{"url":"http://ubio.air/"}}}`
		if jobCreationRequestBody != expectedJobCreationRequestBody {
			t.Errorf("Expected request body to be %v, got %v", expectedJobCreationRequestBody, jobCreationRequestBody)
		}

		// fmt.Println(job, jr.Job)
		if job != *jr.Job {
			t.Error("expected job to be stored in jobrunner")
		}
	})

	t.Run("data generation failed", func(t *testing.T) {
		client := newTestClient(func(req *http.Request) *http.Response {
			return &http.Response{
				StatusCode: 500,
				Body:       ioutil.NopCloser(bytes.NewBufferString("Server error")),
				Header:     make(http.Header),
			}
		})
		jr := NewRunner(client, "apikey", "http://api", "http://jib")
		_, err := jr.RunJob(JobRun{})

		if err == nil {
			t.Error("expected error")
		}

		expectedError := "data generation failed"
		if err.Error() != expectedError {
			t.Errorf("expected error to be %v, got %v", expectedError, err)
		}
	})

	t.Run("job creation failed", func(t *testing.T) {
		client := newTestClient(func(req *http.Request) *http.Response {
			var status int
			var body string
			switch req.URL.String() {
			case "http://jib":
				status = 200
				body = "{}"
			default:
				status = 500
				body = "Server error"
			}
			return &http.Response{
				StatusCode: status,
				Body:       ioutil.NopCloser(bytes.NewBufferString(body)),
				Header:     make(http.Header),
			}
		})
		jr := NewRunner(client, "apikey", "http://api", "http://jib")
		_, err := jr.RunJob(JobRun{})

		if err == nil {
			t.Error("expected error")
			t.FailNow()
		}

		expectedError := "server error"
		if err.Error() != expectedError {
			t.Errorf("expected error to be %v, got %v", expectedError, err)
		}
	})
}

func TestMakeCallbackUrl(t *testing.T) {
	if makeCallbackUrl("url", "domain") != "url?domainId=domain" {
		t.Error("it should add domain to non-empty url")
	}

	if makeCallbackUrl("", "domain") != "" {
		t.Error("it should not add domain to empty url")
	}
}

func TestCreateInput(t *testing.T) {
	t.Run("input derived from output", func(t *testing.T) {
		requestsMade := make(map[string]*http.Request)
		responses := map[string]string{
			"GET http://api/jobs/job-id": `{
				"id": "job-id",
				"state": "awaitingInput",
				"awaitingInputKey": "finalPriceConsent"
			}`,
			"GET http://api/jobs/job-id/outputs/finalPrice": `{"data": 13}`,
			"POST http://api/jobs/job-id/inputs":            `{"id": "input-id"}`,
			"GET https://protocol.automationcloud.net/schema.json": `{
			"domains": {
				"A": {
					"inputs": {
						"finalPriceConsent": {
							"inputMethod": "Consent",
							"sourceOutputKey": "finalPrice"
						}
					}
				}
			}}`,
		}
		client := newTestClient(func(req *http.Request) *http.Response {
			// Test request parameters
			request := req.Method + " " + req.URL.String()
			requestsMade[request] = req
			response, ok := responses[request]
			if !ok {
				panic("undeclared request: " + request)
			}
			// fmt.Println(request, response)
			return &http.Response{
				StatusCode: 200,
				Body:       ioutil.NopCloser(bytes.NewBufferString(response)),
				Header:     make(http.Header),
			}
		})

		jr := NewRunner(client, "apikey", "http://api", "http://jib")
		err := jr.ResumeJob("job-id", "A")
		if err != nil {
			t.Error(err)
			t.FailNow()
		}
		jr.CreateInput()
		inputCreationRequestBody, _ := ioutil.ReadAll(requestsMade["POST http://api/jobs/job-id/inputs"].Body)
		expectedInputCreationRequestBody := `{"key":"finalPriceConsent","data":13}`
		if string(inputCreationRequestBody) != expectedInputCreationRequestBody {
			t.Errorf("expected input creation request body to be %v, got %v", expectedInputCreationRequestBody, inputCreationRequestBody)
		}
	})

	t.Run("stashed input", func(t *testing.T) {
		requestsMade := make(map[string]*http.Request)
		responses := map[string]string{
			"GET http://api/jobs/job-id": `{
				"id": "job-id",
				"state": "awaitingInput",
				"awaitingInputKey": "finalPriceConsent"
			}`,
			"POST http://api/jobs/job-id/inputs":                   `{"id": "input-id"}`,
			"GET https://protocol.automationcloud.net/schema.json": `{"domains": {"A":{}}}`,
		}
		client := newTestClient(func(req *http.Request) *http.Response {
			// Test request parameters
			request := req.Method + " " + req.URL.String()
			requestsMade[request] = req
			response, ok := responses[request]
			if !ok {
				panic("undeclared request: " + request)
			}
			// fmt.Println(request, response)
			return &http.Response{
				StatusCode: 200,
				Body:       ioutil.NopCloser(bytes.NewBufferString(response)),
				Header:     make(http.Header),
			}
		})

		jr := &JobRunner{
			httpClient: client,
			JibUrl:     "http://jib",
			apiClient:  cl.NewApiClient(client, "apiKey").WithBaseURL("http://api"),
			InputData: map[string]interface{}{
				"finalPriceConsent": 13,
			},
		}
		jr.ResumeJob("job-id", "A")
		err := jr.ResumeJob("job-id", "A")
		if err != nil {
			t.Error(err)
			t.FailNow()
		}
		jr.CreateInput()
		inputCreationRequestBody, _ := ioutil.ReadAll(requestsMade["POST http://api/jobs/job-id/inputs"].Body)
		expectedInputCreationRequestBody := `{"key":"finalPriceConsent","data":13}`
		if string(inputCreationRequestBody) != expectedInputCreationRequestBody {
			t.Errorf("expected input creation request body to be %v, got %v", expectedInputCreationRequestBody, inputCreationRequestBody)
		}
	})

	t.Run("job runner not ready", func(t *testing.T) {
		jr := &JobRunner{}
		err := jr.CreateInput()
		expectError(t, "job runner is not ready to create input: no job created or resumed", err)
	})
}

// rtf .
type rtf func(req *http.Request) *http.Response

// roundTrip .
func (f rtf) RoundTrip(req *http.Request) (*http.Response, error) {
	return f(req), nil
}

//NewTestClient returns *http.Client with Transport replaced to avoid making real calls
func newTestClient(fn rtf) *http.Client {
	return &http.Client{
		Transport: rtf(fn),
	}
}
