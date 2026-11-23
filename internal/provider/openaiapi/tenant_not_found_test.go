package openaiapi

import (
	"io"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/openai/openai-go/v3"
	"github.com/openai/openai-go/v3/option"
)

func TestTenantRuleNotFoundUsesExactResponse(t *testing.T) {
	const missing = "Federation rule not found."
	tests := []struct {
		name            string
		body            string
		status          int
		want            bool
		wantMessageOnly bool
	}{
		{
			name:   "tenant API detail",
			body:   `{"detail":"Federation rule not found."}`,
			status: http.StatusNotFound,
			want:   true,
		},
		{
			name:            "OpenAI error envelope",
			body:            `{"error":{"message":"Federation rule not found."}}`,
			status:          http.StatusNotFound,
			want:            true,
			wantMessageOnly: true,
		},
		{
			name:   "beta gate",
			body:   `{"error":{"message":"This endpoint is not available."}}`,
			status: http.StatusNotFound,
		},
		{
			name:   "gateway response",
			body:   `{"detail":"Not Found"}`,
			status: http.StatusNotFound,
		},
		{
			name:   "nested error takes precedence",
			body:   `{"error":{"code":"route_not_found"},"detail":"Federation rule not found."}`,
			status: http.StatusNotFound,
		},
		{
			name:   "wrong HTTP status",
			body:   `{"detail":"Federation rule not found."}`,
			status: http.StatusForbidden,
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
				w.Header().Set("Content-Type", "application/json")
				w.WriteHeader(test.status)
				_, _ = io.WriteString(w, test.body)
			}))
			defer server.Close()

			client := openai.NewClient(
				option.WithBaseURL(server.URL),
				option.WithAPIKey("synthetic-tenant-key"),
				option.WithMaxRetries(0),
			)
			var result map[string]any
			err := client.Get(t.Context(), "/v1/tenant/workload_identity_providers/provider/federation_rules/rule", nil, &result)
			for range 2 {
				if got := IsNotFoundMessageOrDetail(err, missing); got != test.want {
					t.Fatalf("IsNotFoundMessageOrDetail returned %t, want %t", got, test.want)
				}
			}
			if got := IsNotFoundMessage(err, missing); got != test.wantMessageOnly {
				t.Fatalf("IsNotFoundMessage returned %t, want %t", got, test.wantMessageOnly)
			}
		})
	}
}
