package staffnotice

import "time"

// offerReceivedSubject and offerReceivedText are the Offer mail to
// someone already at the Practice. ADR-0009's rule, as the email-path
// Offer mail keeps it (offer.offerSubject): no Practice, no Client, no
// fee. The subject and first line are that mail's own, so the two read
// alike; the date is when the Offer expires, in the Practice's zone with
// the zone named. Worded from three persona consults on #2267.
const offerReceivedSubject = "You've been offered work on DoulaCloud"

const expiresLayout = "January 2, 2006, 3:04pm MST"

func offerReceivedText(link string, expiresAt time.Time) string {
	return "Hello,\n\n" +
		"You've been offered work at a practice on DoulaCloud. This link opens the offer at that practice:\n\n" +
		link + "\n\n" +
		"Answer by " + expiresAt.Format(expiresLayout) + ". After that, the offer expires.\n\n" +
		"If you have questions, reply to this email.\n"
}
