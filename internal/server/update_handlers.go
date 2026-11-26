package server

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"time"

	"github.com/lithium/lithium/internal/version"
)

// The update check asks GitHub which commits on the release branch the
// running build doesn't have yet. Lithium can't update itself (its container
// has no git and no access to Docker); the admin runs ./install.sh update on
// the server, and this page tells them when that's worth doing.

const (
	updateCacheFor      = 15 * time.Minute // GitHub allows 60 unauthenticated requests an hour
	updateErrorCacheFor = time.Minute
	updateRefreshGap    = 10 * time.Second // "Check now" can't hammer GitHub
	updateMaxCommits    = 20
)

type updateCommit struct {
	SHA     string    `json:"sha"`
	Message string    `json:"message"` // first line only
	Date    time.Time `json:"date"`
	URL     string    `json:"url"`
}

type updateStatus struct {
	// Status is "current" (up to date), "behind" (updates available),
	// "unknown" (this build's commit isn't known), "unrecognised" (GitHub
	// doesn't have this build's commit, or it's newer than the branch) or
	// "error" (GitHub couldn't be reached).
	Status    string         `json:"status"`
	Current   string         `json:"current"`
	Latest    *updateCommit  `json:"latest,omitempty"`
	Behind    int            `json:"behind"`
	Commits   []updateCommit `json:"commits"` // newest first, at most updateMaxCommits
	Error     string         `json:"error,omitempty"`
	CheckedAt time.Time      `json:"checkedAt"`
	RepoURL   string         `json:"repoUrl"`
	Branch    string         `json:"branch"`
}

type updateChecker struct {
	api    string // GitHub API base URL; tests point it at a fake
	commit string // the running build's commit
	client *http.Client

	mu      sync.Mutex
	cached  *updateStatus
	expires time.Time
}

func newUpdateChecker() *updateChecker {
	return &updateChecker{
		api:    "https://api.github.com",
		commit: version.Commit(),
		client: &http.Client{Timeout: 10 * time.Second},
	}
}

func (s *Server) handleCheckUpdates(w http.ResponseWriter, r *http.Request) {
	refresh := r.URL.Query().Get("refresh") == "1"
	writeJSON(w, http.StatusOK, s.updates.status(r.Context(), refresh))
}

func (c *updateChecker) status(ctx context.Context, refresh bool) *updateStatus {
	c.mu.Lock()
	defer c.mu.Unlock()
	now := time.Now()
	if c.cached != nil {
		fresh := now.Before(c.expires)
		if (fresh && !refresh) || now.Sub(c.cached.CheckedAt) < updateRefreshGap {
			return c.cached
		}
	}
	st := c.check(ctx)
	st.CheckedAt = now
	c.cached = st
	if st.Status == "error" {
		c.expires = now.Add(updateErrorCacheFor)
	} else {
		c.expires = now.Add(updateCacheFor)
	}
	return st
}

func (c *updateChecker) check(ctx context.Context) *updateStatus {
	st := &updateStatus{
		Current: c.commit,
		Commits: []updateCommit{},
		RepoURL: "https://github.com/" + version.Repo,
		Branch:  version.Branch,
	}
	fail := func(err error) *updateStatus {
		st.Status = "error"
		st.Error = err.Error()
		return st
	}

	var head ghCommit
	if _, err := c.get(ctx, "/repos/"+version.Repo+"/commits/"+url.PathEscape(version.Branch), &head); err != nil {
		return fail(err)
	}
	latest := head.toUpdate()
	st.Latest = &latest

	switch {
	case c.commit == "":
		st.Status = "unknown"
		return st
	case c.commit == head.SHA:
		st.Status = "current"
		return st
	}

	var cmp struct {
		Status   string     `json:"status"`   // of the branch relative to our commit
		AheadBy  int        `json:"ahead_by"` // commits on the branch we don't have
		BehindBy int        `json:"behind_by"`
		Commits  []ghCommit `json:"commits"` // oldest first
	}
	code, err := c.get(ctx, "/repos/"+version.Repo+"/compare/"+url.PathEscape(c.commit)+"..."+url.PathEscape(version.Branch), &cmp)
	if code == http.StatusNotFound {
		st.Status = "unrecognised"
		return st
	}
	if err != nil {
		return fail(err)
	}
	switch cmp.Status {
	case "identical":
		st.Status = "current"
	case "ahead":
		st.Status = "behind"
	default: // "behind" or "diverged": this build has commits GitHub doesn't
		st.Status = "unrecognised"
	}
	st.Behind = cmp.AheadBy
	for i := len(cmp.Commits) - 1; i >= 0 && len(st.Commits) < updateMaxCommits; i-- {
		st.Commits = append(st.Commits, cmp.Commits[i].toUpdate())
	}
	return st
}

type ghCommit struct {
	SHA     string `json:"sha"`
	HTMLURL string `json:"html_url"`
	Commit  struct {
		Message   string `json:"message"`
		Committer struct {
			Date time.Time `json:"date"`
		} `json:"committer"`
	} `json:"commit"`
}

func (g ghCommit) toUpdate() updateCommit {
	msg, _, _ := strings.Cut(g.Commit.Message, "\n")
	return updateCommit{SHA: g.SHA, Message: strings.TrimSpace(msg), Date: g.Commit.Committer.Date, URL: g.HTMLURL}
}

// get fetches path from the GitHub API into v, returning the HTTP status.
func (c *updateChecker) get(ctx context.Context, path string, v any) (int, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, c.api+path, nil)
	if err != nil {
		return 0, err
	}
	req.Header.Set("Accept", "application/vnd.github+json")
	req.Header.Set("User-Agent", "Lithium-update-check")
	resp, err := c.client.Do(req)
	if err != nil {
		return 0, errors.New("couldn't reach GitHub")
	}
	defer resp.Body.Close()
	switch {
	case resp.StatusCode == http.StatusForbidden || resp.StatusCode == http.StatusTooManyRequests:
		return resp.StatusCode, errors.New("GitHub's rate limit was reached; try again later")
	case resp.StatusCode != http.StatusOK:
		return resp.StatusCode, fmt.Errorf("GitHub answered %d", resp.StatusCode)
	}
	if err := json.NewDecoder(resp.Body).Decode(v); err != nil {
		return resp.StatusCode, errors.New("couldn't read GitHub's answer")
	}
	return resp.StatusCode, nil
}
