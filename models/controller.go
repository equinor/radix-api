package models

import (
	"context"
	"errors"
	"io"
	"net/http"

	radixhttp "github.com/equinor/radix-common/net/http"
	"github.com/rs/zerolog/log"
)

// RadixHandlerFunc Pattern for handler functions
type RadixHandlerFunc func(Accounts, http.ResponseWriter, *http.Request)

// Controller Pattern of an rest/stream controller
type Controller interface {
	GetRoutes() Routes
}

// DefaultController Default implementation
type DefaultController struct {
}

// ErrorResponse Marshals error for user requester
func (c *DefaultController) ErrorResponse(w http.ResponseWriter, r *http.Request, err error) {

	logError(r.Context(), err)
	err = radixhttp.ErrorResponse(w, r, err)
	if err != nil {
		log.Ctx(r.Context()).Error().Err(err).Msg("failed to write response")
	}
}

func logError(ctx context.Context, err error) {
	event := log.Ctx(ctx).Warn().Err(err)

	var httpErr *radixhttp.Error
	if errors.As(err, &httpErr) {
		event.Str("user_message", httpErr.Message)
	}

	event.Msg("controller error")
}

// JSONResponse Marshals response with header
func (c *DefaultController) JSONResponse(w http.ResponseWriter, r *http.Request, result interface{}) {
	err := radixhttp.JSONResponse(w, r, result)
	if err != nil {
		log.Ctx(r.Context()).Err(err).Msg("failed to write response")
	}
}

// ReaderFileResponse writes the content from the reader to the response,
// and sets Content-Disposition=attachment; filename=<filename arg>
func (c *DefaultController) ReaderFileResponse(w http.ResponseWriter, r *http.Request, reader io.Reader, fileName, contentType string) {
	err := radixhttp.ReaderFileResponse(w, reader, fileName, contentType)
	if err != nil {
		log.Ctx(r.Context()).Err(err).Msg("failed to write response")
	}
}

// ReaderResponse writes the content from the reader to the response,
func (c *DefaultController) ReaderResponse(w http.ResponseWriter, r *http.Request, reader io.Reader, contentType string) {
	err := radixhttp.ReaderResponse(w, reader, contentType)
	if err != nil {
		log.Ctx(r.Context()).Err(err).Msg("failed to write response")
	}

}

// ByteArrayResponse Used for response data. I.e. image
func (c *DefaultController) ByteArrayResponse(w http.ResponseWriter, r *http.Request, contentType string, result []byte) {
	err := radixhttp.ByteArrayResponse(w, r, contentType, result)
	if err != nil {
		log.Ctx(r.Context()).Err(err).Msg("failed to write response")
	}
}
