// Package testutil provides test utilities and dependencies for gh-pt.
package testutil

// This file ensures test dependencies are direct imports.
// These libraries improve testability, reproducibility, and code quality.

import (
	// High priority - animation physics and time mocking
	_ "github.com/benbjohnson/clock"
	_ "github.com/charmbracelet/harmonica"
	_ "github.com/jarcoal/httpmock"

	// Medium priority - assertions and mocking
	_ "github.com/google/go-cmp/cmp"
	_ "go.uber.org/mock/gomock"
)
