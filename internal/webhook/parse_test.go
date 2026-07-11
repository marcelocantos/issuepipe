// Copyright 2026 Marcelo Cantos
// SPDX-License-Identifier: Apache-2.0

package webhook

import "testing"

func TestParseIssuesEvent(t *testing.T) {
	body := []byte(`{
	  "action": "edited",
	  "issue": {
	    "node_id": "I_kw1",
	    "number": 7,
	    "title": "T",
	    "body": null,
	    "state": "closed",
	    "html_url": "https://example/issues/7",
	    "updated_at": "t",
	    "user": {"login": "bob"},
	    "labels": [{"name": "a"}, {"name": "b"}]
	  },
	  "repository": {"id": 99, "node_id": "R_99"}
	}`)
	issue, err := ParseIssuesEvent(body)
	if err != nil {
		t.Fatal(err)
	}
	if issue == nil {
		t.Fatal("expected issue")
	}
	if issue.IssueNodeID != "I_kw1" || issue.RepoID != 99 || issue.Number != 7 {
		t.Fatalf("%+v", issue)
	}
	if issue.Body != "" {
		t.Fatalf("null body should become empty, got %q", issue.Body)
	}
	if issue.LabelsJSON != `["a","b"]` {
		t.Fatalf("labels = %s", issue.LabelsJSON)
	}
}

func TestParseIssuesEventSkipsIncomplete(t *testing.T) {
	issue, err := ParseIssuesEvent([]byte(`{"action":"opened"}`))
	if err != nil {
		t.Fatal(err)
	}
	if issue != nil {
		t.Fatal("expected nil for incomplete payload")
	}
}
