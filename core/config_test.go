package core

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func writeConfig(t *testing.T, content string) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "config.toml")
	require.NoError(t, os.WriteFile(path, []byte(content), 0o600))
	return path
}

func TestLoadConfigReadsProjects(t *testing.T) {
	cfg, err := LoadConfig(writeConfig(t, `
[sources.gadak]
workspace = "asf"

[projects.calcite]
github = "apache/calcite"
jira = "CALCITE"
jira_url = "https://issues.apache.org/jira"
`))
	require.NoError(t, err)
	assert.Equal(t, "asf", cfg.Sources.Gadak.Workspace)
	assert.Equal(t, map[string]Project{"calcite": {
		Name: "calcite", GitHub: "apache/calcite", Jira: "CALCITE", JiraURL: "https://issues.apache.org/jira",
	}}, cfg.Projects)
}

func TestLoadConfigRefusesAProjectWithoutJiraKey(t *testing.T) {
	_, err := LoadConfig(writeConfig(t, "[projects.calcite]\ngithub = \"apache/calcite\"\n"))
	assert.ErrorContains(t, err, "projects.calcite has no jira key")
}

func TestLoadConfigRefusesAMissingExplicitFile(t *testing.T) {
	_, err := LoadConfig(filepath.Join(t.TempDir(), "absent.toml"))
	assert.ErrorIs(t, err, os.ErrNotExist)
}

func TestProjectForKey(t *testing.T) {
	cfg := &Config{Projects: map[string]Project{
		"calcite": {Name: "calcite", GitHub: "apache/calcite", Jira: "CALCITE"},
		"avatica": {Name: "avatica", GitHub: "apache/calcite-avatica", Jira: "CALCITE", KeyPattern: `CALCITE-1\d{3}`},
	}}
	tests := []struct {
		name string
		key  string
		want Project
	}{
		{"default pattern matches the Jira prefix", "CALCITE-7377", cfg.Projects["calcite"]},
		{"custom pattern wins in name order", "CALCITE-1234", cfg.Projects["avatica"]},
		{"unconfigured prefix yields an ad-hoc project", "HIVE-42", Project{Name: "hive", Jira: "HIVE"}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := cfg.ProjectForKey(tt.key)
			require.NoError(t, err, "ProjectForKey(%q)", tt.key)
			assert.Equal(t, tt.want, got, "ProjectForKey(%q)", tt.key)
		})
	}
}

func TestProjectForKeyRefusesANonKey(t *testing.T) {
	_, err := (&Config{}).ProjectForKey("4746")
	assert.ErrorContains(t, err, `"4746" is not a Jira issue key`)
}

func TestLoadConfigRefusesAnInvalidKeyPattern(t *testing.T) {
	_, err := LoadConfig(writeConfig(t, "[projects.calcite]\njira = \"CALCITE\"\nkey_pattern = \"CALCITE-(\"\n"))
	assert.ErrorContains(t, err, "projects.calcite.key_pattern")
}

// A misspelled key would otherwise be ignored without a word.
func TestLoadConfigRefusesAnUnknownKey(t *testing.T) {
	_, err := LoadConfig(writeConfig(t, "[projects.calcite]\njira = \"CALCITE\"\njira-url = \"https://issues.apache.org/jira\"\n"))
	assert.ErrorContains(t, err, "jira-url")
}

// setHome points the user config and home directories at an empty directory.
func setHome(t *testing.T) string {
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("XDG_CONFIG_HOME", filepath.Join(home, ".config"))
	return home
}

func TestLoadConfigWithoutTheDefaultFileIsEmpty(t *testing.T) {
	setHome(t)

	cfg, err := LoadConfig("")
	require.NoError(t, err)
	assert.Equal(t, &Config{}, cfg)
}

func TestGadakDB(t *testing.T) {
	home := setHome(t)
	profiles := filepath.Join(home, ".gadak", "profiles")
	require.NoError(t, os.MkdirAll(filepath.Join(profiles, "asf"), 0o700))

	tests := []struct {
		name string
		cfg  GadakConfig
		want string
	}{
		{"db wins over workspace", GadakConfig{DB: "~/mirror/gadak.db", Workspace: "other"}, filepath.Join(home, "mirror", "gadak.db")},
		{"workspace names a profile", GadakConfig{Workspace: "other"}, filepath.Join(profiles, "other", "gadak.db")},
		{"the only profile is used", GadakConfig{}, filepath.Join(profiles, "asf", "gadak.db")},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := (&Config{Sources: Sources{Gadak: tt.cfg}}).GadakDB()
			require.NoError(t, err)
			assert.Equal(t, tt.want, got, "GadakDB() with %+v", tt.cfg)
		})
	}
}

func TestGadakDBRefusesToGuessBetweenWorkspaces(t *testing.T) {
	home := setHome(t)
	for _, w := range []string{"asf", "work"} {
		require.NoError(t, os.MkdirAll(filepath.Join(home, ".gadak", "profiles", w), 0o700))
	}

	_, err := (&Config{}).GadakDB()
	assert.ErrorContains(t, err, "found 2 gadak workspaces")
	assert.ErrorContains(t, err, "asf, work")
}
