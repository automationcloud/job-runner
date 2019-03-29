package jobrunner

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestDataGeneration(t *testing.T) {
	t.Run("happy case", func(t *testing.T) {
		ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
			fmt.Fprint(w, `{"hello": "world"}`)
		}))
		defer ts.Close()
		data, err := GenerateData(ts.URL, make(map[string]interface{}), &http.Client{})
		if err != nil {
			t.Errorf("unexpected error: %v", err)
		}

		if data["hello"] != "world" {
			t.Errorf("Expected response {hello:'world'}, got %v", data)
		}
	})

	t.Run("invalid json body", func(t *testing.T) {
		ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
			fmt.Fprint(w, `{`)
		}))
		defer ts.Close()
		_, err := GenerateData(ts.URL, make(map[string]interface{}), &http.Client{})
		expectError(t, "unexpected end of JSON input", err)
	})

	t.Run("bad config", func(t *testing.T) {
		_, err := GenerateData("", map[string]interface{}{"": make(chan byte, 1)}, nil)
		expectError(t, "json: unsupported type: chan uint8", err)
	})

	t.Run("bad url", func(t *testing.T) {
		_, err := GenerateData("http://\t", make(map[string]interface{}), &http.Client{})
		expectError(t, "parse http://	: net/url: invalid control character in URL", err)
	})

	t.Run("bad request", func(t *testing.T) {
		_, err := GenerateData("http://", make(map[string]interface{}), &http.Client{})
		expectError(t, "Post http:: http: no Host in request URL", err)
	})
}

func expectError(t *testing.T, expectedError string, err error) {
	if err == nil {
		t.Errorf("expected error")
		t.FailNow()
	}

	if err.Error() != expectedError {
		t.Errorf("Expected error %v, got %v", expectedError, err)
	}
}
