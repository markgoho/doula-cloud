package internalauth

import "testing"

const (
	emailClaim    = "email"
	verifiedClaim = "email_verified"
	testCaller    = "caller@doula-cloud.iam.gserviceaccount.com"
)

// emailFromClaims is the half of GoogleValidator that does not need
// Google: the rule ADR-0037 states as a guarantee -- an unverified or
// absent email claim is refused rather than passed to the allowlist as
// an empty string.
func TestEmailFromClaims(t *testing.T) {
	tests := []struct {
		name    string
		claims  map[string]any
		want    string
		wantErr bool
	}{
		{
			name:   "a service account's verified claim",
			claims: map[string]any{emailClaim: testCaller, verifiedClaim: true},
			want:   testCaller,
		},
		{
			name:    "an unverified email",
			claims:  map[string]any{emailClaim: testCaller, verifiedClaim: false},
			wantErr: true,
		},
		{
			name:    "no email claim at all",
			claims:  map[string]any{verifiedClaim: true},
			wantErr: true,
		},
		{
			name:    "an email claim of the wrong type",
			claims:  map[string]any{emailClaim: 42, verifiedClaim: true},
			wantErr: true,
		},
		{
			name:    "no claims",
			claims:  nil,
			wantErr: true,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got, err := emailFromClaims(tc.claims)
			if (err != nil) != tc.wantErr {
				t.Fatalf("emailFromClaims() error = %v, wantErr %v", err, tc.wantErr)
			}
			if got != tc.want {
				t.Errorf("emailFromClaims() = %q, want %q", got, tc.want)
			}
		})
	}
}
