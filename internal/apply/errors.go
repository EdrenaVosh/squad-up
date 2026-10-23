package apply

import "errors"

var (
	// ErrConflicts means files that do not belong to squad are in the way.
	ErrConflicts = errors.New("files that squad did not write are in the way")
	// ErrDrift means files squad wrote were edited by hand and the user has
	// not said what to do with the edits.
	ErrDrift = errors.New("manual edits detected")
	// ErrStale means the config changed between planning and writing.
	ErrStale = errors.New("the config changed while the plan was being reviewed")
	// ErrNothingToUndo means no operation with a backup is on record.
	ErrNothingToUndo = errors.New("nothing to undo")
	// ErrNoBackup means the requested backup does not exist (any more).
	ErrNoBackup = errors.New("backup not found")
)
