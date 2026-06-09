package utils

import (
	"errors"
	"strings"
	"testing"
)

func TestHumanizeAPIError(t *testing.T) {
	cases := []struct {
		name    string
		err     error
		wantSub string // substring expected in the result
	}{
		{
			name:    "nil error",
			err:     nil,
			wantSub: "",
		},
		{
			name:    "generic runtime 401 (real format from list endpoints)",
			err:     errors.New("[401] Unauthorized  &{Errors:[{Code:INVALID_TOKEN}]}"),
			wantSub: "lsh login",
		},
		{
			name:    "typed 401 with method/path prefix",
			err:     errors.New("[POST /servers][401] createServerUnauthorized  {}"),
			wantSub: "lsh login",
		},
		{
			name:    "typed 403 with method/path prefix",
			err:     errors.New("[DELETE /projects/{id}][403] deleteProjectForbidden  {}"),
			wantSub: "permission",
		},
		{
			name:    "unrecognized error passes through",
			err:     errors.New("connection refused"),
			wantSub: "connection refused",
		},
		{
			name:    "loose unauthorized word is not rewritten",
			err:     errors.New("field 'role' has an unauthorized value"),
			wantSub: "field 'role'",
		},
		{
			name:    "bare [401] in payload is not a status marker",
			err:     errors.New("[POST /servers][422] validationError  {\"hint\":\"see code [401]\"}"),
			wantSub: "[422]",
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := HumanizeAPIError(tc.err)
			if !strings.Contains(got, tc.wantSub) {
				t.Fatalf("HumanizeAPIError(%v) = %q, want substring %q", tc.err, got, tc.wantSub)
			}
		})
	}
}
