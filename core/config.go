package core

import (
	"bytes"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"regexp"
	"slices"
	"strings"

	"github.com/pelletier/go-toml/v2"
)

// Config is the content of issuecrawl's config.toml.
//
// Every field is optional. Without a file, the mirrors are looked up at the
// paths gitcrawl and gadak use by default, and a project is derived from the
// Jira key that a command is given (see [Config.ProjectForKey]).
type Config struct {
	Sources  Sources            `toml:"sources"`
	Projects map[string]Project `toml:"projects"`
}

// Sources locates the mirror databases.
type Sources struct {
	Gitcrawl GitcrawlConfig `toml:"gitcrawl"`
	Gadak    GadakConfig    `toml:"gadak"`
}

// GitcrawlConfig locates the gitcrawl database.
type GitcrawlConfig struct {
	// DB is the path to gitcrawl.db. The default is gitcrawl's own default,
	// gitcrawl/gitcrawl.db under [os.UserConfigDir].
	DB string `toml:"db"`
}

// GadakConfig locates the gadak database.
type GadakConfig struct {
	// DB is the path to gadak.db. It takes precedence over Workspace.
	DB string `toml:"db"`
	// Workspace selects ~/.gadak/profiles/<Workspace>/gadak.db. When both DB
	// and Workspace are empty and ~/.gadak/profiles holds exactly one
	// workspace, that one is used.
	Workspace string `toml:"workspace"`
}

// Project maps one logical project to its place in each tracker.
type Project struct {
	// Name is the key of the [projects.<name>] table; it is not read from
	// the table itself.
	Name string `toml:"-"`
	// GitHub is the "owner/name" of the repository. When empty, a search
	// covers every repository the gitcrawl mirror holds.
	GitHub string `toml:"github"`
	// Jira is the Jira project key, such as CALCITE.
	Jira string `toml:"jira"`
	// JiraURL is the base URL of the Jira server, used to build issue links
	// when the mirror stores none.
	JiraURL string `toml:"jira_url"`
	// KeyPattern is a regular expression that an issue key of this project
	// matches in full. The default is <Jira>-<digits>, case-insensitive.
	KeyPattern string `toml:"key_pattern"`
}

// DefaultConfigPath returns issuecrawl/config.toml under [os.UserConfigDir].
func DefaultConfigPath() (string, error) {
	dir, err := os.UserConfigDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(dir, "issuecrawl", "config.toml"), nil
}

// LoadConfig reads the config file at path. An empty path reads
// [DefaultConfigPath] and yields an empty Config when that file does not
// exist; an explicit path that does not exist is an error.
func LoadConfig(path string) (*Config, error) {
	explicit := path != ""
	if !explicit {
		var err error
		if path, err = DefaultConfigPath(); err != nil {
			return &Config{}, nil
		}
	}
	data, err := os.ReadFile(path)
	if errors.Is(err, fs.ErrNotExist) && !explicit {
		return &Config{}, nil
	}
	if err != nil {
		return nil, fmt.Errorf("read config: %w", err)
	}
	var c Config
	dec := toml.NewDecoder(bytes.NewReader(data))
	dec.DisallowUnknownFields()
	if err := dec.Decode(&c); err != nil {
		// The strict-mode error names the unknown keys only in String().
		var strict *toml.StrictMissingError
		if errors.As(err, &strict) {
			return nil, fmt.Errorf("config %s has unknown keys:\n%s", path, strict.String())
		}
		return nil, fmt.Errorf("parse config %s: %w", path, err)
	}
	for name, p := range c.Projects {
		p.Name = name
		if p.Jira == "" {
			return nil, fmt.Errorf("config %s: projects.%s has no jira key", path, name)
		}
		if _, err := p.keyRegexp(); err != nil {
			return nil, fmt.Errorf("config %s: projects.%s.key_pattern: %w", path, name, err)
		}
		c.Projects[name] = p
	}
	return &c, nil
}

func (p Project) keyRegexp() (*regexp.Regexp, error) {
	pattern := p.KeyPattern
	if pattern == "" {
		pattern = `(?i)` + regexp.QuoteMeta(p.Jira) + `-\d+`
	}
	return regexp.Compile(`^(?:` + pattern + `)$`)
}

// ProjectForKey returns the configured project whose key pattern matches the
// Jira key in full, checking projects in name order. When none matches, it
// returns an ad-hoc project named after the key's prefix, with no GitHub
// repository.
func (c *Config) ProjectForKey(key string) (Project, error) {
	names := make([]string, 0, len(c.Projects))
	for name := range c.Projects {
		names = append(names, name)
	}
	slices.Sort(names)
	for _, name := range names {
		p := c.Projects[name]
		re, err := p.keyRegexp()
		if err != nil {
			return Project{}, err
		}
		if re.MatchString(key) {
			return p, nil
		}
	}
	prefix, num, ok := strings.Cut(key, "-")
	if !ok || prefix == "" || num == "" || strings.Trim(num, "0123456789") != "" {
		return Project{}, fmt.Errorf("%q is not a Jira issue key such as CALCITE-1234", key)
	}
	return Project{Name: strings.ToLower(prefix), Jira: strings.ToUpper(prefix)}, nil
}

// GitcrawlDB returns the path of the gitcrawl database.
func (c *Config) GitcrawlDB() (string, error) {
	if c.Sources.Gitcrawl.DB != "" {
		return expandHome(c.Sources.Gitcrawl.DB)
	}
	dir, err := os.UserConfigDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(dir, "gitcrawl", "gitcrawl.db"), nil
}

// GadakDB returns the path of the gadak database; see [GadakConfig] for the
// lookup order.
func (c *Config) GadakDB() (string, error) {
	g := c.Sources.Gadak
	if g.DB != "" {
		return expandHome(g.DB)
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	profiles := filepath.Join(home, ".gadak", "profiles")
	if g.Workspace != "" {
		return filepath.Join(profiles, g.Workspace, "gadak.db"), nil
	}
	entries, err := os.ReadDir(profiles)
	if err != nil {
		return "", fmt.Errorf("find gadak workspace: %w; set sources.gadak.workspace or sources.gadak.db in the config", err)
	}
	var workspaces []string
	for _, e := range entries {
		if e.IsDir() {
			workspaces = append(workspaces, e.Name())
		}
	}
	if len(workspaces) != 1 {
		return "", fmt.Errorf("found %d gadak workspaces in %s (%s); set sources.gadak.workspace in the config",
			len(workspaces), profiles, strings.Join(workspaces, ", "))
	}
	return filepath.Join(profiles, workspaces[0], "gadak.db"), nil
}

func expandHome(path string) (string, error) {
	if path != "~" && !strings.HasPrefix(path, "~/") {
		return path, nil
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(home, path[1:]), nil
}
