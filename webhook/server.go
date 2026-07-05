package webhook

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	"github.com/rathore/langchain-agent/agent"
)

type request struct {
	Prompt string `json:"prompt"`
}

type response struct {
	Answer string `json:"answer,omitempty"`
	Error  string `json:"error,omitempty"`
}

// Start runs an HTTP server on the given port that exposes:
//   - POST /webhook  — body {"prompt": "..."}; runs the agent and returns its answer
//   - GET  /health   — liveness probe
//
// It blocks until ctx is cancelled or the server fails. Run it in its own goroutine.
func Start(ctx context.Context, port int, ag *agent.Agent) error {
	mux := http.NewServeMux()

	mux.HandleFunc("/health", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("OK"))
	})

	mux.HandleFunc("/webhook", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			writeJSON(w, http.StatusMethodNotAllowed, response{Error: "POST required"})
			return
		}
		var req request
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			writeJSON(w, http.StatusBadRequest, response{Error: "invalid JSON: " + err.Error()})
			return
		}
		if req.Prompt == "" {
			writeJSON(w, http.StatusBadRequest, response{Error: "prompt is required"})
			return
		}

		fmt.Printf("\n[Webhook] %s\n", req.Prompt)
		answer, err := ag.Run(r.Context(), req.Prompt)
		if err != nil {
			writeJSON(w, http.StatusInternalServerError, response{Error: err.Error()})
			return
		}
		writeJSON(w, http.StatusOK, response{Answer: answer})
	})

	srv := &http.Server{
		Addr:              fmt.Sprintf(":%d", port),
		Handler:           mux,
		ReadHeaderTimeout: 5 * time.Second,
	}

	errCh := make(chan error, 1)
	go func() { errCh <- srv.ListenAndServe() }()

	select {
	case <-ctx.Done():
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		return srv.Shutdown(shutdownCtx)
	case err := <-errCh:
		if err == http.ErrServerClosed {
			return nil
		}
		return err
	}
}

func writeJSON(w http.ResponseWriter, code int, body response) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(code)
	_ = json.NewEncoder(w).Encode(body)
}
