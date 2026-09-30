package web

import "github.com/applicaset/buildset/pkg/httpx"

// Error is what a dependency returns when the request, not the system, is at fault. Kind is one of
// the sentinels above so errors.Is keeps working; Message is written for the visitor.
type Error struct {
	Kind    error
	Message string
}

func NewError(kind error, message string) *Error {
	return &Error{Kind: kind, Message: message}
}

func (e *Error) Error() string {
	return e.Message
}

func (e *Error) Unwrap() error {
	return e.Kind
}

// ErrorFromCode turns a wire code into this package's sentinel and keeps the service's message for
// the visitor. Both adapters use it, so they map codes the same way. An unknown code returns nil,
// and the caller must treat it as a system failure.
func ErrorFromCode(code httpx.Code, message string) error {
	switch code {
	case httpx.CodeNotFound:
		return NewError(ErrNotFound, message)
	case httpx.CodeInvalidInput:
		return NewError(ErrInvalidInput, message)
	case httpx.CodeConflict:
		return NewError(ErrConflict, message)
	case httpx.CodeForbidden:
		// No service the blog calls refuses by role; the blog asks authz itself.
		return nil
	default:
		return nil
	}
}
