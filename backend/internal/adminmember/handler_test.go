package adminmember

import "testing"

func TestNormalizeAndValidateTags(t *testing.T) {
	tags := normalizeTags([]string{" 新客 ", "高频", "新客", ""})
	if len(tags) != 2 || tags[0] != "新客" || tags[1] != "高频" {
		t.Fatalf("unexpected tags: %#v", tags)
	}
	input := updateInput{Status: "ACTIVE", Tags: tags, Version: 1}
	if message := validateUpdate(input); message != "" {
		t.Fatalf("valid update rejected: %s", message)
	}
}

func TestEscapeLike(t *testing.T) {
	if got := escapeLike("A_10%!"); got != "A!_10!%!!" {
		t.Fatalf("escapeLike = %q", got)
	}
}
