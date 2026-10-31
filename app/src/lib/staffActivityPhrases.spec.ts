import { describe, expect, it } from 'vitest';

import { staffActivityPhrase, staffActivityPhrasedActions } from './staffActivityPhrases.js';

/*
 * Every action's Staff wording, with the subject's name (the Practice-wide
 * feed) and without it (the Engagement page's own ledger, where the page
 * already names the Client). A Practice's own rows name no person, so both
 * columns read the same.
 */
const WORDING: readonly (readonly [action: string, named: string, unnamed: string])[] = [
	['absorbed', "Amara's duplicate record merged into another", "The Client's duplicate record merged into another"],
	['billing_mode_changed', 'How the Practice takes payment changed', 'How the Practice takes payment changed'],
	['birth_outcome_recorded', 'Birth outcome recorded for Amara', 'Birth outcome recorded for the Client'],
	['birth_plan_acknowledged', 'Birth Plan marked as read by Amara', 'Birth Plan marked as read by the Client'],
	['birth_plan_edited', 'Birth Plan for Amara changed', 'Birth Plan for the Client changed'],
	['birth_plan_started', 'Birth Plan for Amara started', 'Birth Plan for the Client started'],
	['care_phase_changed', 'Care phase changed for Amara', 'Care phase changed for the Client'],
	['care_plan_edited', 'Care Plan for Amara changed', 'Care Plan for the Client changed'],
	['care_plan_started', 'Care Plan for Amara started', 'Care Plan for the Client started'],
	['contract_amount_overridden', 'Contract price for Amara changed by hand', 'Contract price for the Client changed by hand'],
	[
		'contract_amount_repriced',
		'Contract price for Amara changed to the new rate',
		'Contract price for the Client changed to the new rate'
	],
	['contract_created', 'Contract drafted for Amara', 'Contract drafted for the Client'],
	['contract_priced', 'Price set on the Contract for Amara', 'Price set on the Contract for the Client'],
	['contract_sent', 'Contract sent to Amara', 'Contract sent to the Client'],
	['contract_signed', 'Contract signed by Amara', 'Contract signed by the Client'],
	[
		'contract_void_declined',
		'Request to void the Contract for Amara declined',
		'Request to void the Contract for the Client declined'
	],
	['contract_void_requested', 'Asked to void the Contract for Amara', 'Asked to void the Contract for the Client'],
	['contract_voided', 'Contract for Amara voided', 'Contract for the Client voided'],
	['coverage_gap_cleared', 'Gap in on-call cover closed for Amara', 'Gap in on-call cover closed for the Client'],
	['coverage_gap_created', 'Gap in on-call cover added for Amara', 'Gap in on-call cover added for the Client'],
	['coverage_gap_updated', 'Gap in on-call cover changed for Amara', 'Gap in on-call cover changed for the Client'],
	['created', 'Client record created for Amara', 'Client record created for the Client'],
	['doula_attached', "Doula added to Amara's care", "Doula added to the Client's care"],
	['due_date_changed', 'Due date changed for Amara', 'Due date changed for the Client'],
	['employment_type_changed', 'Employment type changed for Amara', 'Employment type changed for a Staff member'],
	['engagement_completed', 'Care for Amara ended', 'Care for the Client ended'],
	['engagement_created', 'Care for Amara started', 'Care for the Client started'],
	['engagement_reopened', 'Care for Amara reopened', 'Care for the Client reopened'],
	['erased', 'Client record erased', 'Client record erased'],
	['invoice_paid', 'Invoice for Amara paid', 'Invoice for the Client paid'],
	['invoice_payment_failed', 'Payment from Amara did not go through', 'Payment from the Client did not go through'],
	['invoice_raised', 'Invoice created for Amara', 'Invoice created for the Client'],
	['invoice_voided', 'Invoice for Amara voided', 'Invoice for the Client voided'],
	['invoice_written_off', 'Invoice for Amara written off', 'Invoice for the Client written off'],
	['joined', 'Amara joined the Practice', 'A Staff member joined the Practice'],
	['kind_changed', 'Type of care changed for Amara', 'Type of care changed for the Client'],
	['merged', "Another record merged into Amara's", "Another record merged into the Client's"],
	['mfa_required_disabled', 'Two-factor authentication no longer a must for all Staff', 'Two-factor authentication no longer a must for all Staff'],
	['mfa_required_enabled', 'Two-factor authentication made a must for all Staff', 'Two-factor authentication made a must for all Staff'],
	['offer_accepted', "Offer for Amara's care accepted", "Offer for the Client's care accepted"],
	['offer_declined', "Offer for Amara's care declined", "Offer for the Client's care declined"],
	['offer_sent', "Offer sent for Amara's care", "Offer sent for the Client's care"],
	['offer_superseded', "Offer for Amara's care replaced by a newer one", "Offer for the Client's care replaced by a newer one"],
	['offer_withdrawn', "Offer for Amara's care withdrawn", "Offer for the Client's care withdrawn"],
	['on_call_narrowing_changed', 'Doulas on call for Amara changed', 'Doulas on call for the Client changed'],
	['on_call_rule_changed', 'On-call rule for Amara changed', 'On-call rule for the Client changed'],
	['payment_recorded', 'Payment from Amara recorded', 'Payment from the Client recorded'],
	['payment_refunded', 'Money returned to Amara', 'Money returned to the Client'],
	['payment_reversed', 'Payment from Amara removed', 'Payment from the Client removed'],
	['payment_terms_changed', 'Payment terms changed', 'Payment terms changed'],
	['portal_account_linked', "Care added to Amara's login", "Care added to the Client's login"],
	['portal_account_provisioned', 'Login created for Amara', 'Login created for the Client'],
	['portal_invite_sent', 'Amara invited to sign in', 'The Client invited to sign in'],
	['portal_sessions_ended', 'Amara signed out of every device', 'The Client signed out of every device'],
	['portal_sign_in_address_changed', 'Sign-in email changed by Amara', 'Sign-in email changed by the Client'],
	['practice_data_exported', 'Practice data downloaded', 'Practice data downloaded'],
	['practice_deletion_finalized', 'Practice deleted', 'Practice deleted'],
	['practice_deletion_requested', 'Practice set to be deleted', 'Practice set to be deleted'],
	['practice_deletion_restored', 'Practice deletion canceled', 'Practice deletion canceled'],
	['practice_name_changed', 'Practice name changed', 'Practice name changed'],
	['practice_on_call_changed', 'On-call settings changed', 'On-call settings changed'],
	['practice_rate_changed', 'Practice rates changed', 'Practice rates changed'],
	['practice_timezone_changed', 'Practice time zone changed', 'Practice time zone changed'],
	['push_notifications_disabled', 'Notifications turned off by Amara', 'Notifications turned off by the Client'],
	['push_notifications_enabled', 'Notifications turned on by Amara', 'Notifications turned on by the Client'],
	['removed', 'Amara removed from the Practice', 'A Staff member removed from the Practice'],
	['roles_changed', 'Roles changed for Amara', 'Roles changed for a Staff member'],
	['second_factor_refused', 'Stopped until two-factor authentication is set up', 'Stopped until two-factor authentication is set up'],
	['sessions_ended', 'Amara signed out of every device', 'A Staff member signed out of every device'],
	['stripe_connect_form_returned', 'Came back from the Stripe setup form', 'Came back from the Stripe setup form'],
	['stripe_connect_nudge_requested', 'Practice Owners asked to connect Stripe', 'Practice Owners asked to connect Stripe'],
	['terms_accepted', 'Terms of Service and Privacy Policy agreed to', 'Terms of Service and Privacy Policy agreed to'],
	['updated', 'Client record for Amara updated', 'Client record for the Client updated'],
	['visit_logged', 'Visit added for Amara', 'Visit added for the Client'],
	['visit_notes_edited', 'Visit notes for Amara updated', 'Visit notes for the Client updated'],
	['visit_reassigned', 'Visit for Amara given to another doula', 'Visit for the Client given to another doula'],
	['visit_scheduled', 'Visit time for Amara updated', 'Visit time for the Client updated'],
	['welcome_credits_email_sent', 'Practice Owners emailed about the Welcome credits', 'Practice Owners emailed about the Welcome credits']
];

describe('staffActivityPhrase', () => {
	it.each(WORDING)('words %s with the name and without it', (action, named, unnamed) => {
		expect(staffActivityPhrase(action, 'Amara')).toBe(named);
		expect(staffActivityPhrase(action)).toBe(unnamed);
	});

	it('reads an empty name as no name', () => {
		expect(staffActivityPhrase('contract_sent', '')).toBe('Contract sent to the Client');
	});

	it('covers every phrased action in the table above', () => {
		expect(WORDING.map(([action]) => action)).toEqual(staffActivityPhrasedActions());
	});

	it('refuses an action it holds no words for, rather than printing it', () => {
		expect(() => staffActivityPhrase('some_new_action')).toThrow(
			'staffActivityPhrases: no Staff wording for activity action "some_new_action"'
		);
	});
});

