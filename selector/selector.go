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
			normalizedRepoid := strings.ReplaceAll(strings.ReplaceAll(repoid, "-", ""), "_", "")
			levels = append(levels, fallbackLevel{"repoid", func(n string) bool {
				normalizedN := strings.ReplaceAll(strings.ReplaceAll(strings.ToLower(n), "-", ""), "_", "")
				return strings.Contains(normalizedN, normalizedRepoid)
			}})
			// Add a fallback for cases where the binary is a prefix of the repo (e.g., 'nu' for 'nushell')
			levels = append(levels, fallbackLevel{"repoid_prefix", func(n string) bool {
				normalizedN := strings.ReplaceAll(strings.ReplaceAll(strings.ToLower(n), "-", ""), "_", "")
				return strings.HasPrefix(normalizedRepoid, normalizedN) || strings.HasPrefix(normalizedN, normalizedRepoid)
			}})
		}
		if lcp != "" {
			levels = append(levels, fallbackLevel{"lcp", func(n string) bool { return strings.HasPrefix(n, lcp) }})
		}
		if ownerid != "" {
			levels = append(levels, fallbackLevel{"ownerid", func(n string) bool { return strings.Contains(strings.ToLower(n), ownerid) }})
		}
		levels = append(levels, fallbackLevel{"blind", func(n string) bool { return true }})

		var strictRegexps []string
		var weakRegexps []string
		for _, rx := range s.RegexpMatchers {
			if rx == "^.*$" || strings.HasPrefix(rx, "^.*\\.(?i:") {
				weakRegexps = append(weakRegexps, rx)
			} else {
				strictRegexps = append(strictRegexps, rx)
			}
		}

		executePass := func(regexps []string) ([]*SelectorItem, error) {
			for _, level := range levels {
				for _, rx := range regexps {
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
							if s.Kind == Asset {
								lowerName := strings.ToLower(item.Name)
								if strings.Contains(lowerName, "checksum") ||
									strings.Contains(lowerName, "sha256") ||
									strings.Contains(lowerName, "sha512") ||
									strings.Contains(lowerName, "source") ||
									strings.HasSuffix(lowerName, ".txt") ||
									strings.HasSuffix(lowerName, ".md") ||
									strings.HasSuffix(lowerName, ".pem") ||
									strings.HasSuffix(lowerName, ".sig") {
									// Only allow if the regex explicitly looks for this type of file
									if !strings.Contains(strings.ToLower(rx), "txt") && 
									   !strings.Contains(strings.ToLower(rx), "checksum") &&
									   !strings.Contains(strings.ToLower(rx), "sha") &&
									   !strings.Contains(strings.ToLower(rx), "source") {
										continue
									}
								}
							}

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
								switch ext {
								case ".exe", ".appimage", ".bin", ".deb", ".rpm", ".msi", ".dmg", ".pkg":
									execMatches = append(execMatches, item)
								case "":
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
			return nil, nil
		}

		if matches, err := executePass(strictRegexps); err != nil {
			return nil, err
		} else if len(matches) > 0 {
			return matches, nil
		}

		if matches, err := executePass(weakRegexps); err != nil {
			return nil, err
		} else if len(matches) > 0 {
			return matches, nil
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
