package business

import "errors"

// ErrInvalidCredentials is returned for both unknown users and wrong
// passwords. Handlers map it to a single public 401 response so callers cannot
// distinguish account existence.
var ErrInvalidCredentials = errors.New("invalid credentials")
