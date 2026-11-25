package api

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"maragu.dev/gomponents"
	"maragu.dev/gomponents/html"
)

func changelogCommits(n int) []ghCommit {
	out := make([]ghCommit, n)
	for i := range out {
		out[i].SHA = fmt.Sprintf("%07x%033d", 0xabc0000+i, 0)
		out[i].Commit.Committer.Date = time.Date(2026, 11, 20-i, 12, 0, 0, 0, time.Local)
	}

	return out
}

func TestInBuild(t *testing.T) {
	commits := changelogCommits(4)

	in, by := inBuild(commits, commits[2].SHA[:7], "", false)
	if by != "hash" || in[0] || in[1] || !in[2] || !in[3] {
		t.Fatalf("by hash: %v %s", in, by)
	}

	if in, _ := inBuild(commits, commits[0].SHA[:7], "", true); !in[0] {
		t.Fatalf("build on an earlier page: %v", in)
	}

	in, by = inBuild(commits, "unknown", "2026-11-19", false)
	if by != "date" || in[0] || !in[1] || !in[2] {
		t.Fatalf("by date: %v %s", in, by)
	}

	if in, by := inBuild(commits, "", "", false); in != nil || by != "" {
		t.Fatalf("nothing known: %v %s", in, by)
	}
}

func TestChangelogPage(t *testing.T) {
	commits := changelogCommits(3)
	commits[0].Commit.Message = "Newest change\n\nLonger <b>text</b>"
	commits[1].Commit.Message = "The built one"
	commits[2].Commit.Message = "Older one"

	calls := 0
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls++
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprint(w, `[`)
		for i, c := range commits {
			if i > 0 {
				fmt.Fprint(w, ",")
			}
			fmt.Fprintf(w, `{"sha":%q,"html_url":"https://example.invalid/%s","commit":{"message":%q,"author":{"name":"dev","date":%q},"committer":{"date":%q}}}`,
				c.SHA, c.SHA, c.Commit.Message, c.Commit.Committer.Date.Format(time.RFC3339), c.Commit.Committer.Date.Format(time.RFC3339))
		}
		fmt.Fprint(w, `]`)
	}))
	defer srv.Close()

	oldURL, oldHash, oldVer, oldDate := changelogURL, buildHash, buildVersion, buildDate
	t.Cleanup(func() {
		changelogURL = oldURL
		SetBuildInfo(oldVer, oldHash, oldDate)
		changelogMu.Lock()
		changelogCache = map[int]changelogPage{}
		changelogMu.Unlock()
	})

	changelogURL = srv.URL
	changelogMu.Lock()
	changelogCache = map[int]changelogPage{}
	changelogMu.Unlock()
	SetBuildInfo("v1.2.3", commits[1].SHA[:7], "2026-11-19")

	got, err := fetchChangelog(context.Background(), 1)
	if err != nil || len(got) != 3 || got[0].Commit.Message != commits[0].Commit.Message {
		t.Fatalf("fetch: %v %+v", err, got)
	}

	if _, err := fetchChangelog(context.Background(), 1); err != nil || calls != 1 {
		t.Fatalf("cache: %v, %d calls", err, calls)
	}

	var b strings.Builder
	if err := renderChangelogPage(got, nil, 1, buildPage(context.Background(), 5)).Render(&b); err != nil {
		t.Fatal(err)
	}

	page := b.String()
	for _, want := range []string{"v1.2.3", "This build", "Not in this build", "Newest change", "&lt;b&gt;text&lt;/b&gt;", "1 newer commits"} {
		if !strings.Contains(page, want) {
			t.Errorf("page lacks %q", want)
		}
	}

	if strings.Count(page, "Not in this build") != 1 {
		t.Errorf("only the newest commit is missing from the build")
	}
}

// TestCommitListDays: the first page shows the newest days, a button the
// older ones and the pager.
func TestCommitListDays(t *testing.T) {
	commits := changelogCommits(9) // one a day, the last two on the same day
	commits[8].Commit.Committer.Date = commits[7].Commit.Committer.Date

	var b strings.Builder
	if err := renderCommitList(commits, nil, changelogFirstDays, html.Nav(gomponents.Text("pager"))).Render(&b); err != nil {
		t.Fatal(err)
	}

	out := b.String()
	shown, rest, ok := strings.Cut(out, "Show older commits (4 more)")
	if !ok || strings.Count(shown, "card-header") != 5 || strings.Count(rest, "card-header") != 3 || !strings.Contains(rest, "pager") {
		t.Errorf("days:\n%s", out)
	}

	b.Reset()
	if err := renderCommitList(commits, nil, 0, html.Nav(gomponents.Text("pager"))).Render(&b); err != nil || strings.Contains(b.String(), "Show older") {
		t.Errorf("all days: %v %s", err, b.String())
	}
}

func TestBuildSummary(t *testing.T) {
	oldHash, oldVer, oldDate := buildHash, buildVersion, buildDate
	t.Cleanup(func() { SetBuildInfo(oldVer, oldHash, oldDate) })

	SetBuildInfo("v1.2.3", "4086c4a9f", "2026-11-21")
	if got := buildSummary(); got != "v1.2.3 · 4086c4a · built 2026-11-21" {
		t.Errorf("summary: %q", got)
	}

	SetBuildInfo("prod-2026-11-21", "unknown", "2026-11-21")
	if got := buildSummary(); got != "prod-2026-11-21 · built 2026-11-21" {
		t.Errorf("without hash: %q", got)
	}

	SetBuildInfo("", "", "")
	if got := buildSummary(); got != "development build" {
		t.Errorf("nothing set: %q", got)
	}

	var admin, user strings.Builder
	_ = renderBuildFooter(true).Render(&admin)
	_ = renderBuildFooter(false).Render(&user)

	if !strings.Contains(admin.String(), "/api/admin/changelog") || strings.Contains(user.String(), "/api/admin/changelog") {
		t.Errorf("changelog link only for admins: %s | %s", admin.String(), user.String())
	}
}
