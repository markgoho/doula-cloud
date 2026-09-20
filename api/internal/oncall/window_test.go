package oncall_test

import (
	"testing"

	"doula-cloud/api/internal/oncall"
)

func ptr(s string) *string { return &s }

// birth is the ordinary case every table row below starts from: an
// active birth Engagement with a granted Attachment, the Practice's
// default rule, and a due date.
func birth() oncall.WindowInput {
	return oncall.WindowInput{
		Kind:           "birth",
		Status:         "active",
		DueDate:        ptr("2026-10-30"),
		FirstGrantedOn: ptr("2026-06-01"),
		Rule:           oncall.Rule{Start: oncall.StartGestationalWeek, Week: 37, GraceDays: 14},
	}
}

func TestDeriveWindow(t *testing.T) {
	tests := []struct {
		name       string
		edit       func(*oncall.WindowInput)
		wantStart  string
		wantEnd    string
		wantReason oncall.NoWindowReason
	}{
		{
			name:      "37w0d is three weeks before the due date, and the end is the due date plus the grace",
			edit:      func(*oncall.WindowInput) {},
			wantStart: "2026-10-09",
			wantEnd:   "2026-11-13",
		},
		{
			name:      "a later gestational week starts later",
			edit:      func(in *oncall.WindowInput) { in.Rule.Week = 39 },
			wantStart: "2026-10-23",
			wantEnd:   "2026-11-13",
		},
		{
			name:      "the grant-date rule starts on the day the Attachment was granted",
			edit:      func(in *oncall.WindowInput) { in.Rule.Start = oncall.StartAttachmentGranted },
			wantStart: "2026-06-01",
			wantEnd:   "2026-11-13",
		},
		{
			name:      "a recorded pregnancy end closes the window on that day, not on the grace",
			edit:      func(in *oncall.WindowInput) { in.PregnancyEndedOn = ptr("2026-10-27") },
			wantStart: "2026-10-09",
			wantEnd:   "2026-10-27",
		},
		{
			name:      "zero grace ends on the due date itself",
			edit:      func(in *oncall.WindowInput) { in.Rule.GraceDays = 0 },
			wantStart: "2026-10-09",
			wantEnd:   "2026-10-30",
		},
		{
			name:       "no due date under a gestational rule is no window, never a guessed one",
			edit:       func(in *oncall.WindowInput) { in.DueDate = nil },
			wantReason: oncall.NoWindowNoDueDate,
		},
		{
			name: "no due date under the grant-date rule still has no end to resolve",
			edit: func(in *oncall.WindowInput) {
				in.DueDate = nil
				in.Rule.Start = oncall.StartAttachmentGranted
			},
			wantReason: oncall.NoWindowNoDueDate,
		},
		{
			name: "the grant-date rule with a recorded pregnancy end needs no due date",
			edit: func(in *oncall.WindowInput) {
				in.DueDate = nil
				in.Rule.Start = oncall.StartAttachmentGranted
				in.PregnancyEndedOn = ptr("2026-09-02")
			},
			wantStart: "2026-06-01",
			wantEnd:   "2026-09-02",
		},
		{
			name:       "a postpartum Engagement never has a window",
			edit:       func(in *oncall.WindowInput) { in.Kind = "postpartum" },
			wantReason: oncall.NoWindowPostpartum,
		},
		{
			name:       "an Engagement still in intake has no window",
			edit:       func(in *oncall.WindowInput) { in.Status = "intake" },
			wantReason: oncall.NoWindowNotActive,
		},
		{
			name:       "a completed Engagement has no window",
			edit:       func(in *oncall.WindowInput) { in.Status = "completed" },
			wantReason: oncall.NoWindowNotActive,
		},
		{
			name:       "nobody granted on the Engagement means nobody is on call for it",
			edit:       func(in *oncall.WindowInput) { in.FirstGrantedOn = nil },
			wantReason: oncall.NoWindowNobodyAttached,
		},
		{
			name:       "a birth before the window would have opened leaves no window",
			edit:       func(in *oncall.WindowInput) { in.PregnancyEndedOn = ptr("2026-09-20") },
			wantReason: oncall.NoWindowEndedBeforeStart,
		},
		{
			name:       "an empty due-date string is treated as absent",
			edit:       func(in *oncall.WindowInput) { in.DueDate = ptr("") },
			wantReason: oncall.NoWindowNoDueDate,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			in := birth()
			tt.edit(&in)
			w, reason := oncall.DeriveWindow(in)
			if reason != tt.wantReason {
				t.Fatalf("reason = %q, want %q", reason, tt.wantReason)
			}
			if tt.wantReason != "" {
				if w != nil {
					t.Fatalf("window = %+v, want none", *w)
				}
				return
			}
			if w == nil {
				t.Fatal("window = nil, want one")
			}
			if w.Start != tt.wantStart || w.End != tt.wantEnd {
				t.Fatalf("window = %s..%s, want %s..%s", w.Start, w.End, tt.wantStart, tt.wantEnd)
			}
		})
	}
}

func TestRuleResolve(t *testing.T) {
	practice := oncall.Rule{Start: oncall.StartGestationalWeek, Week: 37, GraceDays: 10}

	if got := practice.Resolve(nil, nil); got != practice {
		t.Fatalf("no override = %+v, want the Practice's own rule", got)
	}

	week := int16(38)
	if got := practice.Resolve(ptr(string(oncall.StartGestationalWeek)), &week); got.Week != 38 || got.GraceDays != 10 {
		t.Fatalf("week override = %+v, want week 38 and the Practice's grace", got)
	}

	if got := practice.Resolve(ptr(string(oncall.StartAttachmentGranted)), nil); got.Start != oncall.StartAttachmentGranted || got.GraceDays != 10 {
		t.Fatalf("grant-date override = %+v, want the grant-date rule with the Practice's grace", got)
	}
}

func TestEffectiveInterval(t *testing.T) {
	window := oncall.Window{Start: "2026-10-09", End: "2026-11-13"}

	tests := []struct {
		name     string
		from, to *string
		wantFrom string
		wantTo   string
		wantNone bool
	}{
		{name: "no narrowing is the whole window", wantFrom: "2026-10-09", wantTo: "2026-11-13"},
		{name: "a narrowing inside the window is the narrowing", from: ptr("2026-10-20"), to: ptr("2026-10-25"), wantFrom: "2026-10-20", wantTo: "2026-10-25"},
		{name: "an open end runs to the window's end", from: ptr("2026-10-23"), wantFrom: "2026-10-23", wantTo: "2026-11-13"},
		{name: "an open start runs from the window's start", to: ptr("2026-10-22"), wantFrom: "2026-10-09", wantTo: "2026-10-22"},
		{name: "a narrowing wider than the window is clipped to it", from: ptr("2026-09-01"), to: ptr("2026-12-01"), wantFrom: "2026-10-09", wantTo: "2026-11-13"},
		{name: "a narrowing wholly outside the window is not on call at all", from: ptr("2026-12-01"), to: ptr("2026-12-05"), wantNone: true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, ok := window.Effective(tt.from, tt.to)
			if tt.wantNone {
				if ok {
					t.Fatalf("effective = %+v, want none", got)
				}
				return
			}
			if !ok || got.Start != tt.wantFrom || got.End != tt.wantTo {
				t.Fatalf("effective = %+v (ok=%v), want %s..%s", got, ok, tt.wantFrom, tt.wantTo)
			}
		})
	}
}
