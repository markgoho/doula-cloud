package templatefield_test

import (
	"os"
	"testing"

	"github.com/google/uuid"

	"doula-cloud/api/internal/templatefield"
	"doula-cloud/api/internal/testdb"
)

const (
	label     = "Pronouns"
	shortText = "short_text"
)

func TestMain(m *testing.M) {
	os.Exit(testdb.Main(m))
}

func TestCheckIdentity(t *testing.T) {
	cases := []struct {
		name  string
		seen  map[string]bool
		id    string
		typ   string
		label string
		want  string
	}{
		{"a valid field", map[string]bool{}, "f1", shortText, label, ""},
		{"missing id", map[string]bool{}, "", shortText, label, "field id is required"},
		{"duplicate id", map[string]bool{"f1": true}, "f1", shortText, label, "duplicate field id: f1"},
		{"unknown type", map[string]bool{}, "f1", "date", label, "unknown field type: date"},
		{"missing label", map[string]bool{}, "f1", shortText, "", "field label is required"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := templatefield.CheckIdentity(tc.seen, tc.id, tc.typ, tc.label); got != tc.want {
				t.Fatalf("CheckIdentity = %q, want %q", got, tc.want)
			}
		})
	}

	seen := map[string]bool{}
	templatefield.CheckIdentity(seen, "f1", shortText, label)
	if !seen["f1"] {
		t.Fatal("CheckIdentity did not record the id it passed")
	}
}

func TestCheckOptions(t *testing.T) {
	cases := []struct {
		name    string
		typ     string
		options []string
		want    string
	}{
		{"a select with options", "single_select", []string{"a", "b"}, ""},
		{"a select with none", "multi_select", nil, "field f1 requires at least one option"},
		{"a select with a blank", "single_select", []string{"a", ""}, "field f1 has a blank option"},
		{"a text field with none", shortText, nil, ""},
		{"a text field with some", shortText, []string{"a"}, "field f1 of type short_text may not have options"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := templatefield.CheckOptions("f1", tc.typ, tc.options); got != tc.want {
				t.Fatalf("CheckOptions = %q, want %q", got, tc.want)
			}
		})
	}
}

func TestRecordChange_WritesOneRowOnlyWhenTheListChanged(t *testing.T) {
	db := testdb.New(t)
	practiceID := testdb.SeedPractice(t, db, "Template Field Practice")
	staffID := testdb.SeedStaffAtPractice(t, db, practiceID, "tf-owner", []string{"owner"}, "employee")
	templateID := uuid.NewString()

	tx, err := db.Admin.BeginTx(t.Context(), nil)
	if err != nil {
		t.Fatalf("begin: %v", err)
	}
	defer func() { _ = tx.Rollback() }()

	count := func() int {
		var n int
		if err := tx.QueryRowContext(t.Context(),
			`SELECT count(*) FROM activity WHERE subject_kind = 'plan_template' AND subject_id = $1`, templateID,
		).Scan(&n); err != nil {
			t.Fatalf("count: %v", err)
		}
		return n
	}

	type field struct {
		ID string `json:"id"`
	}
	if err := templatefield.RecordChange(t.Context(), tx, practiceID, staffID, "plan_template", templateID, []field{{ID: "a"}}, []byte(`[{"id":"a"}]`)); err != nil {
		t.Fatalf("unchanged: %v", err)
	}
	if got := count(); got != 0 {
		t.Fatalf("unchanged list wrote %d rows, want 0", got)
	}
	if err := templatefield.RecordChange(t.Context(), tx, practiceID, staffID, "plan_template", templateID, []field{{ID: "a"}}, []byte(`[{"id":"a"},{"id":"b"}]`)); err != nil {
		t.Fatalf("changed: %v", err)
	}
	if got := count(); got != 1 {
		t.Fatalf("changed list wrote %d rows, want 1", got)
	}
}
