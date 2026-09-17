package cmd

import (
	"github.com/stretchr/testify/assert"
	"testing"
)

func TestGetDefaultPaths(t *testing.T) {
	assert.NotEmpty(t, GetDefaultTargetPath())
	assert.NotEmpty(t, GetDefaultClonePath())
	assert.NotEmpty(t, GetDefaultForkPath())
	assert.NotEmpty(t, GetDefaultInstallTypes())
}

func TestGetEnvPrefix(t *testing.T) {
	assert.Equal(t, "GH_PT", GetEnvPrefix())
}
