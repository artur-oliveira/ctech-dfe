package repositories

import "log/slog"

// logUnexpectedError makes failures in legacy no-error helper signatures
// observable until those signatures can be widened without breaking callers.
func logUnexpectedError(operation string, err error) {
	if err != nil {
		slog.Error("repository helper failed", "operation", operation, "err", err)
	}
}
