package visit

// A Visit that has not happened yet, held by a Doula who is no longer on
// its Engagement, is flagged on every Staff read (#2596, #2665): the Visit
// is scheduled for later than now or has no date at all, the Engagement is
// not completed (completion ends every Attachment, which says nothing about
// who covers a Visit), and the holder had a granted Attachment that has
// ended and has none open. Taking a Doula off leaves her Visits with her
// until someone reassigns them, so this is how they stay in view.
//
// "The holder has left" is engagement_holder_left (00173), the one
// definition the Client portal's door calls too. The predicate is a pair
// of constants around the placeholder that carries clock.Now, so the
// caller keeps its query a constant expression. It reads the aliases v
// (visits) and e (engagements).
const (
	offEngagementBefore = `((v.scheduled_at IS NULL OR v.scheduled_at > `
	offEngagementAfter  = `) AND e.status <> 'completed'
	        AND engagement_holder_left(v.engagement_id, v.staff_id))`
)
