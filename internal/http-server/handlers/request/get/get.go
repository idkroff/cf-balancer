package get

import (
	"errors"
	"log/slog"
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	"github.com/go-chi/render"
	"github.com/idkroff/cf-balancer/internal/balancer"
	"github.com/idkroff/cf-balancer/internal/logger/sl"
)

type RequestGetter interface {
	GetRequest(UUID string) (*balancer.Request, error)
}

type Response struct {
	Status string      `json:"status"`
	Body   interface{} `json:"body,omitempty"`
}

func New(log *slog.Logger, requestGetter RequestGetter) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		const op = "handlers.request.New"

		log := log.With(
			slog.String("op", op),
			slog.String("request_id", middleware.GetReqID(r.Context())),
		)

		UUID := chi.URLParam(r, "UUID")

		req, err := requestGetter.GetRequest(UUID)
		if errors.Is(err, balancer.ErrRequestNotFound) {
			w.WriteHeader(404)
			return
		}

		if err != nil {
			log.Error("unable to get request", sl.Err(err), slog.String("Request_UUID", UUID))
			w.WriteHeader(500)
			return
		}

		render.JSON(w, r, Response{
			Status: req.Status,
			Body:   req.ResponseBody,
		})
	}
}
