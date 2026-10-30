// Package errors defines business and data-access sentinel errors that are
// specific to this application. Transport-level AppError and business codes
// remain in common/errors.
package errors

import stderrors "errors"

var (
	// ErrUserNotFound is returned when a user lookup has no match.
	ErrUserNotFound = stderrors.New("user not found")

	// ErrInvalidCredentials is returned for both unknown users and wrong
	// passwords. Handlers map it to a single public 401 response so callers
	// cannot distinguish account existence.
	ErrInvalidCredentials = stderrors.New("invalid credentials")
)
