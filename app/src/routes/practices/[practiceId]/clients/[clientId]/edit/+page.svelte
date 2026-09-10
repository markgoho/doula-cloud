<script lang="ts">
	/*
	 * Client edit (#495, ADR-0017). Pre-fills the twelve structural
	 * columns from her current record and PUTs a full replacement
	 * (api/internal/client/edit.go). There is no editor here for her
	 * Practice-defined values yet -- #495 is scoped to the structural
	 * core -- so `fieldValues` is carried through unchanged from the load
	 * rather than sent empty, which would silently wipe them (see
	 * ClientRecord.fieldValues in clientDetail.ts).
	 *
	 * The collision predicate re-runs on every save (`override: false`
	 * first) and sorts a hit into ADR-0017's amendment's two gates
	 * (#814), told apart by the 409's own `substitution` flag:
	 *
	 * - Gate one, substitution -- exactly `substitution: true`. A name
	 *   column changed and the result is exactly another Client's given
	 *   and family name. The one deliberate override -- "Yes, a different
	 *   person" -- retries with `override: true`, via ConfirmDialog, the
	 *   app's one confirmation mechanism (#473). It is never a
	 *   pre-checked box, and on this path nothing is created: the record
	 *   being edited already exists and simply keeps its own identity.
	 * - Gate two, a possible duplicate -- `substitution: false`. Nothing
	 *   is written; the reader is sent to this route's own `duplicate`
	 *   sub-route, a question page rather than a dismissible dialog, per
	 *   the ADR. `mergeOffered` there says whether "This is her" (which
	 *   absorbs one record into the other) is even offered.
	 */
	import { onMount, tick } from 'svelte';
	import { goto } from '$app/navigation';
	import { page } from '#lib/appState.svelte.js';
	import { resolve } from '$app/paths';
	import { apiFetchWithSession } from '#lib/api.js';
	import { editClient, type ClientEditFields, type CollisionMatch } from '#lib/client.js';
	import { displayName, loadClientDetail, type ClientDetail } from '#lib/clientDetail.js';
	import FormPage from '#lib/components/templates/FormPage.svelte';
	import Button from '#lib/components/atoms/Button.svelte';
	import Link from '#lib/components/atoms/Link.svelte';
	import Notice from '#lib/components/atoms/Notice.svelte';
	import TextInput from '#lib/components/atoms/TextInput.svelte';
	import LabeledField from '#lib/components/molecules/LabeledField.svelte';
	import ErrorSummary from '#lib/components/molecules/ErrorSummary.svelte';
	import ConfirmDialog from '#lib/components/molecules/ConfirmDialog.svelte';
	import { errorsFromCause } from '#lib/formErrors.js';
	import { FormSubmission, orThrownErrors, type FormError } from '#lib/formSubmission.svelte.js';
	import { editMergeDraft } from '#lib/editMergeDraft.svelte.js';

	const givenNameId = 'client-edit-given-name';
	const familyNameId = 'client-edit-family-name';
	const preferredNameId = 'client-edit-preferred-name';
	const emailId = 'client-edit-email';
	const phoneId = 'client-edit-phone';
	const addressLine1Id = 'client-edit-address-line1';
	const addressLine2Id = 'client-edit-address-line2';
	const addressLocalityId = 'client-edit-address-locality';
	const addressRegionId = 'client-edit-address-region';
	const addressPostalCodeId = 'client-edit-address-postal-code';
	const dateOfBirthId = 'client-edit-date-of-birth';
	// The BFF's own field names (client.Record's json tags) mapped onto
	// this form's controls, so a refusal it names lands on the right one
	// (#488). Only these two can be refused server-side.
	const editFieldIds = { givenName: givenNameId, dateOfBirth: dateOfBirthId };

	let detail = $state<ClientDetail | undefined>();
	let loadError = $state('');

	let givenName = $state('');
	let familyName = $state('');
	let preferredName = $state('');
	let email = $state('');
	let phone = $state('');
	let addressLine1 = $state('');
	let addressLine2 = $state('');
	let addressLocality = $state('');
	let addressRegion = $state('');
	let addressPostalCode = $state('');
	let dateOfBirth = $state('');
	let fieldValues = $state<unknown>();

	const submission = new FormSubmission();
	let matches = $state<CollisionMatch[]>([]);
	let isConflictOpen = $state(false);
	// The refused override that belongs to no control, rendered inside the
	// dialog rather than in the page behind it (#1082, below). Deliberately
	// not `submission.errors`: an entry left there would sit unread behind
	// the backdrop and then appear unannounced the moment she cancels.
	let overrideError = $state('');

	// AC5: editing the email revokes any pending portal invite
	// (portalinvite/outbox.go's live-read-at-send rule) -- shown here
	// rather than discovered after the fact.
	const hasChangedEmail = $derived(detail !== undefined && email.trim() !== detail.email);

	function detailHref(clientId: string = page.params.clientId!): string {
		return resolve('/practices/[practiceId]/clients/[clientId]', {
			practiceId: page.params.practiceId!,
			clientId
		});
	}

	function editHref(): string {
		return resolve('/practices/[practiceId]/clients/[clientId]/edit', {
			practiceId: page.params.practiceId!,
			clientId: page.params.clientId!
		});
	}

	function matchNames(): string {
		return matches.map((match) => displayName(match)).join(', ');
	}

	function currentFields(): ClientEditFields {
		return {
			givenName,
			familyName,
			preferredName,
			email,
			phone,
			addressLine1,
			addressLine2,
			addressLocality,
			addressRegion,
			addressPostalCode,
			dateOfBirth,
			fieldValues
		};
	}

	function findRefusals(): FormError[] {
		const found: FormError[] = [];
		if (givenName.trim() === '') found.push({ message: "Enter the Client's given name", targetId: givenNameId });
		return found;
	}

	onMount(async () => {
		try {
			detail = await loadClientDetail(apiFetchWithSession, page.params.practiceId!, page.params.clientId!);
			// A tombstoned row is redirected, never rendered (ADR-0017's
			// amendment): every other field on this response is meaningless
			// placeholder data once mergedInto is set (detail.go).
			if (detail.mergedInto) {
				await goto(detailHref(detail.mergedInto));
				return;
			}
			givenName = detail.givenName;
			familyName = detail.familyName;
			preferredName = detail.preferredName;
			email = detail.email;
			phone = detail.phone;
			addressLine1 = detail.addressLine1;
			addressLine2 = detail.addressLine2;
			addressLocality = detail.addressLocality;
			addressRegion = detail.addressRegion;
			addressPostalCode = detail.addressPostalCode;
			dateOfBirth = detail.dateOfBirth;
			fieldValues = detail.fieldValues;
		} catch (error_) {
			loadError = error_ instanceof Error ? error_.message : 'Failed to load Client';
		}
	});

	async function handleSubmit(event: SubmitEvent) {
		event.preventDefault();
		matches = [];
		// Escape closes the dialog without reaching onCancel, so a stale
		// refusal is cleared here too rather than only on Cancel.
		overrideError = '';

		await submission.run(async () => {
			const refusals = findRefusals();
			if (refusals.length > 0) return refusals;

			const result = await editClient(
				apiFetchWithSession,
				page.params.practiceId!,
				page.params.clientId!,
				currentFields(),
				false
			);
			if (result.conflict) {
				// Named before anything is written, and nothing here was
				// destructive -- the collision predicate ran, found a hit, and
				// the endpoint wrote nothing (edit.go). Gate one (an exact name
				// substitution) blocks with the existing dialog; gate two (a
				// possible duplicate) is a real question page, not a dialog,
				// per ADR-0017's amendment.
				if (result.substitution) {
					matches = result.matches;
					isConflictOpen = true;
					return;
				}
				editMergeDraft.open(page.params.clientId!, currentFields(), result.matches, result.mergeOffered);
				await goto(`${editHref()}/duplicate`);
				return;
			}
			await goto(detailHref());
		}, orThrownErrors(editFieldIds));
	}

	// The single deliberate override -- ConfirmDialog's onConfirm, reached
	// only by pressing its named button, never a pre-checked box. Retries
	// the same save with override: true, which the endpoint applies by
	// skipping the match query entirely (edit.go), so a conflict here would
	// mean something else refused the write -- surfaced rather than
	// swallowed (AC4).
	//
	// Where that refusal is surfaced is decided by its own shape (#1082),
	// and the two answers are not a preference between them. The dialog is
	// a native <dialog> held open by showModal(), so it is in the top layer
	// above a ::backdrop and the whole page behind it is inert:
	//
	// - A refusal that names no control -- a 5xx, a dropped connection, the
	//   unexpected conflict below -- is this control's own operation
	//   outcome (#467's distinction), so it stays in the dialog as
	//   ConfirmDialog's `error` Notice and the rejection keeps the dialog
	//   open over it (#804's contract).
	// - A refusal that names one is a refused form, and the fix is on the
	//   form behind. It cannot be shown here: ErrorSummary's entries are
	//   fragment links, and HTML's fragment-focusing steps cannot focus an
	//   inert element, so every entry would be the link to nowhere useful
	//   ErrorSummary's own contract forbids -- and a second summary is
	//   exactly what #467's split exists to prevent. So the dialog closes
	//   first, and only once it has does `submission.errors` get set, so
	//   ErrorSummary's focus effect runs against a page that is no longer
	//   inert. Setting it while the dialog is still open spends that effect
	//   on nothing and it does not fire again on close.
	async function handleOverrideConfirm() {
		overrideError = '';
		let refused: FormError[] | undefined;
		try {
			const result = await editClient(
				apiFetchWithSession,
				page.params.practiceId!,
				page.params.clientId!,
				currentFields(),
				true
			);
			// override: true skips the match query entirely, so a conflict
			// here means something else refused the write. It belongs to no
			// control on the form, so it reads as the dialog's own outcome.
			if (result.conflict) refused = [{ message: 'The Client record could not be saved.' }];
		} catch (error_) {
			refused = errorsFromCause(error_, editFieldIds);
		}

		if (refused === undefined) {
			await goto(detailHref());
			return;
		}

		if (refused.some((entry) => entry.targetId !== undefined)) {
			isConflictOpen = false;
			matches = [];
			await tick();
			submission.errors = refused;
			return;
		}

		overrideError = refused[0]!.message;
		// Rethrown so ConfirmDialog leaves the dialog open over the failure,
		// with the Notice it now carries readable inside it.
		throw new Error(overrideError);
	}

	function handleConflictCancel() {
		matches = [];
		overrideError = '';
	}
</script>

{#snippet errorSummary()}
	<ErrorSummary errors={submission.errors} />
{/snippet}

{#snippet structuralFields()}
	<!--
		autocomplete="off" throughout (#469): this asks about the Client,
		not the signed-in Staff member's own saved details.
	-->
	<LabeledField id={givenNameId} label="Given name" error={submission.errorFor(givenNameId)}>
		{#snippet children({ id, describedBy, invalid })}
			<TextInput {id} {describedBy} {invalid} value={givenName} onInput={(v) => (givenName = v)} required autocomplete="off" />
		{/snippet}
	</LabeledField>
	<LabeledField id={familyNameId} label="Family name">
		{#snippet children({ id, describedBy })}
			<TextInput {id} {describedBy} value={familyName} onInput={(v) => (familyName = v)} autocomplete="off" />
		{/snippet}
	</LabeledField>
	<LabeledField
		id={preferredNameId}
		label="Preferred name"
		hint={`What ${detail!.givenName} is called day to day, if different`}
	>
		{#snippet children({ id, describedBy })}
			<TextInput {id} {describedBy} value={preferredName} onInput={(v) => (preferredName = v)} autocomplete="off" />
		{/snippet}
	</LabeledField>
	<LabeledField id={emailId} label="Email">
		{#snippet children({ id, describedBy })}
			<TextInput {id} {describedBy} type="email" value={email} onInput={(v) => (email = v)} autocomplete="off" />
		{/snippet}
	</LabeledField>
	{#if hasChangedEmail}
		<Notice
			variant="info"
			message={`Saving with this email revokes any pending portal invite sent to ${detail!.givenName}'s old address.`}
		/>
	{/if}
	<LabeledField id={phoneId} label="Phone">
		{#snippet children({ id, describedBy })}
			<TextInput {id} {describedBy} type="tel" value={phone} onInput={(v) => (phone = v)} autocomplete="off" />
		{/snippet}
	</LabeledField>
	<LabeledField id={addressLine1Id} label="Address line 1">
		{#snippet children({ id, describedBy })}
			<TextInput {id} {describedBy} value={addressLine1} onInput={(v) => (addressLine1 = v)} autocomplete="off" />
		{/snippet}
	</LabeledField>
	<LabeledField id={addressLine2Id} label="Address line 2">
		{#snippet children({ id, describedBy })}
			<TextInput {id} {describedBy} value={addressLine2} onInput={(v) => (addressLine2 = v)} autocomplete="off" />
		{/snippet}
	</LabeledField>
	<LabeledField id={addressLocalityId} label="Town or city">
		{#snippet children({ id, describedBy })}
			<TextInput {id} {describedBy} value={addressLocality} onInput={(v) => (addressLocality = v)} autocomplete="off" />
		{/snippet}
	</LabeledField>
	<LabeledField id={addressRegionId} label="State">
		{#snippet children({ id, describedBy })}
			<TextInput {id} {describedBy} value={addressRegion} onInput={(v) => (addressRegion = v)} autocomplete="off" />
		{/snippet}
	</LabeledField>
	<LabeledField id={addressPostalCodeId} label="Postal code">
		{#snippet children({ id, describedBy })}
			<TextInput {id} {describedBy} value={addressPostalCode} onInput={(v) => (addressPostalCode = v)} autocomplete="off" />
		{/snippet}
	</LabeledField>
	<LabeledField id={dateOfBirthId} label="Date of birth">
		{#snippet children({ id, describedBy })}
			<TextInput {id} {describedBy} type="date" value={dateOfBirth} onInput={(v) => (dateOfBirth = v)} autocomplete="off" />
		{/snippet}
	</LabeledField>
{/snippet}

{#snippet formActions()}
	<Button type="submit" label="Save" loading={submission.isSubmitting} />
	<Link href={detailHref()} label="Cancel" variant="secondary" />
{/snippet}

<form onsubmit={handleSubmit} novalidate>
	<FormPage
		title={detail ? `Edit ${displayName(detail)}` : 'Edit Client'}
		fieldsets={detail ? [{ content: structuralFields }] : []}
		errorSummary={submission.errors.length > 0 ? errorSummary : undefined}
		actions={formActions}
		loading={detail || loadError ? undefined : 'Loading the Client'}
		loadError={loadError || undefined}
	/>
</form>

<ConfirmDialog
	bind:open={isConflictOpen}
	title="Possible duplicate Client"
	consequence={`This name exactly matches an existing Client at this Practice: ${matchNames()}. Saving keeps this as its own separate record -- nothing here is merged.`}
	confirmLabel="Yes, a different person"
	error={overrideError || undefined}
	onConfirm={handleOverrideConfirm}
	onCancel={handleConflictCancel}
/>
