package add

import (
	"log/slog"
	"net/http"

	"github.com/go-chi/chi/v5/middleware"
	"github.com/go-chi/render"
	"github.com/idkroff/cf-balancer/internal/logger/sl"
)

type RequestAdder interface {
	AddRequest(URL string, RequestType string, Body map[string]interface{}, Headers map[string]interface{}) (string, error)
}

type Request struct {
	URL         string                 `json:"url"`
	RequestType string                 `json:"type"`
	Headers     map[string]interface{} `json:"headers,omitempty"`
	Body        map[string]interface{} `json:"body,omitempty"`
}

type Response struct {
	RequestID string `json:"requestID,omitempty"`
}

func New(log *slog.Logger, requestAdder RequestAdder) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		const op = "handlers.request.New"

		log := log.With(
			slog.String("op", op),
			slog.String("request_id", middleware.GetReqID(r.Context())),
		)

		var req Request
		err := render.DecodeJSON(r.Body, &req)
		if err != nil {
			log.Error("failed to decode request body", sl.Err(err))
			w.WriteHeader(500)
			return
		}

		requestID, err := requestAdder.AddRequest(
			req.URL,
			req.RequestType,
			req.Body,
			req.Headers,
		)
		if err != nil {
			log.Error("unable to add request to queue", sl.Err(err), slog.Any("request", req))
			w.WriteHeader(500)
			return
		}

		log.Info("added request to queue", slog.Any("request", req))

		render.JSON(w, r, Response{
			RequestID: requestID,
		})
	}
}
