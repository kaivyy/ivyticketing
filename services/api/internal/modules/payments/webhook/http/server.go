package webhookhttp

import (
	"io"
	"net/http"

	"github.com/go-chi/chi/v5"
	chimw "github.com/go-chi/chi/v5/middleware"

	"github.com/varin/ivyticketing/services/api/internal/modules/payments"
	gw "github.com/varin/ivyticketing/services/api/internal/modules/payments/gateway"
)

type Server struct {
	processor *payments.Processor
	registry  *gw.Registry
}

func NewServer(processor *payments.Processor, registry *gw.Registry) *Server {
	return &Server{processor: processor, registry: registry}
}

func (s *Server) Router() http.Handler {
	r := chi.NewRouter()
	r.Use(chimw.Recoverer)
	r.Get("/healthz", func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
	})
	r.Post("/webhooks/duitku", s.handle("duitku"))
	r.Post("/webhooks/xendit", s.handle("xendit"))
	return r
}

func (s *Server) handle(name string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		g, ok := s.registry.Get(name)
		if !ok {
			w.WriteHeader(http.StatusNotFound)
			return
		}
		body, err := io.ReadAll(r.Body)
		if err != nil {
			w.WriteHeader(http.StatusBadRequest)
			return
		}
		headers := make(map[string][]string)
		for k, v := range r.Header {
			headers[k] = v
		}
		if err := s.processor.ProcessRaw(r.Context(), g, headers, body); err != nil {
			if err == payments.ErrInvalidSignature {
				w.WriteHeader(http.StatusUnauthorized)
				return
			}
			// For other errors, return 200 to prevent gateway retry storms.
		}
		w.WriteHeader(http.StatusOK)
	}
}
