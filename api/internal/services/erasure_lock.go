package services

import "context"

// BlockedFunc reports whether a sub is locked or erased by the account-deletion
// saga (ctech-go-common erasure.Store.Blocked). Nil means the lock is off.
type BlockedFunc func(ctx context.Context, sub string) (bool, error)
