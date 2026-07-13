package main

import "testing"

func TestPRURL(t *testing.T) {
	cases := []struct{ remote, branch, want string }{
		{"git@github.com:osman-yahya/git2.git", "feature/x",
			"https://github.com/osman-yahya/git2/compare/feature/x?expand=1"},
		{"https://github.com/osman-yahya/git2.git", "main",
			"https://github.com/osman-yahya/git2/compare/main?expand=1"},
		{"https://gitlab.com/grp/proj.git", "fix",
			"https://gitlab.com/grp/proj/-/merge_requests/new?merge_request%5Bsource_branch%5D=fix"},
		{"git@bitbucket.org:team/repo.git", "dev",
			"https://bitbucket.org/team/repo/pull-requests/new?source=dev"},
		{"https://git.company.io/team/repo.git", "b",
			"https://git.company.io/team/repo"},
	}
	for _, c := range cases {
		if got := prURL(c.remote, c.branch); got != c.want {
			t.Errorf("prURL(%q, %q) = %q, want %q", c.remote, c.branch, got, c.want)
		}
	}
}

func TestBuildGraphLinear(t *testing.T) {
	commits := []Commit{
		{Hash: "c", Parents: []string{"b"}},
		{Hash: "b", Parents: []string{"a"}},
		{Hash: "a"},
	}
	rows := BuildGraph(commits)
	if len(rows) != 3 {
		t.Fatalf("want 3 rows, got %d", len(rows))
	}
	for i, r := range rows {
		if len(r.Cells) != 1 || r.Cells[0].Ch != '●' {
			t.Errorf("row %d: want single ● cell, got %+v", i, r.Cells)
		}
	}
}

func TestBuildGraphMerge(t *testing.T) {
	// merge commit M with parents a (lane 0) and f (feature lane)
	commits := []Commit{
		{Hash: "M", Parents: []string{"a", "f"}},
		{Hash: "f", Parents: []string{"a"}},
		{Hash: "a"},
	}
	rows := BuildGraph(commits)
	if rows[0].Cells[0].Ch != '○' {
		t.Errorf("merge commit should render ○, got %q", rows[0].Cells[0].Ch)
	}
	if len(rows[0].Cells) < 2 || rows[0].Cells[1].Ch != '╮' {
		t.Errorf("merge should open a second lane with ╮, got %+v", rows[0].Cells)
	}
	if rows[1].Cells[1].Ch != '●' {
		t.Errorf("feature commit should sit on lane 1, got %+v", rows[1].Cells)
	}
	// root: both lanes fold into one
	if rows[2].Cells[0].Ch != '●' || rows[2].Cells[1].Ch != '╯' {
		t.Errorf("root should join lanes (● ╯), got %+v", rows[2].Cells)
	}
}

func TestBuildGraphNoPanicOnRoot(t *testing.T) {
	rows := BuildGraph([]Commit{{Hash: "only"}})
	if len(rows) != 1 {
		t.Fatalf("want 1 row, got %d", len(rows))
	}
}

func TestParseRefsSlashBranches(t *testing.T) {
	remotes := []string{"origin"}
	refs := parseRefs("HEAD -> dev/main, origin/main, dev/feature, tag: v1.0", remotes)
	want := map[string]RefKind{
		"dev/main": RefHead, "origin/main": RefRemote,
		"dev/feature": RefBranch, "v1.0": RefTag,
	}
	for _, r := range refs {
		if k, ok := want[r.Name]; !ok || k != r.Kind {
			t.Errorf("ref %q classified as %v, want %v", r.Name, r.Kind, want[r.Name])
		}
	}
	if len(refs) != len(want) {
		t.Errorf("got %d refs, want %d", len(refs), len(want))
	}
}
