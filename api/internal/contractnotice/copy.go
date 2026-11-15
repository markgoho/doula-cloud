package contractnotice

// The mail is content-free (ADR-0009, ADR-0011, #221): no Client name, no
// Engagement detail and, for v1, no Practice name, in subject or body. It
// speaks to her in her own register (ADR-0005): "contract", lowercase, and
// "you". The link is the only variable.
const (
	sentSubject     = "You have a contract waiting for you"
	reminderSubject = "A reminder: you have a contract waiting for you"
)

func copyFor(kind Kind, link string) (subject, text string) {
	if kind == KindReminder {
		return reminderSubject, body("This is a reminder that you have a contract waiting for your signature.", link)
	}
	return sentSubject, body("You have a contract waiting for your signature.", link)
}

func body(sentence, link string) string {
	return "Hello,\n\n" +
		sentence + "\n\n" +
		link + "\n\n" +
		"If you weren't expecting this, you can safely ignore this email.\n"
}
