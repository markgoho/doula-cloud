package main

import "testing"

func TestRun_MissingDatabaseURL(t *testing.T) {
	t.Setenv("DATABASE_URL", "")

	err := run()
	if err == nil {
		t.Fatal("expected an error when DATABASE_URL is unset, got nil")
	}
}
