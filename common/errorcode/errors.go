// Package errorcode holds shared sentinel errors used to map business
// failures to public API responses.
package errorcode

import "errors"

// ErrInvalidCredentials is returned for both unknown users and wrong
// passwords. Handlers map it to a single public 401 response so callers cannot
// distinguish account existence.
var ErrInvalidCredentials = errors.New("invalid credentials")
