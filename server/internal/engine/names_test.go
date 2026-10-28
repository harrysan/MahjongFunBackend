package engine

import (
	"encoding/json"
	"strings"
	"testing"
)

func TestJSONNames(t *testing.T) {
	g := NewGame(1, 0)
	b, err := json.Marshal(g.ViewFor(0))
	if err != nil {
		t.Fatal(err)
	}
	s := string(b)
	for _, want := range []string{`"phase":"Discarding"`, `"type":"discard"`} {
		if !strings.Contains(s, want) {
			t.Errorf("view JSON missing %s", want)
		}
	}

	b, _ = json.Marshal(Meld{Type: MeldConcealedKong})
	if !strings.Contains(string(b), `"type":"concealed_kong"`) {
		t.Errorf("meld JSON = %s", b)
	}
}

func TestParseActionType(t *testing.T) {
	for a := ActionDiscard; a <= ActionPass; a++ {
		got, ok := ParseActionType(a.String())
		if !ok || got != a {
			t.Errorf("ParseActionType(%q) = %v, %v", a.String(), got, ok)
		}
	}
	if _, ok := ParseActionType("ron"); ok {
		t.Error("unknown name accepted")
	}
	if _, ok := ParseActionType(""); ok {
		t.Error("empty name accepted")
	}
}
