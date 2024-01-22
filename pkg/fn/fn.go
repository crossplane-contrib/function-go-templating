// Package fn defines the public interface for patch-and-transform functions.
package fn

import (
	fninternal "github.com/crossplane-contrib/function-go-templating/internal/fn"
)

var (
	// NewFunction creates a new Function with the given options.
	NewFunction = fninternal.NewFunction

	// WithLogger adds a logger to a Function.
	WithLogger = fninternal.WithLogger

	// WithFilesystem adds a filesystem to a Function.
	WithFileSystem = fninternal.WithFileSystem
)
