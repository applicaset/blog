package httpapi

import (
	"errors"

	"github.com/applicaset/buildset/blog/content"
	"github.com/applicaset/buildset/pkg/httpx"
)

// Classify maps content's errors to a wire code and a sentence for the visitor. The in-process
// adapter uses it too, so both topologies answer identically. False means the system failed, not
// the request.
func Classify(err error) (httpx.Code, string, bool) {
	switch {
	case err == nil:
		return "", "", false
	case errors.Is(err, content.ErrPostNotFound):
		return httpx.CodeNotFound, "That post could not be found.", true
	case errors.Is(err, content.ErrInvalidPost),
		errors.Is(err, content.ErrInvalidStatus),
		errors.Is(err, content.ErrInvalidTransition),
		errors.Is(err, content.ErrUnsupportedContent):
		return httpx.CodeInvalidInput, err.Error(), true
	default:
		return "", "", false
	}
}
