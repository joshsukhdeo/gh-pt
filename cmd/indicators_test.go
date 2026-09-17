package cmd

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestGetStateIndicator(t *testing.T) {
	tests := []struct {
		name               string
		isInstalled        bool
		isPinned           bool
		isPrerelease       bool
		isLatestPrerelease bool
		isLatestStable     bool
		disableIcons       bool
		expected           string
	}{
		{
			name:         "pinned installed (emoji)",
			isInstalled:  true,
			isPinned:     true,
			disableIcons: false,
			expected:     "📌🎯",
		},
		{
			name:         "pinned installed (text fallback)",
			isInstalled:  true,
			isPinned:     true,
			disableIcons: true,
			expected:     "^@",
		},
		{
			name:               "latest prerelease installed (emoji)",
			isInstalled:        true,
			isPrerelease:       true,
			isLatestPrerelease: true,
			disableIcons:       false,
			expected:           "🗻🧪🎯",
		},
		{
			name:               "latest prerelease installed (text fallback)",
			isInstalled:        true,
			isPrerelease:       true,
			isLatestPrerelease: true,
			disableIcons:       true,
			expected:           "*!@",
		},
		{
			name:           "latest stable not installed (emoji)",
			isLatestStable: true,
			disableIcons:   false,
			expected:       "🗻",
		},
		{
			name:           "latest stable not installed (text fallback)",
			isLatestStable: true,
			disableIcons:   true,
			expected:       "*",
		},
		{
			name:         "empty (all false)",
			disableIcons: false,
			expected:     "",
		},
		{
			name:         "empty (all false, text fallback)",
			disableIcons: true,
			expected:     "",
		},
		{
			name:           "pinned latest stable installed (emoji)",
			isInstalled:    true,
			isPinned:       true,
			isLatestStable: true,
			disableIcons:   false,
			expected:       "📌🗻🎯",
		},
		{
			name:           "pinned latest stable installed (text fallback)",
			isInstalled:    true,
			isPinned:       true,
			isLatestStable: true,
			disableIcons:   true,
			expected:       "^*@",
		},
		{
			name:         "older prerelease installed (emoji)",
			isInstalled:  true,
			isPrerelease: true,
			disableIcons: false,
			expected:     "🧪🎯",
		},
		{
			name:         "older prerelease installed (text fallback)",
			isInstalled:  true,
			isPrerelease: true,
			disableIcons: true,
			expected:     "!@",
		},
		{
			name:               "latest prerelease not installed (emoji)",
			isLatestPrerelease: true,
			disableIcons:       false,
			expected:           "🗻🧪",
		},
		{
			name:               "latest prerelease not installed (text fallback)",
			isLatestPrerelease: true,
			disableIcons:       true,
			expected:           "*!",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			actual := GetStateIndicator(
				tc.isInstalled,
				tc.isPinned,
				tc.isPrerelease,
				tc.isLatestPrerelease,
				tc.isLatestStable,
				tc.disableIcons,
			)
			assert.Equal(t, tc.expected, actual)
		})
	}
}
