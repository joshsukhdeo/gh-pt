package cmd

import "strings"

// GetStateIndicator returns visual indicators (📌🗻⚡🎯 or ^*!@) for an asset's state.
func GetStateIndicator(isInstalled, isPinned, isPrerelease, isLatestPrerelease, isLatestStable, disableIcons bool) string {
	var sb strings.Builder
	if isPinned {
		if disableIcons {
			sb.WriteRune('^')
		} else {
			sb.WriteString("📌")
		}
	}
	if isLatestStable || isLatestPrerelease {
		if disableIcons {
			sb.WriteRune('*')
		} else {
			sb.WriteString("🗻")
		}
	}
	if isPrerelease || isLatestPrerelease {
		if disableIcons {
			sb.WriteRune('!')
		} else {
			sb.WriteString("⚡")
		}
	}
	if isInstalled {
		if disableIcons {
			sb.WriteRune('@')
		} else {
			sb.WriteString("🎯")
		}
	}
	return sb.String()
}
