package legalnotice

// Values is what one email names: which document, the date its new
// version takes effect, which Practice, the version's one line of what
// changed, and the three links. docs/copy/legal-change-email.md is the
// copy this fills, and TestCopy_MatchesTheCopyDocument fails when the
// two differ.
type Values struct {
	Document    string
	Date        string
	Practice    string
	Change      string
	DocumentURL string
	SettingsURL string
	DeleteURL   string
}

// Compose writes the subject and the plain-text body of the email for v.
// Plain text only, with no tracking (ADR-0030): it is a notice the Terms
// promise, not a message anybody measures.
func Compose(v Values) (subject, text string) {
	subject = "DoulaCloud: a new version of the " + v.Document + " starts on " + v.Date
	text = "Hello,\n\n" +
		"A new version of the DoulaCloud " + v.Document + " starts on " + v.Date + ". This email is about your practice, " + v.Practice + ".\n\n" +
		"What changes: " + v.Change + "\n\n" +
		"Read the new version:\n" +
		v.DocumentURL + "\n\n" +
		"From " + v.Date + ", the new version applies to " + v.Practice + ". The first Owner of " + v.Practice + " who signs in after that date is asked to agree.\n\n" +
		"If you do not agree, you can leave before " + v.Date + ":\n\n" +
		"1. Export your practice's data: " + v.SettingsURL + "\n" +
		"2. Ask for a refund of your unspent purchased Credits: reply to this email. We do not need a reason.\n" +
		"3. Delete your practice: " + v.DeleteURL + "\n\n" +
		"Ask for the refund before you start deletion. A Credit that is still unspent when deletion finishes ends with the practice.\n\n" +
		"If you have questions, reply to this email.\n"
	return subject, text
}
