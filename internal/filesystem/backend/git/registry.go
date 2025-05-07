package git

import (
	"net/url"
	"time"

	"github.com/bornholm/corpus/internal/filesystem"
	"github.com/bornholm/corpus/internal/filesystem/backend"
	"github.com/pkg/errors"
)

func init() {
	backend.RegisterBackendFactory("git", FromDSN)
}

type Config struct {
	RepoURL      *url.URL
	PullInterval time.Duration
	Branch       string
}

func FromDSN(dsn *url.URL) (filesystem.Backend, error) {
	conf := &Config{
		RepoURL: dsn.JoinPath(),
	}

	configurations := []ConfigureFunc{
		configureRepoURL,
		configurePullInterval,
		configureBranch,
	}

	for _, configure := range configurations {
		if err := configure(conf); err != nil {
			return nil, errors.WithStack(err)
		}
	}

	backend := New(conf.RepoURL.String(), conf.Branch, conf.PullInterval)

	return backend, nil
}

type ConfigureFunc func(conf *Config) error

const (
	paramGitScheme = "gitScheme"
)

func configureRepoURL(conf *Config) error {
	gitScheme := "https"

	query := conf.RepoURL.Query()
	if query.Has(paramGitScheme) {
		gitScheme = query.Get(paramGitScheme)
		query.Del(paramGitScheme)
		conf.RepoURL.RawQuery = query.Encode()
	}

	conf.RepoURL.Scheme = gitScheme

	return nil
}

const (
	paramGitBranch = "gitBranch"
)

func configureBranch(conf *Config) error {
	branch := ""

	query := conf.RepoURL.Query()
	if query.Has(paramGitBranch) {
		branch = query.Get(paramGitBranch)
		query.Del(paramGitBranch)
		conf.RepoURL.RawQuery = query.Encode()
	}

	conf.Branch = branch

	return nil
}

const (
	paramGitPullInterval = "gitPullInterval"
)

func configurePullInterval(conf *Config) error {
	pullInterval := time.Minute * 30

	query := conf.RepoURL.Query()
	if query.Has(paramGitPullInterval) {
		rawPullInterval := query.Get(paramGitPullInterval)
		var err error
		pullInterval, err = time.ParseDuration(rawPullInterval)
		if err != nil {
			return errors.WithStack(err)
		}

		query.Del(paramGitPullInterval)
		conf.RepoURL.RawQuery = query.Encode()
	}

	conf.PullInterval = pullInterval

	return nil
}
