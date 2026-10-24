package main

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// goog and cloud are cut-down copies of Google's two published lists, in
// the shape https://www.gstatic.com/ipranges/ serves them: goog.json is
// every range Google uses, cloud.json the part of it a customer can rent.
// The live goog.json holds every range in clientip's hostingEdgeRanges;
// this copy does too, plus 8.8.4.0/24 (Google's own, not an edge) and
// 34.0.0.0/15 (rentable, in both lists).
const goog = `{"syncToken":"1","creationTime":"2026-10-24T01:06:28","prefixes":[
{"ipv4Prefix":"8.8.4.0/24"},
{"ipv4Prefix":"34.0.0.0/15"},
{"ipv4Prefix":"64.233.160.0/19"},
{"ipv4Prefix":"66.102.0.0/20"},
{"ipv4Prefix":"66.249.64.0/19"},
{"ipv4Prefix":"74.125.0.0/16"},
{"ipv4Prefix":"192.178.0.0/15"},
{"ipv6Prefix":"2001:4860::/32"}]}`

const cloud = `{"syncToken":"1","creationTime":"2026-10-24T01:06:28","prefixes":[
{"ipv4Prefix":"34.0.0.0/15","service":"Google Cloud","scope":"us-central1"}]}`

// lists writes goog.json and cloud.json into a temporary directory and
// returns their paths, the two arguments the command takes.
func lists(t *testing.T, googJSON, cloudJSON string) []string {
	t.Helper()
	dir := t.TempDir()
	googPath := filepath.Join(dir, "goog.json")
	cloudPath := filepath.Join(dir, "cloud.json")
	if err := os.WriteFile(googPath, []byte(googJSON), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(cloudPath, []byte(cloudJSON), 0o600); err != nil {
		t.Fatal(err)
	}
	return []string{googPath, cloudPath}
}

// TestRun_GoogleAddressOutsideTheEdgeRangesFails is the case the check
// exists for (#1867): Hosting connected from 64.233.172.x for a month
// before anyone added 64.233.160.0/19, and nothing said so. Any address
// from Google's own egress that is not a known edge is reported by name.
func TestRun_GoogleAddressOutsideTheEdgeRangesFails(t *testing.T) {
	var out strings.Builder
	in := strings.NewReader("74.125.209.169\n8.8.4.4\n8.8.4.4\n")

	err := run(lists(t, goog, cloud), in, &out)
	if err == nil {
		t.Fatalf("run() = nil, want an error naming 8.8.4.4; output:\n%s", out.String())
	}
	if !strings.Contains(out.String(), "8.8.4.4: 2 requests") {
		t.Fatalf("output does not name 8.8.4.4 with its count:\n%s", out.String())
	}
	if strings.Contains(out.String(), "74.125.209.169") {
		t.Fatalf("output names an address inside hostingEdgeRanges:\n%s", out.String())
	}
}

// TestRun_KnownEdgesAndRentedAddressesPass is a normal day: Hosting
// connects from inside the ranges, and Cloud Scheduler, Stripe and the
// deploy smoke test connect from addresses that are either a customer's
// (cloud.json) or not Google's at all. None of them is reported.
func TestRun_KnownEdgesAndRentedAddressesPass(t *testing.T) {
	var out strings.Builder
	in := strings.NewReader("74.125.209.169\n\n64.233.172.134\n::ffff:66.102.6.224\n34.1.2.3\n20.118.239.226\n")

	if err := run(lists(t, goog, cloud), in, &out); err != nil {
		t.Fatalf("run() = %v, want nil; output:\n%s", err, out.String())
	}
	if !strings.Contains(out.String(), "checked 5 requests from 5 addresses") {
		t.Fatalf("output does not report what it checked:\n%s", out.String())
	}
}

// failingWriter is a stdout that refuses every write.
type failingWriter struct{}

func (failingWriter) Write([]byte) (int, error) { return 0, errors.New("closed") }

// TestRun_AReportThatCannotBeWrittenFails proves a run whose report never
// reached the log is a failed run, not a silent pass.
func TestRun_AReportThatCannotBeWrittenFails(t *testing.T) {
	err := run(lists(t, goog, cloud), strings.NewReader("74.125.209.169\n"), failingWriter{})
	if err == nil || !strings.Contains(err.Error(), "writing the report") {
		t.Fatalf("run() = %v, want an error about writing the report", err)
	}
}

// TestRun_EdgeRangeDriftFails covers the other way the list goes wrong:
// a range From trusts stops being Google's own egress. Out of goog.json
// it is no longer Google's; inside cloud.json a customer can rent an
// address in it and choose the address From returns.
func TestRun_EdgeRangeDriftFails(t *testing.T) {
	tests := []struct {
		name      string
		googJSON  string
		cloudJSON string
		want      string
	}{
		{
			name:      "a range left goog.json",
			googJSON:  strings.Replace(goog, `{"ipv4Prefix":"74.125.0.0/16"},`, "", 1),
			cloudJSON: cloud,
			want:      "74.125.0.0/16: in hostingEdgeRanges but no longer inside goog.json",
		},
		{
			name:      "goog.json holds only a narrower part of a range",
			googJSON:  strings.Replace(goog, "74.125.0.0/16", "74.125.0.0/17", 1),
			cloudJSON: cloud,
			want:      "74.125.0.0/16: in hostingEdgeRanges but no longer inside goog.json",
		},
		{
			name:      "part of a range moved into cloud.json",
			googJSON:  goog,
			cloudJSON: strings.Replace(cloud, "34.0.0.0/15", "74.125.128.0/17", 1),
			want:      "74.125.0.0/16: in hostingEdgeRanges but now overlaps cloud.json",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var out strings.Builder
			if err := run(lists(t, tt.googJSON, tt.cloudJSON), strings.NewReader(""), &out); err == nil {
				t.Fatalf("run() = nil, want an error; output:\n%s", out.String())
			}
			if !strings.Contains(out.String(), tt.want) {
				t.Fatalf("output does not say %q:\n%s", tt.want, out.String())
			}
		})
	}
}

// TestRun_RefusesInputItCannotRead proves a broken input is a failed run,
// never a quiet pass: a check that cannot read Google's lists or the log
// has checked nothing.
func TestRun_RefusesInputItCannotRead(t *testing.T) {
	missing := filepath.Join(t.TempDir(), "absent.json")
	tests := []struct {
		name  string
		args  func(t *testing.T) []string
		stdin string
		want  string
	}{
		{name: "wrong argument count", args: func(*testing.T) []string { return []string{"one"} }, want: "usage"},
		{name: "goog.json missing", args: func(t *testing.T) []string { return []string{missing, lists(t, goog, cloud)[1]} }, want: "absent.json"},
		{name: "cloud.json missing", args: func(t *testing.T) []string { return []string{lists(t, goog, cloud)[0], missing} }, want: "absent.json"},
		{name: "not JSON", args: func(t *testing.T) []string { return lists(t, "<html>", cloud) }, want: "goog.json"},
		{name: "no prefixes", args: func(t *testing.T) []string { return lists(t, `{"prefixes":[]}`, cloud) }, want: "holds no prefixes"},
		{name: "a prefix that does not parse", args: func(t *testing.T) []string { return lists(t, `{"prefixes":[{"ipv4Prefix":"nope"}]}`, cloud) }, want: "goog.json"},
		{name: "a remote IP that does not parse", args: func(t *testing.T) []string { return lists(t, goog, cloud) }, stdin: "not-an-address\n", want: `"not-an-address"`},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var out strings.Builder
			err := run(tt.args(t), strings.NewReader(tt.stdin), &out)
			if err == nil || !strings.Contains(err.Error(), tt.want) {
				t.Fatalf("run() = %v, want an error containing %q", err, tt.want)
			}
		})
	}
}
