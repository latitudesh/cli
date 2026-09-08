package objectstorage

import "testing"

func TestNormalizeStorageJSON(t *testing.T) {
	cases := map[string]string{
		`{"retention_period":""}`:                                    `{"retention_period":null}`,
		`{"retention_period": ""}`:                                   `{"retention_period":null}`,
		`{"retention_period":"30"}`:                                  `{"retention_period":30}`,
		`{"retention_period":"n/a"}`:                                 `{"retention_period":null}`,
		`{"retention_period":30}`:                                    `{"retention_period":30}`,
		`{"retention_period":null}`:                                  `{"retention_period":null}`,
		`{"name":"x"}`:                                               `{"name":"x"}`,
		`{"a":{"retention_period":"7"},"b":{"retention_period":""}}`: `{"a":{"retention_period":7},"b":{"retention_period":null}}`,
	}
	for in, want := range cases {
		if got := string(NormalizeStorageJSON([]byte(in))); got != want {
			t.Errorf("NormalizeStorageJSON(%s) = %s, want %s", in, got, want)
		}
	}
}
