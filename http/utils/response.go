package utils

import (
	"encoding/json"
	"fmt"
	"github.com/Sirupsen/logrus"
	"github.com/pkg/errors"
	radError "github.com/statoil/radix-api-go/errors"
	"net/http"
)

func WriteError(w http.ResponseWriter, r *http.Request, code int, err error) {
	// An Accept header with "application/json" is sent by clients
	// understanding how to decode JSON errors. Older clients don't
	// send an Accept header, so we just give them the error text.
	if len(r.Header.Get("Accept")) > 0 {
		switch negotiateContentType(r, []string{"application/json", "text/plain"}) {
		case "application/json":
			body, encodeErr := json.Marshal(err)
			if encodeErr != nil {
				w.Header().Set(http.CanonicalHeaderKey("Content-Type"), "text/plain; charset=utf-8")
				w.WriteHeader(http.StatusInternalServerError)
				fmt.Fprintf(w, "Error encoding error response: %s\n\nOriginal error: %s", encodeErr.Error(), err.Error())
				return
			}
			w.Header().Set(http.CanonicalHeaderKey("Content-Type"), "application/json; charset=utf-8")
			w.WriteHeader(code)
			w.Write(body)
			return
		case "text/plain":
			w.Header().Set(http.CanonicalHeaderKey("Content-Type"), "text/plain; charset=utf-8")
			w.WriteHeader(code)
			switch err := err.(type) {
			case *radError.Error:
				fmt.Fprint(w, err.Help)
			default:
				fmt.Fprint(w, err.Error())
			}
			return
		}
	}
	w.Header().Set(http.CanonicalHeaderKey("Content-Type"), "text/plain; charset=utf-8")
	w.WriteHeader(code)
	fmt.Fprint(w, err.Error())
	logrus.Error(err.Error())
}

func JSONResponse(w http.ResponseWriter, r *http.Request, result interface{}) {
	body, err := json.Marshal(result)
	if err != nil {
		ErrorResponse(w, r, err)
		return
	}

	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(http.StatusOK)
	w.Write(body)
}

func ErrorResponse(w http.ResponseWriter, r *http.Request, apiError error) {
	var outErr *radError.Error
	var code int
	var ok bool

	err := errors.Cause(apiError)
	if outErr, ok = err.(*radError.Error); !ok {
		outErr = radError.CoverAllError(apiError)
	}
	switch outErr.Type {
	case radError.Missing:
		code = http.StatusNotFound
	case radError.User:
		code = http.StatusUnprocessableEntity
	case radError.Server:
		code = http.StatusInternalServerError
	default:
		code = http.StatusInternalServerError
	}
	WriteError(w, r, code, outErr)
}
