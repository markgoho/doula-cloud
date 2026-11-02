package visit

// A booked Visit held by a Doula who is no longer on its Engagement is
// flagged on every Staff read (#2596): the Visit is scheduled for later
// than now, the Engagement is not completed (completion ends every
// Attachment, which says nothing about who covers a Visit), and the holder
// had a granted Attachment that has ended and has none open. Taking a
// Doula off leaves her Visits with her until someone reassigns them, so
// this is how they stay in view.
//
// The predicate is a pair of constants around the placeholder that carries
// clock.Now, so the caller keeps its query a constant expression. It reads
// the aliases v (visits) and e (engagements).
const (
	offEngagementBefore = `(COALESCE(v.scheduled_at > `
	offEngagementAfter  = `, false) AND e.status <> 'completed'
	        AND NOT EXISTS (SELECT 1 FROM engagement_attachments ea
	                         WHERE ea.engagement_id = v.engagement_id AND ea.staff_id = v.staff_id AND ea.ended_at IS NULL)
	        AND EXISTS (SELECT 1 FROM engagement_attachments ea
	                     WHERE ea.engagement_id = v.engagement_id AND ea.staff_id = v.staff_id
	                       AND ea.origin = 'granted' AND ea.ended_at IS NOT NULL))`
)
