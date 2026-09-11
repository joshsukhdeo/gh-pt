package cmd

import (
	"bufio"
	"bytes"
	"context"
	"fmt"
	"os/exec"
	"sort"
	"strings"
	"time"

	"github.com/joshsukhdeo/gh-pt/state"
	"github.com/posener/complete"
)

// ConfigKeys holds the hardcoded configuration setting keys corresponding to config.yml
var ConfigKeys = []string{
	"add_deps",
	"ai_cmd",
	"ai_interactive_cmd",
	"allow_prerelease",
	"clone_path",
	"disable_icons",
	"disable_prompts",
	"extractor",
	"fork_path",
	"global_path",
	"install_path",
	"install_types",
	"keep_suffixes",
	"log_to_file",
	"no_deps",
	"no_save_state",
	"vt_api_key",
	"wine",
}

func predictInstalledApps(args complete.Args) []string {
	st, err := state.LoadState()
	if err != nil || st == nil || st.Apps == nil {
		return []string{}
	}
	keys := make([]string, 0, len(st.Apps))
	for repo := range st.Apps {
		if repo != "" {
			keys = append(keys, repo)
		}
	}
	sort.Strings(keys)
	return keys
}

func predictGithubRepos(args complete.Args) []string {
	if !strings.Contains(args.Last, "/") {
		return []string{}
	}
	parts := strings.Split(args.Last, "/")
	owner := parts[0]
	if owner == "" {
		return []string{}
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	cmd := exec.CommandContext(ctx, "gh", "api", fmt.Sprintf("users/%s/repos", owner), "-q", ".[].full_name")
	out, err := cmd.Output()
	if err != nil {
		return []string{}
	}

	scanner := bufio.NewScanner(bytes.NewReader(out))
	var repos []string
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line != "" {
			repos = append(repos, line)
		}
	}
	return repos
}

func predictConfigKeys(args complete.Args) []string {
	return ConfigKeys
}

var (
	PredictInstalledApps complete.Predictor = complete.PredictFunc(predictInstalledApps)
	PredictGithubRepos   complete.Predictor = complete.PredictFunc(predictGithubRepos)
	PredictConfigKeys    complete.Predictor = complete.PredictFunc(predictConfigKeys)

	InstalledAppsPredictor = PredictInstalledApps
	GithubReposPredictor   = PredictGithubRepos
	ConfigKeysPredictor    = PredictConfigKeys
)

func NewInstalledAppsPredictor() complete.Predictor { return PredictInstalledApps }
func NewGithubReposPredictor() complete.Predictor   { return PredictGithubRepos }
func NewConfigKeysPredictor() complete.Predictor    { return PredictConfigKeys }
