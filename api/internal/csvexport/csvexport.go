// Package csvexport is what the Staff CSV downloads of a list share (#2871,
// #2891): the rate limit, the Practice's zone and instant, the one Activity
// row a download writes in the same transaction as its read, and the
// response. What differs between them, the body and the diff, is the
// caller's.
package csvexport

import (
	"encoding/json/v2"
	"fmt"
	"net/http"
	"time"

	"doula-cloud/api/internal/activity"
	"doula-cloud/api/internal/apierr"
	"doula-cloud/api/internal/clock"
	"doula-cloud/api/internal/practicetimezone"
	"doula-cloud/api/internal/ratelimit"
	"doula-cloud/api/internal/staffauth"
)

// Rules limits a download as a heavy endpoint (docs/api-design.md section 6):
// every call reads the whole book the request matches. One Staff member's
// session, 30 an hour, is a person downloading a few views of the book, well
// below a script's rate. The count is taken on the request's own transaction,
// because the route sits behind staffauth.Middleware, which holds the
// request's one pool connection.
var Rules = []ratelimit.Rule{
	ratelimit.SessionCookieRule(30, time.Hour),
}

// Spec names one download: the rate limit's name, the Activity action it
// records, and the file's name before its day.
type Spec struct {
	Limit  string
	Action activity.PracticeAction
	File   string
}

// Env is the Practice's zone and the request's one instant.
type Env struct {
	Zone *time.Location
	Now  time.Time
}

// File is what a build read: the CSV, the Activity diff of which slice of the
// book it was, and any header the caller adds.
type File struct {
	Body    []byte
	Diff    any
	Headers map[string]string
}

// Handler is the download Spec names, for the Owner and Admin roles that the
// mount declares. build reads the request and answers a refusal itself,
// returning false; a read that failed answers 500 with no file and no row.
// The CSV is built whole before anything is written: the Activity row goes in
// the same transaction as the read, and nothing is sent until it is in.
func Handler(spec Spec, build func(w http.ResponseWriter, r *http.Request, c staffauth.Call, env Env) (File, bool)) http.Handler {
	return staffauth.Handler(func(w http.ResponseWriter, r *http.Request, c staffauth.Call) {
		if !ratelimit.Enforce(w, r, c.Tx, spec.Limit, Rules) {
			return
		}
		zone, err := practicetimezone.Load(r.Context(), c.Tx, c.PracticeID)
		if err != nil {
			// coverage:ignore reason: DB query failure, not exercised by unit tests
			apierr.WriteError(w, apierr.MsgInternalError, http.StatusInternalServerError)
			return
		}
		env := Env{Zone: zone, Now: clock.Now(r.Context())}
		file, ok := build(w, r, c, env)
		if !ok {
			return
		}
		diff, _ := json.Marshal(file.Diff)
		if err := activity.Record(r.Context(), c.Tx, activity.Entry{
			PracticeID:  c.PracticeID,
			SubjectKind: activity.SubjectPractice,
			SubjectID:   c.PracticeID,
			Action:      string(spec.Action),
			Diff:        diff,
			Actor:       activity.StaffActor(c.StaffID),
			CreatedAt:   env.Now,
		}); err != nil {
			// coverage:ignore reason: DB query failure, not exercised by unit tests
			apierr.WriteError(w, apierr.MsgInternalError, http.StatusInternalServerError)
			return
		}
		w.Header().Set("Content-Type", "text/csv; charset=utf-8")
		w.Header().Set("Content-Disposition", fmt.Sprintf(`attachment; filename="%s-%s.csv"`, spec.File, env.Now.In(zone).Format(time.DateOnly)))
		for k, v := range file.Headers {
			w.Header().Set(k, v)
		}
		_, _ = w.Write(file.Body)
	}, staffauth.NotAmbientContractor())
}
