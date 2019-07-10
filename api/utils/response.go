package utils

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"sort"

	log "github.com/sirupsen/logrus"

	"github.com/golang/gddo/httputil/header"
	"github.com/pkg/errors"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
)

// Error Representation of errors in the API. These are divided into a small
// number of categories, essentially distinguished by whose fault the
// error is; i.e., is this error:
//  - a transient problem with the service, so worth trying again?
//  - not going to work until the user takes some other action, e.g., updating config?
type Error struct {
	Type Type
	// a message that can be printed out for the user
	Message string `json:"message"`
	// the underlying error that can be e.g., logged for developers to look at
	Err error
}

func (e *Error) Error() string {
	if e.Err != nil {
		return e.Err.Error()
	}

	return e.Message
}

// Type Type of error
type Type string

const (
	// Server The operation looked fine on paper, but something went wrong
	Server Type = "server"
	// Missing The thing you mentioned, whatever it is, just doesn't exist
	Missing = "missing"
	// User The operation was well-formed, but you asked for something that
	// can't happen at present (e.g., because you've not supplied some
	// config yet)
	User = "user"
)

// MarshalJSON Writes error as json
func (e *Error) MarshalJSON() ([]byte, error) {
	var errMsg string
	if e.Err != nil {
		errMsg = e.Err.Error()
	}
	jsonable := &struct {
		Type    string `json:"type"`
		Message string `json:"message"`
		Err     string `json:"error,omitempty"`
	}{
		Type:    string(e.Type),
		Message: e.Message,
		Err:     errMsg,
	}
	return json.Marshal(jsonable)
}

// UnmarshalJSON Parses json
func (e *Error) UnmarshalJSON(data []byte) error {
	jsonable := &struct {
		Type    string `json:"type"`
		Message string `json:"message"`
		Err     string `json:"error,omitempty"`
	}{}
	if err := json.Unmarshal(data, &jsonable); err != nil {
		return err
	}
	e.Type = Type(jsonable.Type)
	e.Message = jsonable.Message
	if jsonable.Err != "" {
		e.Err = errors.New(jsonable.Err)
	}
	return nil
}

// UnexpectedError any unexpected error
func UnexpectedError(message string, underlyingError error) error {
	return &Error{
		Type:    Server,
		Err:     underlyingError,
		Message: message,
	}
}

// TypeMissingError indication of underlying type missing
func TypeMissingError(message string, underlyingError error) error {
	return &Error{
		Type:    Missing,
		Err:     underlyingError,
		Message: message,
	}
}

// ValidationError Used for indication of validation errors
func ValidationError(kind, message string) error {
	return &Error{
		Type:    User,
		Err:     fmt.Errorf("%s failed validation", kind),
		Message: message,
	}
}

// CoverAllError Cover all other errors
func CoverAllError(err error) *Error {
	return &Error{
		Type:    User,
		Err:     err,
		Message: `Error: ` + err.Error(),
	}
}

func writeErrorWithCode(w http.ResponseWriter, r *http.Request, code int, err *Error) {
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
			fmt.Fprint(w, err.Message)
			return
		}
	}
	w.Header().Set(http.CanonicalHeaderKey("Content-Type"), "text/plain; charset=utf-8")
	w.WriteHeader(code)
	fmt.Fprint(w, err.Error())
}

// StringResponse Used for textual response data. I.e. log data
func StringResponse(w http.ResponseWriter, r *http.Request, result string) {
	w.Header().Set("Content-Type", "text/plain; charset=utf-8")
	w.WriteHeader(http.StatusOK)
	w.Write([]byte(result))
}

// JSONResponse Marshals response with header
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

// ErrorResponse Marshals error
func ErrorResponse(w http.ResponseWriter, r *http.Request, apiError error) {
	var outErr *Error
	var code int
	var ok bool

	err := errors.Cause(apiError)
	if outErr, ok = err.(*Error); !ok {
		outErr = CoverAllError(apiError)
	}

	log.Error(outErr.Message)

	switch apiError.(type) {
	case *url.Error:
		// Reflect any underlying network error
		writeErrorWithCode(w, r, http.StatusInternalServerError, outErr)

	case *apierrors.StatusError:
		// Reflect any underlying error from Kubernetes API
		se := apiError.(*apierrors.StatusError)
		writeErrorWithCode(w, r, int(se.ErrStatus.Code), outErr)

	default:
		switch outErr.Type {
		case Missing:
			code = http.StatusNotFound
		case User:
			code = http.StatusBadRequest
		case Server:
			code = http.StatusInternalServerError
		default:
			code = http.StatusInternalServerError
		}
		writeErrorWithCode(w, r, code, outErr)

	}
}

// negotiateContentType picks a content type based on the Accept
// header from a request, and a supplied list of available content
// types in order of preference. If the Accept header mentions more
// than one available content type, the one with the highest quality
// (`q`) parameter is chosen; if there are a number of those, the one
// that appears first in the available types is chosen.
func negotiateContentType(r *http.Request, orderedPref []string) string {
	specs := header.ParseAccept(r.Header, "Accept")
	if len(specs) == 0 {
		return orderedPref[0]
	}

	preferred := []header.AcceptSpec{}
	for _, spec := range specs {
		if indexOf(orderedPref, spec.Value) < len(orderedPref) {
			preferred = append(preferred, spec)
		}
	}
	if len(preferred) > 0 {
		sort.Sort(sortAccept{preferred, orderedPref})
		return preferred[0].Value
	}
	return ""
}

// sortAccept Holds accepted response types
type sortAccept struct {
	specs []header.AcceptSpec
	prefs []string
}

func (s sortAccept) Len() int {
	return len(s.specs)
}

// We want to sort by descending order of suitability: higher quality
// to lower quality, and preferred to less preferred.
func (s sortAccept) Less(i, j int) bool {
	switch {
	case s.specs[i].Q == s.specs[j].Q:
		return indexOf(s.prefs, s.specs[i].Value) < indexOf(s.prefs, s.specs[j].Value)
	default:
		return s.specs[i].Q > s.specs[j].Q
	}
}

func (s sortAccept) Swap(i, j int) {
	s.specs[i], s.specs[j] = s.specs[j], s.specs[i]
}

// This exists so we can search short slices of strings without
// requiring them to be sorted. Returning the len value if not found
// is so that it can be used directly in a comparison when sorting (a
// `-1` would mean "not found" was sorted before found entries).
func indexOf(ss []string, search string) int {
	for i, s := range ss {
		if s == search {
			return i
		}
	}
	return len(ss)
}
