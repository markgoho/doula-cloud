package staffauth

// The sentences this package puts in APIError.Details (#488).
//
// A Details entry is read by a person, not by a program: the app maps the
// key onto the control it names and prints the value above that control
// and in the error summary. So these follow the same GOV.UK rules
// app/src/lib/formErrors.ts is already gated on -- say what to do, start
// with the field's own noun, and never write "please", "valid", "invalid"
// or "required". apierr's TestDetailsWording is the Go half of that gate.
//
// Each one is the same sentence the client's own check already shows for
// that field (app/src/routes/(signed-out)/signup, .../invite,
// .../forgot-password, account), on the reasoning formErrors.ts's module
// comment gives: the same refusal has to read the same way wherever it
// happens, and a person who trips the client check and then the server
// one should not be told off in two different wordings.

// fieldEmail is the DTO json tag every address field in this package
// carries, and so the Details key each of their refusals is written
// under.
const fieldEmail = "email"

// fieldWorkState is the workState json tag, which three of this
// package's refusals key their Details under -- signup, the account
// page's own correction, and invitation acceptance.
const fieldWorkState = "workState"

// fieldTimezone is the timezone json tag signup's own zone refusal keys
// its Details under (#1166).
const fieldTimezone = "timezone"

const (
	// MsgPracticeNameNeeded is signup's practiceName field.
	MsgPracticeNameNeeded = "Enter the name of your Practice"
	// MsgStaffNameNeeded is signup's staffName field.
	MsgStaffNameNeeded = "Enter your name"
	// MsgOwnNameNeeded is the name field on the invitation-acceptance
	// form, which is the same question signup's staffName asks -- one
	// sentence apart because that form's own control is called `name`,
	// and a Details key is the DTO's json tag, not the concept's.
	MsgOwnNameNeeded = "Enter your name"
	// MsgWorkStateNeeded is the workState field, on both the screens that
	// ask for it -- signup and the account page's own correction (#437).
	// The pair of MsgWorkStateRequired, which is the summary line the
	// same refusal carries in APIError.Message: that one names the JSON
	// field and the format for a caller reading the API, this one is for
	// the person looking at the control.
	MsgWorkStateNeeded = "Choose the state you work from"
	// MsgTimezoneNeeded is signup's timezone field (#1166): the zone this
	// Practice keeps its calendar days in, which decides which day a
	// Visit falls on. The pair of MsgTimezoneRequired, on the same split
	// MsgWorkStateNeeded and MsgWorkStateRequired make -- this sentence
	// is for the person looking at the control, that one for a caller
	// reading the API.
	MsgTimezoneNeeded = "Choose the timezone this Practice works in"
	// MsgOwnAddressNeeded is the email field where the address is the
	// caller's own -- the password-reset request screen.
	MsgOwnAddressNeeded = "Enter your email address"
	// MsgInviteAddressNeeded is the email field on the invite screen,
	// where the address belongs to the person being invited rather than
	// to the caller.
	MsgInviteAddressNeeded = "Enter the Staff member's email address"
	// MsgRoleNeeded is the roles field on the invite and membership-edit
	// screens.
	MsgRoleNeeded = "Select at least one role"
	// MsgUnknownRole answers a roles array carrying a name that is not a
	// practice_role. Nobody reaches it from the invite form, whose only
	// control is a checkbox group built from the enum -- it is here for a
	// caller that sent its own body, and it still names the field so the
	// summary can focus it.
	MsgUnknownRole = "Select a role from the list"
	// MsgEmploymentTypeNeeded is the employmentType field.
	MsgEmploymentTypeNeeded = "Choose employee or contractor"
	// MsgMembershipAlreadyHeld is the ticket's own worked example: the
	// rule only the server knows, said in the reader's words and keyed
	// onto the control that caused it.
	MsgMembershipAlreadyHeld = "That address already holds a membership at this Practice"
	// MsgAddressBlocked refuses a Staff invitation to a currently
	// suppressed address (ADR-0029). Same wording as
	// portalinvite.msgAddressBlocked (#789): a person who trips either
	// invite's refusal reads it the same way. Not exported there because
	// that package's check is unconditional-on-a-stored-record rather
	// than a form field, so it carries no separate Details constant to
	// share.
	MsgAddressBlocked = "This email address is blocked. Blocked email addresses shows why and what can be done."
	// MsgPasswordTooShort is the newPassword field on the reset screen.
	// Spelled out rather than built with fmt.Sprintf from
	// minPasswordLength: TestDetailsWording reads a literal or a constant
	// and skips a call expression rather than guessing at it, so a
	// formatted sentence would be the one Details value the gate cannot
	// see. TestPasswordTooShortMatchesTheLimit keeps the number honest.
	MsgPasswordTooShort = "Password must be 6 characters or more"
)
