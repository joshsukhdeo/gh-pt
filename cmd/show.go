package cmd

import (
	"encoding/base64"
	"fmt"
	"strings"

	"github.com/cli/go-gh/v2/pkg/api"
	"github.com/joshsukhdeo/gh-pt/state"
)

type ghRestClient interface {
	Get(path string, response interface{}) error
}

var defaultRestClient = func() (ghRestClient, error) {
	return api.DefaultRESTClient()
}

type ReleaseAsset struct {
	ID   int64  `json:"id"`
	Name string `json:"name"`
	Size int64  `json:"size"`
}

type Release struct {
	ID         int64          `json:"id"`
	TagName    string         `json:"tag_name"`
	Name       string         `json:"name"`
	Prerelease bool           `json:"prerelease"`
	Draft      bool           `json:"draft"`
	Assets     []ReleaseAsset `json:"assets"`
}

func ShowInfo(r *RootCLI) error {
	client, err := defaultRestClient()
	if err != nil {
		return fmt.Errorf("could not init GitHub REST client: %w", err)
	}
	return showInfoWithClient(r, client)
}

func showInfoWithClient(r *RootCLI, client ghRestClient) error {
	repo := r.ExecContext.Repository
	if repo == "" {
		return fmt.Errorf("repository must be provided")
	}

	var installedVersion string
	var appPinned bool
	if st, err := state.LoadState(); err == nil && st != nil && st.Apps != nil {
		if app, ok := st.Apps[repo]; ok && app != nil {
			installedVersion = app.Version
			appPinned = app.Pinned
		} else {
			for k, app := range st.Apps {
				if strings.EqualFold(k, repo) && app != nil {
					installedVersion = app.Version
					appPinned = app.Pinned
					break
				}
			}
		}
	}

	cfg := loadConfig()
	var disableIcons bool
	if cfg != nil {
		disableIcons = cfg.Core.DisableIcons
		if cfg.Core.AllowPrerelease && !r.ExecContext.Stable {
			r.ExecContext.Prerelease = true
		}
	}

	var releases []Release
	if err := client.Get("repos/"+repo+"/releases", &releases); err != nil {
		return fmt.Errorf("failed to fetch releases for %s: %w", repo, err)
	}
	if len(releases) == 0 {
		fmt.Printf("No releases found for %s\n", repo)
		return nil
	}

	var latestStable *Release
	var latestPrerelease *Release
	for i := range releases {
		if releases[i].Draft {
			continue
		}
		if !releases[i].Prerelease && latestStable == nil {
			latestStable = &releases[i]
		}
		if releases[i].Prerelease && latestPrerelease == nil {
			latestPrerelease = &releases[i]
		}
	}

	printRelease := func(rel *Release) {
		isInstalled := installedVersion != "" && (rel.TagName == installedVersion || strings.TrimPrefix(rel.TagName, "v") == strings.TrimPrefix(installedVersion, "v"))
		isPinned := isInstalled && appPinned
		isLatestP := latestPrerelease != nil && (rel == latestPrerelease || (rel.ID != 0 && rel.ID == latestPrerelease.ID))
		isLatestS := latestStable != nil && (rel == latestStable || (rel.ID != 0 && rel.ID == latestStable.ID))
		indicator := GetStateIndicator(isInstalled, isPinned, rel.Prerelease, isLatestP, isLatestS, disableIcons)
		if indicator != "" {
			fmt.Printf("%s %s\n", indicator, rel.TagName)
		} else {
			fmt.Println(rel.TagName)
		}
	}

	pickTargetRelease := func() *Release {
		if r.ExecContext.ReleaseVersion != "" && r.ExecContext.ReleaseVersion != "latest" {
			for i := range releases {
				if releases[i].TagName == r.ExecContext.ReleaseVersion ||
					strings.TrimPrefix(releases[i].TagName, "v") == strings.TrimPrefix(r.ExecContext.ReleaseVersion, "v") {
					return &releases[i]
				}
			}
			var single Release
			if err := client.Get(fmt.Sprintf("repos/%s/releases/tags/%s", repo, r.ExecContext.ReleaseVersion), &single); err == nil && single.TagName != "" {
				return &single
			}
			return nil
		}
		if r.ExecContext.Prerelease {
			if latestPrerelease != nil {
				return latestPrerelease
			}
			return latestStable
		}
		if latestStable != nil {
			return latestStable
		}
		if latestPrerelease != nil {
			return latestPrerelease
		}
		if len(releases) > 0 {
			return &releases[0]
		}
		return nil
	}

	fetchAssets := func(target *Release) ([]ReleaseAsset, error) {
		if target == nil {
			return nil, fmt.Errorf("no target release")
		}
		var assets []ReleaseAsset
		var getErr error
		if target.ID != 0 {
			getErr = client.Get(fmt.Sprintf("repos/%s/releases/%d/assets?per_page=100", repo, target.ID), &assets)
		}
		if len(assets) == 0 && len(target.Assets) > 0 {
			return target.Assets, nil
		}
		if getErr != nil && len(assets) == 0 {
			return nil, getErr
		}
		return assets, nil
	}

	showVersionsLimit := r.ExecContext.ShowVersions
	showAssetsLimit := r.ExecContext.ShowAssets
	showDescLimit := r.ExecContext.ShowDescription
	showReadmeLimit := r.ExecContext.ShowReadme

	if showVersionsLimit <= -1 && showAssetsLimit <= -1 && showDescLimit <= -1 && showReadmeLimit <= -1 {
		showVersionsLimit = 10
		showAssetsLimit = 25
		showDescLimit = 10
		showReadmeLimit = 50
	}

	order := getShowFlagOrder()
	if len(order) == 0 {
		if showVersionsLimit > -1 { order = append(order, "versions") }
		if showAssetsLimit > -1 { order = append(order, "assets") }
		if showDescLimit > -1 { order = append(order, "description") }
		if showReadmeLimit > -1 { order = append(order, "readme") }
	}

	target := pickTargetRelease()
	var assets []ReleaseAsset
	if showAssetsLimit > -1 {
		if target == nil {
			return fmt.Errorf("no release found for %s", repo)
		}
		{
			var err error
			assets, err = fetchAssets(target)
			if err != nil {
				return fmt.Errorf("failed to fetch assets: %w", err)
			}
		}
	}

	for i, section := range order {
		if i > 0 {
			fmt.Println()
		}
		switch section {
		case "versions":
			if showVersionsLimit <= -1 { continue }
			if disableIcons {
				fmt.Println("--- VERSIONS ---")
			} else {
				fmt.Println("--- 📦 VERSIONS 📦 ---")
			}
			limit := len(releases)
			if showVersionsLimit > -1 && limit > showVersionsLimit {
				limit = showVersionsLimit
			}
			for j := 0; j < limit; j++ {
				printRelease(&releases[j])
			}
			if len(releases) > limit {
				fmt.Println("...")
			}
		case "assets":
			if showAssetsLimit <= -1 { continue }
			if disableIcons {
				fmt.Println("--- ASSETS ---")
			} else {
				fmt.Println("--- 📂 ASSETS 📂 ---")
			}
			limit := len(assets)
			if showAssetsLimit > -1 && limit > showAssetsLimit {
				limit = showAssetsLimit
			}
			for j := 0; j < limit; j++ {
				fmt.Println(assets[j].Name)
			}
			if len(assets) > limit {
				fmt.Println("...")
			}
		case "description":
			if showDescLimit <= -1 { continue }
			if disableIcons {
				fmt.Println("--- DESCRIPTION ---")
			} else {
				fmt.Println("--- 📝 DESCRIPTION 📝 ---")
			}
			
			var repoInfo struct {
				Description string `json:"description"`
			}
			err := client.Get("repos/"+repo, &repoInfo)
			if err == nil && repoInfo.Description != "" {
				lines := strings.Split(repoInfo.Description, "\n")
				limit := len(lines)
				if showDescLimit > -1 && limit > showDescLimit {
					limit = showDescLimit
				}
				for j := 0; j < limit; j++ {
					fmt.Println(lines[j])
				}
				if len(lines) > limit {
					fmt.Println("...")
				}
			}
		case "readme":
			if showReadmeLimit <= -1 { continue }
			if disableIcons {
				fmt.Println("--- README ---")
			} else {
				fmt.Println("--- 📖 README 📖 ---")
			}
			
			var readme struct {
				Content string `json:"content"`
			}
			err := client.Get("repos/"+repo+"/readme", &readme)
			if err == nil && readme.Content != "" {
				decoded, decErr := base64.StdEncoding.DecodeString(strings.ReplaceAll(readme.Content, "\n", ""))
				content := readme.Content
				if decErr == nil {
					content = string(decoded)
				}
				lines := strings.Split(content, "\n")
				limit := len(lines)
				if showReadmeLimit > -1 && limit > showReadmeLimit {
					limit = showReadmeLimit
				}
				for j := 0; j < limit; j++ {
					fmt.Println(lines[j])
				}
				if len(lines) > limit {
					fmt.Println("...")
				}
			}
		}
	}
	return nil
}
