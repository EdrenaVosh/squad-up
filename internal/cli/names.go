package cli

import (
	"errors"

	"github.com/EdrenaVosh/squad-up/internal/workspace"
)

// nameError turns a problem with a preset name into a validation failure
// (exit code 2) with a fix.
func nameError(err error) error {
	fix := "use lowercase letters, digits and dashes, starting with a letter or digit (up to 64 characters)"
	if errors.Is(err, workspace.ErrReservedName) {
		fix = "pick another preset name"
	}
	return &ExitError{Code: 2, Err: err, Fix: fix}
}
