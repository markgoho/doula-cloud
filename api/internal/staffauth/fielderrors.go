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
const (
	// MsgPracticeNameNeeded is signup's practiceName field.
	MsgPracticeNameNeeded = "Enter the name of your Practice"
	// MsgStaffNameNeeded is signup's staffName field.
	MsgStaffNameNeeded = "Enter your name"
	// MsgWorkStateNeeded is the workState field, on both the screens that
	// ask for it -- signup and the account page's own correction (#437).
	// The pair of MsgWorkStateRequired, which is the summary line the
	// same refusal carries in APIError.Message: that one names the JSON
	// field and the format for a caller reading the API, this one is for
	// the person looking at the control.
	MsgWorkStateNeeded = "Choose the state you work from"
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
)
