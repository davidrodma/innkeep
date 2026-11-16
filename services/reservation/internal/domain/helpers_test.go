package domain_test

import (
	"encoding/json"
	"testing"
)

func mustJSON(t *testing.T, value any) []byte {
	t.Helper()
	encoded, err := json.Marshal(value)
	if err != nil {
		t.Fatalf("serializando: %v", err)
	}
	return encoded
}
