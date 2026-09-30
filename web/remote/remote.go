// Package remote satisfies the site's dependencies by calling each service over HTTP. It mirrors
// the composition root's in-process adapters, wrapping a client rather than a service value. It
// lives here so a binary serving only the site links no service code, hashing or database driver.
package remote

import (
	"errors"

	"github.com/applicaset/buildset/blog/web"
	"github.com/applicaset/buildset/pkg/httpx"
)

// translate turns a domain failure into the sentinel the site matches on. Everything else passes
// through unchanged: if an unreachable identity service yielded ErrNotFound, the site would treat
// every visitor as anonymous.
func translate(err error) error {
	if err == nil {
		return nil
	}

	var domain *httpx.Error
	if !errors.As(err, &domain) {
		return err
	}

	if translated := web.ErrorFromCode(domain.Code, domain.Message); translated != nil {
		return translated
	}

	return err
}
