package db

import (
	"reflect"
	"testing"
)

func TestJSONBArgEncodesStructuredValuesForPgxRunners(t *testing.T) {
	arg, err := JSONBArg(PgxRunner{}, map[string]any{"ok": true})
	if err != nil {
		t.Fatal(err)
	}
	if arg != `{"ok":true}` {
		t.Fatalf("jsonb arg = %#v, want compact JSON string", arg)
	}
}

func TestJSONBArgPreservesStructuredValuesForFakeRunners(t *testing.T) {
	value := map[string]any{"ok": true}
	arg, err := JSONBArg(struct{}{}, value)
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(arg, value) {
		t.Fatalf("jsonb arg = %#v, want original map", arg)
	}
}
