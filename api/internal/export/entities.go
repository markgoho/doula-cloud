package export

// subjectPractice and actionPracticeDataExported name the one Activity
// row Handler writes. staffauth's own PutMFARequiredHandler already
// writes "practice" as a bare subject_kind literal against a Practice's
// own id (mfarequired.go); this export sits beside it in the same
// vocabulary rather than inventing a second one.
const (
	subjectPractice            = "practice"
	actionPracticeDataExported = "practice_data_exported"
)

// Column-name literals repeated across more than one entity's header,
// named once for golangci-lint's goconst check rather than because any
// two entities share a meaning beyond the name.
const (
	colCreatedAt      = "created_at"
	colUpdatedAt      = "updated_at"
	colEmploymentType = "employment_type"
	colStatus         = "status"
	colFields         = "fields"
	colDueDate        = "due_date"
	colEngagementID   = "engagement_id"
	colAmountCents    = "amount_cents"
	colStaffID        = "staff_id"
)

// entity is one file in the archive: a name, its header row, and the
// query that produces it. query must select exactly len(header) columns
// -- each already cast to text (::text, array_to_string(...), or a
// jsonb column, which reads back as text on its own) -- and take
// practiceID as its sole placeholder, repeated as many times as the
// query needs it. Every row is a defense-in-depth practice_id filter or
// join on top of the RLS policy staffauth.Middleware's
// app.current_practice_id already enforces -- the same belt-and-braces
// stance every handler in this codebase takes, never relying on RLS
// alone.
type entity struct {
	file   string
	header []string
	query  string
}

// entities is the whole archive, in the order ADR-0027's brief lists
// them. Adding one later is adding one element here and one test in
// entities_test.go -- never editing writeEntity, which knows nothing
// about what any of these mean.
//
// Excluded throughout, and not a judgment call (issue #288's own
// wording): token_digest, access_code_digest and every other
// credential- or key-shaped column; signed_pdf_object_path and
// attachment_object_path, which point at rendered documents and
// object-store content this ticket leaves out; and every outbox,
// session, token, idempotency-key, rate-limit and raw-webhook table,
// none of which is a domain record a Practice owns.
func entities() []entity {
	return []entity{
		{
			file:   "practice.csv",
			header: []string{"id", "name", "stripe_customer_id", "stripe_connect_account_id", "require_mfa_for_all_staff", colCreatedAt},
			query: `SELECT id::text, name, stripe_customer_id, stripe_connect_account_id,
			               require_mfa_for_all_staff::text, created_at::text
			          FROM practices WHERE id = $1`,
		},
		{
			file:   "practice_website.csv",
			header: []string{"practice_id", "mode", "own_url", "slug", "service_description", "cancellation_policy", "page_state", colCreatedAt, colUpdatedAt},
			query: `SELECT practice_id::text, mode, own_url, slug, service_description, cancellation_policy,
			               page_state::text, created_at::text, updated_at::text
			          FROM practice_websites WHERE practice_id = $1`,
		},
		{
			// invite_token (staff's own bootstrap signup token) and
			// last_practice_id (which Practice she last visited, not
			// specific to this one) are deliberately not selected.
			file:   "staff.csv",
			header: []string{colStaffID, "name", "email", "roles", colEmploymentType, "work_state", "last_active_at", "membership_created_at"},
			query: `SELECT s.id::text, s.name, s.email, array_to_string(pm.roles, ','), pm.employment_type::text,
			               s.work_state, s.last_active_at::text, pm.created_at::text
			          FROM practice_memberships pm
			          JOIN staff s ON s.id = pm.staff_id
			         WHERE pm.practice_id = $1
			         ORDER BY pm.created_at`,
		},
		{
			file:   "staff_invitation.csv",
			header: []string{"id", "address", "roles", colEmploymentType, colStatus, "invited_by_staff_id", colCreatedAt, "expires_at", "revoked_by_staff_id", "revoked_at", "accepted_staff_id", "accepted_at"},
			query: `SELECT id::text, address, array_to_string(roles, ','), employment_type::text, status::text,
			               invited_by::text, created_at::text, expires_at::text, revoked_by::text,
			               revoked_at::text, accepted_staff_id::text, accepted_at::text
			          FROM practice_invitations WHERE practice_id = $1
			         ORDER BY created_at`,
		},
		{
			// field_values is jsonb -- selected bare, not ::text, since a
			// jsonb column already reads back through database/sql as its
			// own text representation, which is already valid JSON.
			file:   "client.csv",
			header: []string{"id", "given_name", "family_name", "preferred_name", "email", "phone", "address_line1", "address_line2", "address_locality", "address_region", "address_postal_code", "date_of_birth", "field_values", "erased_at", "merged_into", colCreatedAt},
			query: `SELECT id::text, given_name, family_name, preferred_name, email, phone,
			               address_line1, address_line2, address_locality, address_region, address_postal_code,
			               date_of_birth::text, field_values::text, erased_at::text, merged_into::text, created_at::text
			          FROM clients WHERE practice_id = $1
			         ORDER BY created_at`,
		},
		{
			file:   "client_field_template.csv",
			header: []string{"practice_id", colFields, colCreatedAt, colUpdatedAt},
			query: `SELECT practice_id::text, fields::text, created_at::text, updated_at::text
			          FROM client_field_templates WHERE practice_id = $1`,
		},
		{
			file:   "engagement.csv",
			header: []string{"id", "client_id", "kind", colStatus, colDueDate, colCreatedAt},
			query: `SELECT id::text, client_id::text, kind::text, status::text, due_date::text, created_at::text
			          FROM engagements WHERE practice_id = $1
			         ORDER BY created_at`,
		},
		{
			file:   "engagement_request.csv",
			header: []string{"id", "client_id", "kind", colDueDate, "note", "state", "requested_by_staff_id", "requested_at", "decided_by_staff_id", "decided_at", "reason", colEngagementID},
			query: `SELECT id::text, client_id::text, kind::text, due_date::text, note, state::text,
			               requested_by::text, requested_at::text, decided_by::text, decided_at::text,
			               reason, engagement_id::text
			          FROM engagement_requests WHERE practice_id = $1
			         ORDER BY requested_at`,
		},
		{
			// access_code_digest is key material (EraseHandler's own
			// exclusion list applies just as much to an unused Offer) and
			// is deliberately not selected here.
			file:   "engagement_offer.csv",
			header: []string{"id", colEngagementID, colStaffID, "invitation_id", colEmploymentType, colAmountCents, "terms", "client_first_initial", "client_area", colDueDate, "state", "offered_by_staff_id", "offered_at", "expires_at", "decided_by_staff_id", "decided_at", "access_code_attempts"},
			query: `SELECT o.id::text, o.engagement_id::text, o.staff_id::text, o.invitation_id::text,
			               o.employment_type::text, o.amount_cents::text, o.terms, o.client_first_initial,
			               o.client_area, o.due_date::text, o.state::text, o.offered_by::text, o.offered_at::text,
			               o.expires_at::text, o.decided_by::text, o.decided_at::text, o.access_code_attempts::text
			          FROM engagement_offers o
			          JOIN engagements e ON e.id = o.engagement_id
			         WHERE e.practice_id = $1
			         ORDER BY o.offered_at`,
		},
		{
			file:   "engagement_attachment.csv",
			header: []string{"id", colEngagementID, colStaffID, "origin", "attached_by_staff_id", "attached_at", "ended_at", "ended_by_staff_id", "fee_amount_cents", "fee_terms"},
			query: `SELECT a.id::text, a.engagement_id::text, a.staff_id::text, a.origin::text,
			               a.attached_by::text, a.attached_at::text, a.ended_at::text, a.ended_by::text,
			               a.fee_amount_cents::text, a.fee_terms
			          FROM engagement_attachments a
			          JOIN engagements e ON e.id = a.engagement_id
			         WHERE e.practice_id = $1
			         ORDER BY a.attached_at`,
		},
		{
			file:   "visit.csv",
			header: []string{"id", colEngagementID, colStaffID, colCreatedAt},
			query: `SELECT v.id::text, v.engagement_id::text, v.staff_id::text, v.created_at::text
			          FROM visits v
			          JOIN engagements e ON e.id = v.engagement_id
			         WHERE e.practice_id = $1
			         ORDER BY v.created_at`,
		},
		{
			file:   "plan_template.csv",
			header: []string{"id", "plan_type", colFields, colCreatedAt},
			query: `SELECT id::text, plan_type::text, fields::text, created_at::text
			          FROM plan_templates WHERE practice_id = $1
			         ORDER BY created_at`,
		},
		{
			file:   "plan_instance.csv",
			header: []string{"id", colEngagementID, "plan_type", colFields, "answers", colCreatedAt},
			query: `SELECT p.id::text, p.engagement_id::text, p.plan_type::text, p.fields::text, p.answers::text,
			               p.created_at::text
			          FROM plan_instances p
			          JOIN engagements e ON e.id = p.engagement_id
			         WHERE e.practice_id = $1
			         ORDER BY p.created_at`,
		},
		{
			file:   "contract_template.csv",
			header: []string{"id", "prose", colCreatedAt},
			query: `SELECT id::text, prose, created_at::text
			          FROM contract_templates WHERE practice_id = $1`,
		},
		{
			// signed_pdf_object_path points at a rendered document, out of
			// scope for this ticket (#288's "not a judgment call" list),
			// and is deliberately not selected.
			file:   "contract.csv",
			header: []string{"id", colEngagementID, colStatus, "prose", "merge_field_values", "signer_full_name", "signer_attestation", "signer_ip", "signed_at", colCreatedAt},
			query: `SELECT c.id::text, c.engagement_id::text, c.status::text, c.prose, c.merge_field_values::text,
			               c.signer_full_name, c.signer_attestation::text, c.signer_ip, c.signed_at::text, c.created_at::text
			          FROM contracts c
			          JOIN engagements e ON e.id = c.engagement_id
			         WHERE e.practice_id = $1
			         ORDER BY c.created_at`,
		},
		{
			file:   "invoice.csv",
			header: []string{"id", "contract_id", "stripe_invoice_id", "stripe_customer_id", colStatus, colAmountCents, "currency", colCreatedAt, "paid_at"},
			query: `SELECT id::text, contract_id::text, stripe_invoice_id, stripe_customer_id, status::text,
			               amount_cents::text, currency, created_at::text, paid_at::text
			          FROM invoices WHERE practice_id = $1
			         ORDER BY created_at`,
		},
		{
			file:   "payment.csv",
			header: []string{"id", "invoice_id", "stripe_payment_reference", colAmountCents, "paid_at", colCreatedAt},
			query: `SELECT p.id::text, p.invoice_id::text, p.stripe_payment_reference, p.amount_cents::text,
			               p.paid_at::text, p.created_at::text
			          FROM payments p
			          JOIN invoices i ON i.id = p.invoice_id
			         WHERE i.practice_id = $1
			         ORDER BY p.created_at`,
		},
		{
			// attachment_object_path points at object-store content, out of
			// scope for this ticket, and is deliberately not selected --
			// attachment_filename/content_type/byte_size still say what it
			// was.
			file:   "message.csv",
			header: []string{"id", colEngagementID, "sender_type", "sender_id", "body", "attachment_filename", "attachment_content_type", "attachment_byte_size", colCreatedAt},
			query: `SELECT m.id::text, m.engagement_id::text, m.sender_type::text, m.sender_id::text, m.body,
			               m.attachment_filename, m.attachment_content_type, m.attachment_byte_size::text,
			               m.created_at::text
			          FROM messages m
			          JOIN engagements e ON e.id = m.engagement_id
			         WHERE e.practice_id = $1
			         ORDER BY m.created_at`,
		},
		{
			file:   "credit_ledger.csv",
			header: []string{"id", "origin", "quantity", "unit_price_cents", "tax_cents", "consumed_engagement_id", "consumed_at", "stripe_payment_intent_id", "stripe_refund_id", "drawn_lot_id", "refund_request_key", "granted_by", colCreatedAt},
			query: `SELECT id::text, origin::text, quantity::text, unit_price_cents::text, tax_cents::text,
			               consumed_engagement_id::text, consumed_at::text, stripe_payment_intent_id,
			               stripe_refund_id, drawn_lot_id::text, refund_request_key, granted_by, created_at::text
			          FROM credit_ledger WHERE practice_id = $1
			         ORDER BY created_at`,
		},
		{
			file:   "notification_preference.csv",
			header: []string{colEngagementID, "identity_uid", "channel", "muted", colCreatedAt, colUpdatedAt},
			query: `SELECT np.engagement_id::text, np.identity_uid, np.channel::text, np.muted::text,
			               np.created_at::text, np.updated_at::text
			          FROM notification_preferences np
			          JOIN engagements e ON e.id = np.engagement_id
			         WHERE e.practice_id = $1
			         ORDER BY np.created_at`,
		},
		{
			// Every address this Practice is responsible for -- her Clients,
			// her Staff, and everyone she has invited -- mirroring
			// mailsuppress.practiceAddressesSQL's own join shape (that
			// package's own const stays unexported, so this restates the
			// same three-way union rather than reaching across the package
			// boundary for it).
			file:   "email_suppression.csv",
			header: []string{"address", "cause", colCreatedAt, "cleared_at", "cleared_by_staff_id"},
			query: `SELECT es.address, es.cause, es.created_at::text, es.cleared_at::text, es.cleared_by::text
			          FROM email_suppressions es
			          JOIN (
			                SELECT lower(c.email) AS address FROM clients c WHERE c.practice_id = $1 AND c.email IS NOT NULL
			                UNION
			                SELECT lower(s.email) FROM staff s JOIN practice_memberships pm ON pm.staff_id = s.id WHERE pm.practice_id = $1
			                UNION
			                SELECT lower(pi.address) FROM practice_invitations pi WHERE pi.practice_id = $1
			               ) a ON a.address = es.address
			         ORDER BY es.created_at`,
		},
		{
			// diff is jsonb: for a sealed row (ADR-0027) it already reads
			// back as clientkey.Sealed's own ciphertext envelope
			// ({"v":1,"enc":"..."}) rather than anything this package ever
			// decrypts -- this package imports neither clientkey nor
			// client_data_keys, so it has no way to. A plaintext diff reads
			// back as whatever JSON its own write site shaped.
			file:   "activity.csv",
			header: []string{"id", "subject_kind", "subject_id", "action", "diff", "actor_kind", "actor_staff_id", "actor_client_id", colCreatedAt},
			query: `SELECT id::text, subject_kind, subject_id::text, action, diff::text, actor_kind::text,
			               actor_staff_id::text, actor_client_id::text, created_at::text
			          FROM activity WHERE practice_id = $1
			         ORDER BY created_at`,
		},
	}
}
