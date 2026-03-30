package cli

import (
	"testing"
)

func TestParseTelegramAllowlist(t *testing.T) {
	ids, names, err := parseTelegramAllowlist("")
	if err != nil || len(ids) != 0 || len(names) != 0 {
		t.Fatalf("empty: ids=%v names=%v err=%v", ids, names, err)
	}

	ids, names, err = parseTelegramAllowlist(" 123456789 , @ExampleUser , exampleuser ")
	if err != nil {
		t.Fatal(err)
	}
	if len(ids) != 1 || ids[0] != 123456789 {
		t.Fatalf("ids = %v", ids)
	}
	if len(names) != 1 || names[0] != "exampleuser" {
		t.Fatalf("names = %v (want deduped lowercase)", names)
	}

	_, _, err = parseTelegramAllowlist("ab")
	if err == nil {
		t.Fatal("expected error for too-short username")
	}
}
