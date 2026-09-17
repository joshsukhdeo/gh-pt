package selector

import (
	"fmt"
	"regexp"
	"runtime"
	"strings"
	"path/filepath"
)

type Selector struct {
	Kind             SelectorKind
	Items            []*SelectorItem
	NamesMatcher     []string
	RegexpMatchers   []string
	Single           bool
	AllowForeignArch bool
	Repository       string
}


func getMajorityPrefix(items []*SelectorItem) string {
	n := len(items)
	if n == 0 {
		return ""
	}
	bestPrefix := ""
	for _, item := range items {
		for l := len(item.Name); l > len(bestPrefix); l-- {
			prefix := item.Name[:l]
			count := 0
			for _, other := range items {
				if strings.HasPrefix(other.Name, prefix) {
					count++
				}
			}
			if count > n/2 {
				if len(prefix) > len(bestPrefix) {
					bestPrefix = prefix
				}
				break
			}
		}
	}
	return bestPrefix
}

type fallbackLevel struct {
	name   string
	filter func(string) bool
}

func (s *Selector) Run() ([]*SelectorItem, error) {
	var selectedItems []*SelectorItem

	if len(s.NamesMatcher) > 0 {
		for _, item := range s.Items {
			for _, name := range s.NamesMatcher {
				if strings.Compare(strings.ToLower(name), strings.ToLower(item.Name)) == 0 {
					item.Selected = true
					selectedItems = append(selectedItems, item)
				}
			}
		}
	} else if len(s.RegexpMatchers) > 0 {
		muslRegex := regexp.MustCompile("(?i)[-_]musl[-_.]")
		foreignArchRegex := getForeignArchRegex(runtime.GOARCH)

		var ownerid, repoid string
		parts := strings.Split(s.Repository, "/")
		if len(parts) == 2 {
			ownerid = strings.ToLower(parts[0])
			repoid = strings.ToLower(parts[1])
		} else if len(parts) == 1 {
			repoid = strings.ToLower(parts[0])
		}

		lcp := getMajorityPrefix(s.Items)

		var levels []fallbackLevel
		if repoid != "" {
			levels = append(levels, fallbackLevel{"repoid", func(n string) bool { return strings.Contains(strings.ToLower(n), repoid) }})
			// Add a fallback for cases where the binary is a prefix of the repo (e.g., 'nu' for 'nushell')
			levels = append(levels, fallbackLevel{"repoid_prefix", func(n string) bool { 
				lowerN := strings.ToLower(n)
				return strings.HasPrefix(repoid, lowerN) || strings.HasPrefix(lowerN, repoid)
			}})
		}
		if lcp != "" {
			levels = append(levels, fallbackLevel{"lcp", func(n string) bool { return strings.HasPrefix(n, lcp) }})
		}
		if ownerid != "" {
			levels = append(levels, fallbackLevel{"ownerid", func(n string) bool { return strings.Contains(strings.ToLower(n), ownerid) }})
		}
		levels = append(levels, fallbackLevel{"blind", func(n string) bool { return true }})

		// Try fallback levels in order
		for _, level := range levels {
			// Try regex matchers in priority order
			for _, rx := range s.RegexpMatchers {
				compiledRx, err := regexp.Compile(rx)
				if err != nil {
					return nil, err
				}
				var currentMatches []*SelectorItem
				for _, item := range s.Items {
					if !level.filter(item.Name) {
						continue
					}
					if compiledRx.MatchString(item.Name) {
						if !s.AllowForeignArch && foreignArchRegex != nil && foreignArchRegex.MatchString(item.Name) {
							// Only apply foreign filter if the regex itself didn't explicitly ask for it
							if !foreignArchRegex.MatchString(rx) && !strings.Contains(strings.ToLower(rx), "arm") && !strings.Contains(strings.ToLower(rx), "386") {
								continue
							}
						}
						currentMatches = append(currentMatches, item)
					}
				}
				if len(currentMatches) > 0 {

					if s.Kind == Binary {
						var execMatches []*SelectorItem
						for _, item := range currentMatches {
							ext := strings.ToLower(filepath.Ext(item.Name))
							if ext == ".exe" || ext == ".appimage" || ext == ".bin" || ext == ".deb" || ext == ".rpm" || ext == ".msi" || ext == ".dmg" || ext == ".pkg" {
								execMatches = append(execMatches, item)
							} else if ext == "" {
								// Strictly filter extensionless files using magic bytes
								if IsActuallyExecutable(item) {
									execMatches = append(execMatches, item)
								}
							}
						}
						if len(execMatches) > 0 {
							currentMatches = execMatches
						}
					}
					// If multiple items match, prefer non-musl over musl on Linux/standard distros
					var nonMusl []*SelectorItem
					for _, item := range currentMatches {
						if !muslRegex.MatchString(item.Name) {
							nonMusl = append(nonMusl, item)
						}
					}
					if len(nonMusl) > 0 {
						currentMatches = nonMusl
					}

					for _, item := range currentMatches {
						item.Selected = true
						selectedItems = append(selectedItems, item)
					}
					return selectedItems, nil // Return immediately upon finding the highest priority match
				}
			}
		}
	}

	if len(selectedItems) == 0 {
		return nil, fmt.Errorf("no %s matches found for the requested criteria", s.Kind.String())
	}
	return selectedItems, nil
}

func (s *Selector) GetKind() SelectorKind {
	return s.Kind
}

func getForeignArchRegex(goarch string) *regexp.Regexp {
	var foreign []string
	switch goarch {
	case "amd64":
		foreign = []string{"arm64", "aarch64", "armhf", "armv7", "armv6", "386", "i386", "32-bit", "mips64", "ppc64le", "s390x", "riscv64"}
	case "arm64":
		foreign = []string{"amd64", "x86_64", "x64", "x86", "386", "i386", "armhf", "armv7", "armv6", "mips64", "ppc64le", "s390x", "riscv64"}
	default:
		return nil
	}
	pattern := "(?i)[-_\\.](?:" + strings.Join(foreign, "|") + ")(?:[-_\\.]|$)"
	return regexp.MustCompile(pattern)
}
