package router

import (
	"context"
	"net/http"

	"github.com/felixge/httpsnoop"
	"github.com/rs/xid"
	"github.com/rs/zerolog"
	"github.com/urfave/negroni/v3"
)

func zerologRequestLogger() negroni.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request, next http.HandlerFunc) {
		m := httpsnoop.CaptureMetrics(next, w, r)

		logger := zerolog.Ctx(r.Context())
		var ev *zerolog.Event
		switch {
		case m.Code >= 400 && m.Code <= 499:
			ev = logger.Warn() //nolint:zerologlint // Msg for ev is called later
		case m.Code >= 500:
			ev = logger.Error() //nolint:zerologlint // Msg for ev is called later
		default:
			ev = logger.Info() //nolint:zerologlint // Msg for ev is called later
		}

		ev.
			Str("remote_addr", r.RemoteAddr).
			Str("referer", r.Referer()).
			Str("method", r.Method).
			Str("path", r.URL.Path).
			Str("query", r.URL.RawQuery).
			Int("status", m.Code).
			Int64("body_size", m.Written).
			Int64("elapsed_ms", m.Duration.Milliseconds()).
			Str("user_agent", r.UserAgent()).
			Msg(http.StatusText(m.Code))
	}
}

type setZerologLoggerFn func(context.Context) zerolog.Logger

// zerologLoggerWithRequestId returns a zerolog logger with a request_id field with a new GUID
func zerologLoggerWithRequestId(ctx context.Context) zerolog.Logger {
	return zerolog.Ctx(ctx).With().Str("request_id", xid.New().String()).Logger()
}

// setZerologLogger attaches the zerolog logger returned from each loggerFns function to a shallow copy of the request context
// The logger can then be accessed in a controller method by calling zerolog.Ctx(ctx)
func setZerologLogger(loggerFns ...setZerologLoggerFn) negroni.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request, next http.HandlerFunc) {
		for _, loggerFn := range loggerFns {
			logger := loggerFn(r.Context())
			r = r.WithContext(logger.WithContext(r.Context()))
		}
		next.ServeHTTP(w, r)
	}
}
