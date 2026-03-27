package awn

import "testing"

func TestByLabel_MatchesElement(t *testing.T) {
	e := Element{Label: "Submit"}
	match := ByLabel("Submit")
	if !match(e) {
		t.Fatal("expected ByLabel to match element with same label")
	}
}

func TestByLabel_DoesNotMatchDifferentLabel(t *testing.T) {
	e := Element{Label: "Cancel"}
	match := ByLabel("Submit")
	if match(e) {
		t.Fatal("expected ByLabel not to match element with different label")
	}
}

func TestByType_MatchesElement(t *testing.T) {
	e := Element{Type: "button"}
	match := ByType("button")
	if !match(e) {
		t.Fatal("expected ByType to match element with same type")
	}
}
