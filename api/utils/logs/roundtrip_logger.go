package logs

import (
	"net/http"
	"time"

	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"
)

// RoundTripperFunc implements http.RoundTripper for convenient usage.
type RoundTripperFunc func(*http.Request) (*http.Response, error)

// RoundTrip satisfies http.RoundTripper and calls fn.
func (fn RoundTripperFunc) RoundTrip(req *http.Request) (*http.Response, error) {
	return fn(req)
}

type WithFunc func(e *zerolog.Event)

// Logger returns a http.RoundTripper that logs failed requests, and add traces for successfull requests
//
// nolint Zerolog complains about potential unsent event, but we send the event on line 50
func Logger(fns ...WithFunc) func(t http.RoundTripper) http.RoundTripper {
	return func(t http.RoundTripper) http.RoundTripper {
		return RoundTripperFunc(func(r *http.Request) (*http.Response, error) {
			start := time.Now()
			logger := log.Ctx(r.Context())

			resp, err := t.RoundTrip(r)

			var ev *zerolog.Event
			switch {
			case err != nil:
				logger.Error().Err(err)
			case resp.StatusCode >= 400 && resp.StatusCode <= 499:
				ev = logger.Warn()
			case resp.StatusCode >= 500:
				ev = logger.Error()
			default:
				ev = logger.Trace()
			}

			for _, fn := range fns {
				ev.Func(fn)
			}
			ev.
				Str("method", r.Method).
				Stringer("path", r.URL).
				Int64("elapsed_ms", time.Since(start).Milliseconds()).
				Msg(http.StatusText(resp.StatusCode))
			return resp, err
		})
	}
}
