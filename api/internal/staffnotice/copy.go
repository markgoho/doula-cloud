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

// putOnEngagementSubject and putOnEngagementText are the mail to a Doula
// someone else put on an Engagement (#2403). ADR-0009 kept on
// 2026-11-01: no Client, colleague or Practice; the Engagement page,
// one sign-in away, names the Client and who put her on. "A client",
// not "a client's birth", because an Engagement may be postpartum care
// only. Shaped like offerReceivedText, from the persona consults on
// #2403.
const putOnEngagementSubject = "You've been put on a client on DoulaCloud"

func putOnEngagementText(link string) string {
	return "Hello,\n\n" +
		"You've been put on a client at your practice on DoulaCloud. This link opens that client's page at your practice:\n\n" +
		link + "\n\n" +
		"If you have questions, reply to this email.\n"
}

// takenOffEngagementSubject and takenOffEngagementText are the mail to a
// Doula someone else took off an Engagement (#2596). ADR-0009 as
// putOnEngagementText keeps it: no Client, colleague or Practice. A
// contractor who holds neither Owner nor Admin can no longer open the
// Engagement, only the screen that says her work ended, so her mail says
// that is where the link goes and repeats that screen's line about her
// fee, her terms and her pay (Lena, on #2596). An employee can still open
// the client's page. Both say her visits that have not happened yet stay hers until the
// Practice moves them, with no date or kind of visit (#2634).
const takenOffEngagementSubject = "You've been taken off a client on DoulaCloud"

const openVisitsStayLine = "Any visits with this client that haven't happened yet stay yours until your practice moves them to someone else."

func takenOffEngagementText(link string, isContractor bool) string {
	if isContractor {
		return "Hello,\n\n" +
			"You've been taken off a client at your practice on DoulaCloud. This link shows the day your work ended:\n\n" +
			link + "\n\n" +
			openVisitsStayLine + "\n\n" +
			"Your fee, the terms and what you were paid stay on Your offers.\n\n" +
			"If you have questions, reply to this email.\n"
	}
	return "Hello,\n\n" +
		"You've been taken off a client at your practice on DoulaCloud. This link opens that client's page at your practice:\n\n" +
		link + "\n\n" +
		openVisitsStayLine + "\n\n" +
		"If you have questions, reply to this email.\n"
}

// visitReassignedToSubject, visitReassignedFromSubject and their texts
// are the mail to a Doula someone else moved a Visit onto, or off (#2436).
// ADR-0009 as putOnEngagementText keeps it: no Client, colleague,
// Practice, date or kind of Visit; the Engagement page, one sign-in away,
// lists the Visit and who has it. "A visit", not "the client", for the
// one taken off her: she may still be on the Client. Worded from the
// persona consults on #2436.
const (
	visitReassignedToSubject   = "You've been put on a visit on DoulaCloud"
	visitReassignedFromSubject = "You've been taken off a visit on DoulaCloud"
)

func visitReassignedToText(link string) string {
	return "Hello,\n\n" +
		"You've been put on a visit at your practice on DoulaCloud. This link opens that client's page at your practice, where the visit is listed:\n\n" +
		link + "\n\n" +
		"If you have questions, reply to this email.\n"
}

func visitReassignedFromText(link string, isEndedContractor bool) string {
	if isEndedContractor {
		return "Hello,\n\n" +
			"You've been taken off a visit at your practice on DoulaCloud. Someone else has it now. This link shows the day your work ended:\n\n" +
			link + "\n\n" +
			"Your fee, the terms and what you were paid stay on Your offers.\n\n" +
			"If you have questions, reply to this email.\n"
	}
	return "Hello,\n\n" +
		"You've been taken off a visit at your practice on DoulaCloud. Someone else has it now. This link opens that client's page at your practice:\n\n" +
		link + "\n\n" +
		"If you have questions, reply to this email.\n"
}
