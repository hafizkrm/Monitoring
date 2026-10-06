package monitoring

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/hafizkrm/Monitoring/backend/internal/config"
)

func TestTSDBQueryHandler_Limits(t *testing.T) {
	// Mock Prometheus Server
	promServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		query := r.URL.Query().Get("query")
		
		// Simulate a slow query
		if strings.Contains(query, "slow") {
			time.Sleep(6 * time.Second)
		}

		// Simulate large result
		if strings.Contains(query, "large") {
			w.WriteHeader(http.StatusOK)
			w.Write([]byte(`{"status":"success","data":{"resultType":"vector","result":[{"metric":{},"values":[[1,"1"]]},{"metric":{},"values":[[1,"1"]]},{"metric":{},"values":[[1,"1"]]}]}}`))
			return
		}

		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{"status":"success","data":{"resultType":"vector","result":[{"metric":{},"values":[[1,"1"]]}]}}`))
	}))
	defer promServer.Close()

	cfg := config.PromQLConfig{
		MaxRange:          1 * time.Hour,
		MaxResultSize:     2,
		RateLimitRequests: 2,
		RateLimitWindow:   1 * time.Minute,
	}



	tests := []struct {
		name       string
		role       string
		query      string
		wantStatus int
	}{
		{
			name:       "non-admin rejected",
			role:       "user",
			query:      "up",
			wantStatus: http.StatusForbidden,
		},
		{
			name:       "admin accepted",
			role:       "admin",
			query:      "up",
			wantStatus: http.StatusOK,
		},
		{
			name:       "range below limit",
			role:       "admin",
			query:      "rate(up[30m])",
			wantStatus: http.StatusOK,
		},
		{
			name:       "range above limit rejected",
			role:       "admin",
			query:      "rate(up[2h])",
			wantStatus: http.StatusBadRequest,
		},
		{
			name:       "result below limit",
			role:       "admin",
			query:      "up",
			wantStatus: http.StatusOK,
		},
		{
			name:       "result above limit rejected",
			role:       "admin",
			query:      "large_query",
			wantStatus: http.StatusRequestEntityTooLarge,
		},
		{
			name:       "timeout rejected",
			role:       "admin",
			query:      "slow_query",
			wantStatus: http.StatusBadGateway,
		},
	}

	// Because of rate limit, we need to reset the handler or just increase rate limit for the test.
	// Actually, wait, RateLimitRequests is 2. The above test has 5 valid requests. It will hit rate limit!
	// Let's create a new handler per test case so rate limiter is fresh.
	
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			h := TSDBQueryHandler(promServer.URL, cfg)
			req := httptest.NewRequest("GET", "/api/tsdb/query?query="+tt.query, nil)
			ctx := context.WithValue(req.Context(), "role", tt.role)
			req = req.WithContext(ctx)

			rr := httptest.NewRecorder()
			h.ServeHTTP(rr, req)

			if rr.Code != tt.wantStatus {
				t.Errorf("expected status %d, got %d. Body: %s", tt.wantStatus, rr.Code, rr.Body.String())
			}
		})
	}

	// Test Rate Limiting explicitly
	t.Run("rate limit exceeded", func(t *testing.T) {
		h := TSDBQueryHandler(promServer.URL, cfg)
		// We allow 2 requests per minute
		for i := 0; i < 2; i++ {
			req := httptest.NewRequest("GET", "/api/tsdb/query?query=up", nil)
			ctx := context.WithValue(req.Context(), "role", "admin")
			req = req.WithContext(ctx)
			rr := httptest.NewRecorder()
			h.ServeHTTP(rr, req)
			if rr.Code != http.StatusOK {
				t.Errorf("expected OK, got %d", rr.Code)
			}
		}

		// 3rd request should fail
		req := httptest.NewRequest("GET", "/api/tsdb/query?query=up", nil)
		ctx := context.WithValue(req.Context(), "role", "admin")
		req = req.WithContext(ctx)
		rr := httptest.NewRecorder()
		h.ServeHTTP(rr, req)
		if rr.Code != http.StatusTooManyRequests {
			t.Errorf("expected TooManyRequests, got %d", rr.Code)
		}
	})
}
