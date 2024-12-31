package httpd

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/ayinke-llc/sdump"
	"github.com/ayinke-llc/sdump/config"
	"github.com/ayinke-llc/sdump/internal/util"
	"github.com/go-chi/chi/v5"
	"github.com/go-chi/render"
	"github.com/google/uuid"
	"github.com/r3labs/sse/v2"
	"github.com/sirupsen/logrus"
	"go.opentelemetry.io/otel/codes"
)

type urlHandler struct {
	logger     *logrus.Entry
	urlRepo    sdump.URLRepository
	ingestRepo sdump.IngestRepository
	userRepo   sdump.UserRepository
	cfg        config.Config
	sseServer  *sse.Server
}

type createURLRequest struct {
	SSHFingerprint   string `json:"ssh_fingerprint,omitempty"`
	ForceNewEndpoint bool   `json:"force_new_endpoint,omitempty"`
}

func (u *urlHandler) create(w http.ResponseWriter, r *http.Request) {
	ctx, span, requestID := getTracer(r.Context(), r, "url.create")
	defer span.End()

	logger := u.logger.WithField("method", "url.create").
		WithField("request_id", requestID)

	logger.Debug("Creating new url endpoint")

	req := new(createURLRequest)

	if err := json.NewDecoder(r.Body).Decode(req); err != nil {
		span.SetStatus(codes.Error, "invalid request body")
		_ = render.Render(w, r, newAPIError(http.StatusBadRequest, "please provide a valid request body"))
		return
	}

	if util.IsStringEmpty(req.SSHFingerprint) {
		span.SetStatus(codes.Error, "please provide ssh fingerprint")
		_ = render.Render(w, r, newAPIError(http.StatusBadRequest, "please provide your ssh fingerprint"))
		return
	}

	user, err := u.userRepo.Find(ctx, &sdump.FindUserOptions{
		SSHKeyFingerprint: req.SSHFingerprint,
	})

	if err != nil && !errors.Is(err, sdump.ErrUserNotFound) {

		logger.WithError(err).Error("could not find user from database")
		span.SetStatus(codes.Error, "could not find user from database")
		_ = render.Render(w, r, newAPIError(http.StatusInternalServerError, "could not find user from database"))
		return
	}

	var userID uuid.UUID

	switch err {

	default:
		userID = user.ID

	case sdump.ErrUserNotFound:

		user := &sdump.User{
			SSHFingerPrint: req.SSHFingerprint,
			IsBanned:       false,
		}

		err = u.userRepo.Create(ctx, user)
		if err != nil {
			span.SetStatus(codes.Error, "could not create user")
			logger.WithError(err).
				WithField("ssh_fingerprint", req.SSHFingerprint).
				Error("could not create user")

			_ = render.Render(w, r, newAPIError(http.StatusInternalServerError, "an error occurred while storing your ssh fingerprint"))
			return
		}

		userID = user.ID
	}

	endpoint, err := u.createOrFetchEndpoint(ctx, sdump.NewURLEndpoint(userID), req.ForceNewEndpoint)
	if err != nil {

		logger.WithError(err).Error("could not create url endpoint")

		span.SetStatus(codes.Error, "could not create url endpoint")

		_ = render.Render(w, r, newAPIError(http.StatusInternalServerError,
			"an error occurred while generating endpoint"))
		return
	}

	go func() {
		_ = u.sseServer.CreateStream(endpoint.PubChannel())
	}()

	createdURLMetrics.Inc()
	span.SetStatus(codes.Ok, "created url")
	_ = render.Render(w, r, &createdURLEndpointResponse{
		APIStatus: newAPIStatus(http.StatusOK, "created url endpoint"),
		SSE: struct {
			Channel string "json:\"channel,omitempty\""
		}{
			Channel: endpoint.PubChannel(),
		},
		URL: struct {
			FQDN                  string "json:\"fqdn,omitempty\""
			Identifier            string "json:\"identifier,omitempty\""
			HumanReadableEndpoint string "json:\"human_readable_endpoint,omitempty\""
		}{
			FQDN:       u.cfg.HTTP.Domain,
			Identifier: endpoint.Reference,
			HumanReadableEndpoint: fmt.Sprintf("%s/%s",
				u.cfg.HTTP.Domain, endpoint.Reference),
		},
	})
}

func (u *urlHandler) createOrFetchEndpoint(
	ctx context.Context,
	endpoint *sdump.URLEndpoint,
	forceRefresh bool,
) (*sdump.URLEndpoint, error) {
	if forceRefresh {
		return endpoint, u.urlRepo.Create(ctx, endpoint)
	}

	lastUsedEndpoint, err := u.urlRepo.Latest(ctx, endpoint.UserID)
	if err == nil {
		return lastUsedEndpoint, nil
	}

	if errors.Is(err, sdump.ErrURLEndpointNotFound) {
		if err := u.urlRepo.Create(ctx, endpoint); err != nil {
			return nil, err
		}

		return endpoint, nil
	}

	return endpoint, err
}

func (u *urlHandler) ingest(w http.ResponseWriter, r *http.Request) {
	ctx, span, requestID := getTracer(r.Context(), r, "url.ingest")
	defer span.End()

	if r.Method == http.MethodOptions {
		_ = render.Render(w, r, newAPIError(http.StatusBadRequest, "Unsupported HTTP method"))
		return
	}

	reference := chi.URLParam(r, "reference")

	logger := u.logger.WithField("request_id", requestID).
		WithField("method", "urlHandler.ingest").
		WithField("reference", reference)

	logger.Debug("Ingesting http request")

	r.Body = http.MaxBytesReader(w, r.Body, u.cfg.HTTP.MaxRequestBodySize)

	endpoint, err := u.urlRepo.Get(ctx, &sdump.FindURLOptions{
		Reference: reference,
	})
	if errors.Is(err, sdump.ErrURLEndpointNotFound) {
		_ = render.Render(w, r, newAPIError(http.StatusNotFound,
			"Dump url does not exist"))
		return
	}

	if err != nil {

		span.SetStatus(codes.Error, "url not found")

		failedIngestedHTTPRequestsCounter.Inc()
		logger.WithError(err).Error("could not find dump url by reference")
		_ = render.Render(w, r, newAPIError(http.StatusInternalServerError,
			"an error occurred while ingesting HTTP request"))
		return
	}

	s := &strings.Builder{}

	size, err := io.Copy(s, r.Body)
	if err != nil {
		failedIngestedHTTPRequestsCounter.Inc()
		msg := "could not copy request body"
		status := http.StatusInternalServerError
		if maxErr, ok := err.(*http.MaxBytesError); ok {
			msg = maxErr.Error()
			status = http.StatusBadRequest
		}

		logger.WithError(err).Error("could not copy request body")
		span.SetStatus(codes.Error, "could not copy request body for ingestion")
		_ = render.Render(w, r, newAPIError(status,
			msg))
		return
	}

	ingestedRequest := &sdump.IngestHTTPRequest{
		UrlID: endpoint.ID,
		Request: sdump.RequestDefinition{
			Body:      s.String(),
			Query:     r.URL.Query().Encode(),
			Headers:   r.Header,
			IPAddress: util.GetIP(r),
			Size:      size,
			Method:    r.Method,
		},
	}

	if err := u.ingestRepo.Create(ctx, ingestedRequest); err != nil {
		failedIngestedHTTPRequestsCounter.Inc()
		logger.WithError(err).Error("could not ingest request")
		span.SetStatus(codes.Error, "could not ingest request")
		_ = render.Render(w, r, newAPIError(http.StatusInternalServerError,
			"an error occurred while ingesting request"))
		return
	}

	ingestedHTTPRequestsCounter.Inc()

	go func() {
		if !u.sseServer.StreamExists(endpoint.PubChannel()) {
			_ = u.sseServer.CreateStream(endpoint.PubChannel())
		}

		b := new(bytes.Buffer)

		var sseEvent struct {
			Request   sdump.RequestDefinition `json:"request"`
			ID        string                  `json:"id"`
			CreatedAt time.Time               `json:"created_at,omitempty"`
		}

		sseEvent.Request = ingestedRequest.Request
		sseEvent.ID = ingestedRequest.ID.String()
		sseEvent.CreatedAt = ingestedRequest.CreatedAt

		if err := json.NewEncoder(b).Encode(&sseEvent); err != nil {
			logger.WithError(err).Error("could not format SSE event")
			return
		}

		u.sseServer.Publish(endpoint.PubChannel(), &sse.Event{
			ID:   []byte(ingestedRequest.ID.String()),
			Data: b.Bytes(),
		})
	}()

	span.SetStatus(codes.Ok, "ingested request")
	_ = render.Render(w, r, newAPIStatus(http.StatusAccepted,
		"Request ingested"))
}
