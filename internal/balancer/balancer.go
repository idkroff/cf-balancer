package balancer

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"net/url"

	"github.com/google/uuid"
	"github.com/idkroff/cf-balancer/internal/config"
	"github.com/idkroff/cf-balancer/internal/logger/sl"
	"github.com/idkroff/cf-balancer/internal/timer"
)

type Balancer struct {
	Queues   map[string]Queue
	Requests map[string]*Request
	Limits   config.CFLimits
	log      *slog.Logger
}

type Queue chan *Request

type Request struct {
	UUID        string
	URL         string
	RequestType string
	Headers     map[string]interface{}
	Body        map[string]interface{}

	// Possible status: waiting, done
	Status       string
	ResponseBody map[string]interface{}
}

var (
	ErrRequestNotFound = errors.New("request not found")
)

func New(limits config.CFLimits, log *slog.Logger) *Balancer {
	queues := map[string]Queue{}
	for k := range limits.TimingsByRoute {
		q := make(Queue, limits.MaxQueue)
		queues[k] = q
	}

	requests := map[string]*Request{}

	balancer := Balancer{Queues: queues, Requests: requests, Limits: limits, log: log}

	return &balancer
}

// Caution: will not block program, waiting not handled
func (b *Balancer) StartQueueTimers() {
	for k := range b.Queues {
		b.StartQueueTimer(k)
	}
	b.log.Info("balance queues timers started")
}

func (b *Balancer) StartQueueTimer(name string) {
	timer.SetInterval(func() { b.HandleQueue(name) }, b.Limits.TimingsByRoute[name]*1000)
}

func (b *Balancer) HandleQueue(name string) {
	var reqPtr *Request
	select {
	case reqPtr = <-b.Queues[name]:
	default:
		return
	}

	b.log.Debug("handling queue item", slog.Any("item", *reqPtr))

	bodyData, err := json.Marshal(reqPtr.Body)
	if err != nil {
		b.log.Error("unable to marshal request json", sl.Err(err))
		return
	}

	httpReq, err := http.NewRequest(reqPtr.RequestType, reqPtr.URL, bytes.NewBuffer(bodyData))
	if err != nil {
		b.log.Error("request initialization failed", sl.Err(err))
		return
	}

	response, err := http.DefaultClient.Do(httpReq)
	if err != nil {
		b.log.Error("request failed", sl.Err(err))
		return
	}

	resBodyData, err := io.ReadAll(response.Body)
	if err != nil {
		b.log.Error("unable to read response body", sl.Err(err))
		return
	}

	var resBody map[string]interface{}
	err = json.Unmarshal(resBodyData, &resBody)
	if err != nil {
		b.log.Error("unable to unmarshal response body", sl.Err(err))
		return
	}

	reqPtr.ResponseBody = resBody
	reqPtr.Status = "done"

	b.log.Debug("handle done")
}

func (b *Balancer) AddRequest(
	URL string,
	RequestType string,
	Body map[string]interface{},
	Headers map[string]interface{},
) (string, error) {
	u, err := url.Parse(URL)
	if err != nil {
		return "", fmt.Errorf("unable to parse URL from request: %w", err)
	}

	route := u.Path
	if _, exists := b.Queues[route]; !exists {
		return "", fmt.Errorf("unable to find queue for such route: %s: %s", URL, route)
	}

	if len(b.Queues[route]) >= b.Limits.MaxQueue {
		return "", fmt.Errorf("queue limit exceeded: %d", b.Limits.MaxQueue)
	}

	req := Request{
		URL:         URL,
		RequestType: RequestType,
		Body:        Body,
		Headers:     Headers,
		UUID:        uuid.New().String(),
		Status:      "waiting",
	}
	b.Requests[req.UUID] = &req
	go func() { b.Queues[route] <- &req }()

	return req.UUID, nil
}

func (b *Balancer) GetRequest(UUID string) (*Request, error) {
	req, exists := b.Requests[UUID]
	if !exists {
		return nil, ErrRequestNotFound
	}

	return req, nil
}
