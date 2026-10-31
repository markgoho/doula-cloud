package stripemode_test

import (
	"strings"
	"testing"

	"doula-cloud/api/internal/stripemode"
)

//nolint:gosec // test fixture values, not real credentials
const (
	testSecret  = "sk_test_51SECRETVALUEtest"
	liveSecret  = "sk_live_51SECRETVALUElive"
	testRestrct = "rk_test_51SECRETVALUErtest"
	liveRestrct = "rk_live_51SECRETVALUErlive"
	testPub     = "pk_test_51PUBVALUEtest"
	livePub     = "pk_live_51PUBVALUElive"
)

const (
	key      = stripemode.APIKeyVar
	pub      = stripemode.PublishableKeyVar
	modeVar  = stripemode.ModeVar
	cloudRun = stripemode.CloudRunVar
	service  = "doula-api"
	test     = string(stripemode.Test)
	live     = string(stripemode.Live)
)

func env(m map[string]string) func(string) string {
	return func(k string) string { return m[k] }
}

func TestLoad_Accepts(t *testing.T) {
	cases := []struct {
		name string
		env  map[string]string
		want stripemode.Mode
	}{
		{"no key", map[string]string{}, ""},
		{"no key on Cloud Run", map[string]string{cloudRun: service}, ""},
		{"local test key", map[string]string{key: testSecret}, stripemode.Test},
		{"local restricted test key with newline", map[string]string{key: testRestrct + "\n"}, stripemode.Test},
		{"local test key with publishable and mode", map[string]string{key: testSecret, pub: testPub, modeVar: test}, stripemode.Test},
		{"Cloud Run test", map[string]string{key: testSecret, pub: testPub, modeVar: test, cloudRun: service}, stripemode.Test},
		{"Cloud Run live", map[string]string{key: liveSecret, pub: livePub, modeVar: live, cloudRun: service}, stripemode.Live},
		{"Cloud Run restricted live", map[string]string{key: liveRestrct, pub: livePub, modeVar: live, cloudRun: service}, stripemode.Live},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got, err := stripemode.Load(env(tc.env))
			if err != nil {
				t.Fatalf("Load: unexpected error: %v", err)
			}
			if got != tc.want {
				t.Fatalf("Load = %q, want %q", got, tc.want)
			}
		})
	}
}

func TestLoad_Refuses(t *testing.T) {
	cases := []struct {
		name  string
		env   map[string]string
		names []string
	}{
		{"invalid STRIPE_MODE", map[string]string{modeVar: "prod"}, []string{modeVar}},
		{"unrecognized key prefix", map[string]string{key: "pk_test_51PUBVALUEtest"}, []string{key}},
		{"unrecognized publishable prefix", map[string]string{key: testSecret, pub: "sk_test_51SECRETVALUEtest"}, []string{pub}},
		{"test key, live publishable key", map[string]string{key: testSecret, pub: livePub}, []string{key, pub}},
		{"live key, test publishable key", map[string]string{key: liveSecret, pub: testPub, modeVar: live, cloudRun: service}, []string{key, pub}},
		{"Cloud Run with no STRIPE_MODE", map[string]string{key: testSecret, pub: testPub, cloudRun: service}, []string{modeVar, cloudRun}},
		{"Cloud Run with no publishable key", map[string]string{key: testSecret, modeVar: test, cloudRun: service}, []string{pub, cloudRun}},
		{"Cloud Run with a blank publishable key", map[string]string{key: liveSecret, pub: " \n", modeVar: live, cloudRun: service}, []string{pub, cloudRun}},
		{"Cloud Run, STRIPE_MODE test, live key", map[string]string{key: liveSecret, pub: livePub, modeVar: test, cloudRun: service}, []string{modeVar, key}},
		{"Cloud Run, STRIPE_MODE live, test key", map[string]string{key: testSecret, pub: testPub, modeVar: live, cloudRun: service}, []string{modeVar, key}},
		{"local live key", map[string]string{key: liveSecret}, []string{key, cloudRun}},
		{"local live key with STRIPE_MODE live", map[string]string{key: liveSecret, modeVar: live}, []string{key, cloudRun}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			_, err := stripemode.Load(env(tc.env))
			if err == nil {
				t.Fatal("Load: want an error, got none")
			}
			msg := err.Error()
			for _, name := range tc.names {
				if !strings.Contains(msg, name) {
					t.Errorf("error %q does not name %s", msg, name)
				}
			}
			for _, v := range []string{"SECRETVALUE", "PUBVALUE"} {
				if strings.Contains(msg, v) {
					t.Errorf("error %q contains a key value", msg)
				}
			}
		})
	}
}

func TestMode_Matches(t *testing.T) {
	cases := []struct {
		mode     stripemode.Mode
		livemode bool
		want     bool
	}{
		{"", false, true},
		{"", true, true},
		{stripemode.Test, false, true},
		{stripemode.Test, true, false},
		{stripemode.Live, true, true},
		{stripemode.Live, false, false},
	}
	for _, tc := range cases {
		if got := tc.mode.Matches(tc.livemode); got != tc.want {
			t.Errorf("Mode(%q).Matches(%v) = %v, want %v", tc.mode, tc.livemode, got, tc.want)
		}
	}
}
