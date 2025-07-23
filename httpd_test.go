package main

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"io"
)

func TestAuthMiddleware(t *testing.T) {
	mockNextHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("ok")) //nolint:all
	})

	tests := []struct {
		name              string
		configuredToken   string
		requestAuthHeader string
		expectedStatus    int
		expectNextCalled  bool
	}{
		{
			name:              "No token configured, no auth header",
			configuredToken:   "",
			requestAuthHeader: "",
			expectedStatus:    http.StatusOK,
			expectNextCalled:  true,
		},
		{
			name:              "No token configured, with auth header",
			configuredToken:   "",
			requestAuthHeader: "Token sometoken",
			expectedStatus:    http.StatusOK,
			expectNextCalled:  true,
		},
		{
			name:              "Token configured, correct auth header",
			configuredToken:   "secrettoken",
			requestAuthHeader: "Token secrettoken",
			expectedStatus:    http.StatusOK,
			expectNextCalled:  true,
		},
		{
			name:              "Token configured, incorrect auth header",
			configuredToken:   "secrettoken",
			requestAuthHeader: "Token wrongtoken",
			expectedStatus:    http.StatusUnauthorized,
			expectNextCalled:  false,
		},
		{
			name:              "Token configured, no auth header",
			configuredToken:   "secrettoken",
			requestAuthHeader: "",
			expectedStatus:    http.StatusUnauthorized,
			expectNextCalled:  false,
		},
		{
			name:              "Token configured, malformed auth header (no 'Token ' prefix)",
			configuredToken:   "secrettoken",
			requestAuthHeader: "secrettoken",
			expectedStatus:    http.StatusUnauthorized,
			expectNextCalled:  false,
		},
		{
			name:              "Token configured, malformed auth header (Bearer prefix)",
			configuredToken:   "secrettoken",
			requestAuthHeader: "Bearer secrettoken",
			expectedStatus:    http.StatusUnauthorized,
			expectNextCalled:  false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			nextCalled := false
			testHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				nextCalled = true
				mockNextHandler.ServeHTTP(w, r)
			})

			handlerToTest := authMiddleware(tt.configuredToken, testHandler)

			req := httptest.NewRequest("GET", "http://localhost/test", nil)
			if tt.requestAuthHeader != "" {
				req.Header.Set("Authorization", tt.requestAuthHeader)
			}

			rr := httptest.NewRecorder()
			handlerToTest.ServeHTTP(rr, req)

			assert.Equal(t, tt.expectedStatus, rr.Code, "HTTP status code mismatch")
			assert.Equal(t, tt.expectNextCalled, nextCalled, "Next handler called state mismatch")

			switch tt.expectedStatus {
			case http.StatusUnauthorized:
				assert.Contains(t, rr.Body.String(), "unauthorized", "Error message mismatch for unauthorized")
			case http.StatusOK:
				assert.Equal(t, "ok", rr.Body.String(), "Response body mismatch for OK")
			}
		})
	}
}

type mockExecutor struct{}

func (e *mockExecutor) execCommand(command string) (string, error) {
	if command == "error" {
		return "", assert.AnError
	}

	return command, nil
}

func TestHttpCommandHandler_ServeHTTP(t *testing.T) {

	tests := []struct {
		name               string
		requestURL         string
		requestMethod      string
		requestHeaders     map[string]string
		commands           map[string]Command
		mockExecOutput     string
		expectedStatusCode int
		expectedBody       string
	}{
		{
			name:          "valid command no args",
			requestURL:    "/cmd/testcmd",
			requestMethod: http.MethodGet,
			commands: map[string]Command{
				"testcmd": {Command: "echo hello"},
			},
			expectedStatusCode: http.StatusOK,
			expectedBody:       "echo hello",
		},
		{
			name:          "valid command with args",
			requestURL:    "/cmd/greet?name=world&times=2",
			requestMethod: http.MethodGet,
			commands: map[string]Command{
				"greet": {Command: "echo Hello %s %s times", Args: []string{"name", "times"}},
			},
			expectedStatusCode: http.StatusOK,
			expectedBody:       "echo Hello world 2 times",
		},
		{
			name:               "unknown command",
			requestURL:         "/cmd/unknown",
			requestMethod:      http.MethodGet,
			commands:           map[string]Command{},
			expectedStatusCode: http.StatusNotFound,
			expectedBody:       "unknown command \"unknown\"\n",
		},
		{
			name:          "missing required query parameter",
			requestURL:    "/cmd/greet?name=world", // missing 'times'
			requestMethod: http.MethodGet,
			commands: map[string]Command{
				"greet": {Command: "echo Hello %s %s times", Args: []string{"name", "times"}},
			},
			expectedStatusCode: http.StatusBadRequest,
			expectedBody:       "missing required query parameter \"times\"\n",
		},
		{
			name:          "command execution error",
			requestURL:    "/cmd/failcmd",
			requestMethod: http.MethodGet,
			commands: map[string]Command{
				"failcmd": {Command: "error"},
			},
			expectedStatusCode: http.StatusBadRequest,
			expectedBody:       "assert.AnError general error for testing\n",
		},
		{
			name:               "no command specified",
			requestURL:         "/cmd/",
			requestMethod:      http.MethodGet,
			commands:           map[string]Command{},
			expectedStatusCode: http.StatusBadRequest,
			expectedBody:       "no command specified\n",
		},
		{
			name:          "createCommand error - arg count mismatch",
			requestURL:    "/cmd/greet?name=world", // createCommand will fail due to placeholder mismatch
			requestMethod: http.MethodGet,
			commands: map[string]Command{
				"greet": {Command: "echo Hello %s %s", Args: []string{"name"}}, // 2 placeholders, 1 arg def
			},
			expectedStatusCode: http.StatusBadRequest,
			expectedBody:       "mismatch between placeholders (%s)=2 and number of args=1\n",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			executor := &mockExecutor{}
			handler := &httpCommandHandler{
				commands: tc.commands,
				// authToken is not directly tested here as it's part of authMiddleware
				executor: executor,
			}

			req := httptest.NewRequest(tc.requestMethod, tc.requestURL, nil)
			for k, v := range tc.requestHeaders {
				req.Header.Set(k, v)
			}
			rr := httptest.NewRecorder()

			handler.ServeHTTP(rr, req)

			assert.Equal(t, tc.expectedStatusCode, rr.Code)
			body, err := io.ReadAll(rr.Body)
			require.NoError(t, err)
			assert.Equal(t, tc.expectedBody, string(body))
			if tc.expectedStatusCode == http.StatusOK {
				assert.Equal(t, "text/plain", rr.Header().Get("Content-Type"))
			}
		})
	}
}
