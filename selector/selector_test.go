package selector

import (
	"bytes"
	"encoding/json"
	"io"
	"net/http"
	"runtime"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type MockGithubClient struct {
	GetResponses map[string]interface{}
	ReqResponses map[string]*http.Response
	GetError     error
	ReqError     error
}

func (m *MockGithubClient) Get(path string, response interface{}) error {
	if m.GetError != nil {
		return m.GetError
	}
	if val, ok := m.GetResponses[path]; ok {
		b, _ := json.Marshal(val)
		json.Unmarshal(b, response)
	}
	return nil
}

func (m *MockGithubClient) Request(method string, path string, body io.Reader) (*http.Response, error) {
	if m.ReqError != nil {
		return nil, m.ReqError
	}
	if resp, ok := m.ReqResponses[path]; ok {
		return resp, nil
	}
	return &http.Response{Body: io.NopCloser(bytes.NewReader([]byte(`[]`)))}, nil
}

func TestSelector_Run(t *testing.T) {
	s := &Selector{
		Kind: Release,
		Items: []*SelectorItem{
			{Name: "v1.0.0"},
			{Name: "v2.0.0"},
		},
		RegexpMatchers: []string{"v2.0.0"},
	}
	assert.Equal(t, Release, s.GetKind())

	res, err := s.Run()
	require.NoError(t, err)
	assert.Len(t, res, 1)
	assert.Equal(t, "v2.0.0", res[0].Name)

	s.RegexpMatchers = []string{}
	s.NamesMatcher = []string{"v1.0.0"}
	res, err = s.Run()
	require.NoError(t, err)
	assert.Len(t, res, 1)
	assert.Equal(t, "v1.0.0", res[0].Name)

	s.NamesMatcher = []string{"missing"}
	_, err = s.Run()
	assert.Error(t, err)
}

func TestReleaseSelector(t *testing.T) {
	client := &MockGithubClient{
		GetResponses: map[string]interface{}{
			"repos/owner/repo/releases": []map[string]interface{}{
				{"Tag_name": "v1.0.0", "Id": 1},
				{"Tag_name": "v2.0.0", "Id": 2},
			},
			"repos/owner/repo/releases/latest": map[string]interface{}{
				"Tag_name": "v2.0.0",
			},
		},
	}

	t.Run("NonInteractiveLatest", func(t *testing.T) {
		sel, err := ReleaseSelector(client, "owner/repo", "latest", false)
		require.NoError(t, err)
		assert.Equal(t, Release, sel.GetKind())
		s, ok := sel.(*Selector)
		require.True(t, ok)
		assert.Equal(t, []string{"v2.0.0"}, s.RegexpMatchers)
		var names []string
		for _, it := range s.Items {
			names = append(names, it.Name)
		}
		assert.Contains(t, names, "v2.0.0")
	})
}

func TestAssetSelector(t *testing.T) {
	respBody := `[{"Name": "asset-linux-amd64.tar.gz"}]`
	client := &MockGithubClient{}
	client.ReqResponses = map[string]*http.Response{
		"repos/owner/repo/releases/1/assets": {
			Body:   io.NopCloser(bytes.NewReader([]byte(respBody))),
			Header: http.Header{},
		},
	}

	t.Run("NonInteractive", func(t *testing.T) {
		sel, err := AssetSelector(client, "owner/repo", AssetMatchCriteria{
			ReleaseId:   1,
			Name:        "asset-linux",
			Regexps:     []string{".*linux.*"},
			Interactive: false,
		})
		require.NoError(t, err)
		assert.Equal(t, Asset, sel.GetKind())
		s, ok := sel.(*Selector)
		require.True(t, ok)
		assert.Equal(t, []string{"asset-linux"}, s.NamesMatcher)
		var names []string
		for _, it := range s.Items {
			names = append(names, it.Name)
		}
		assert.Contains(t, names, "asset-linux-amd64.tar.gz")
	})
}

func TestSelector_PrioritizesNonMusl(t *testing.T) {
	items := []*SelectorItem{
		{Name: "app-linux-musl-x64.tar.gz"},
		{Name: "app-linux-x64.tar.gz"},
	}

	sel := &Selector{
		Kind:           Asset,
		Items:          items,
		RegexpMatchers: []string{`.*(?:amd64|x86_64|x64).*\.(?i:tar\.gz)$`},
		Single:         true,
	}

	selected, err := sel.Run()
	assert.NoError(t, err)
	assert.Len(t, selected, 1)
	assert.Equal(t, "app-linux-x64.tar.gz", selected[0].Name)
}

func TestSelector_ForeignArchitectureBlacklisting(t *testing.T) {
	items := []*SelectorItem{
		{Name: "app_linux_arm64.tar.gz"},
		{Name: "app_linux_amd64.tar.gz"},
	}

	sel := &Selector{
		Kind:           Asset,
		Items:          items,
		RegexpMatchers: []string{`(?i)app_linux.*\.tar\.gz$`},
		Single:         true,
	}

	// This validates the architecture exclusion logic
	// e.g. "app_linux_arm64.tar.gz" is rejected on amd64
	// "app_linux_amd64.tar.gz" is selected on amd64
	selected, err := sel.Run()
	assert.NoError(t, err)
	assert.Len(t, selected, 1)

	if runtime.GOARCH == "amd64" {
		assert.Equal(t, "app_linux_amd64.tar.gz", selected[0].Name)
	} else if runtime.GOARCH == "arm64" {
		assert.Equal(t, "app_linux_arm64.tar.gz", selected[0].Name)
	}
}

func TestSelector_NoArchitecture_AssumeCompatible(t *testing.T) {
	reAmd64 := getForeignArchRegex("amd64")
	if reAmd64 != nil {
		assert.False(t, reAmd64.MatchString("app_linux.tar.gz"))
	}
}

func TestSelector_FinalFallbackPattern(t *testing.T) {
	items := []*SelectorItem{
		{Name: "app.deb"},
	}

	sel := &Selector{
		Kind:           Asset,
		Items:          items,
		RegexpMatchers: []string{`(?i)^app\.deb$`},
	}

	selected, err := sel.Run()
	assert.NoError(t, err)
	assert.Len(t, selected, 1)
	assert.Equal(t, "app.deb", selected[0].Name)
}

func TestInteractiveSelector_GetKind(t *testing.T) {
	sel := &InteractiveSelector{Kind: Asset}
	assert.Equal(t, Asset, sel.GetKind())
}

func TestSelectorKind_String(t *testing.T) {
	assert.Equal(t, "release_selector", Release.String())
	assert.Equal(t, "asset_selector", Asset.String())
	assert.Equal(t, "binary_selector", Binary.String())
	assert.Equal(t, "Unknown(99)", SelectorKind(99).String())
}

func TestItem(t *testing.T) {
	assert.Equal(t, BinaryExecutable, BinaryTypeFromPath("/tmp/test.exe"))
	assert.Equal(t, BinaryExecutable, BinaryTypeFromPath("/tmp/test"))
	assert.Equal(t, BinaryDebInstaller, BinaryTypeFromPath("/tmp/test.deb"))
	assert.Equal(t, BinaryRpmInstaller, BinaryTypeFromPath("/tmp/test.rpm"))
	assert.Equal(t, BinaryPkgInstaller, BinaryTypeFromPath("/tmp/test.pkg"))
	assert.Equal(t, BinaryPkgInstaller, BinaryTypeFromPath("/tmp/test.txz"))
}
