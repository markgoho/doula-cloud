<script lang="ts">
	/*
	 * Intake (#497, ADR-0017). Three pages in one route -- name, contact,
	 * date of birth -- then a save, then the Client detail hub (#494). One
	 * SvelteKit route rather than three: nothing here is a real navigable
	 * destination a person could bookmark or share mid-flow, and #464's
	 * StepRail/QuestionPage pair (the app's "one question per page"
	 * archetype) needs each step to carry its own real `href` -- building
	 * the steps array is explicitly left to "the route's job" and the
	 * all-steps page to a route of its own in StepRail's own comment
	 * (#466), neither of which exists for a single-route sequence with no
	 * per-step URL. So this follows the prototype's shape instead: local
	 * `$state` for the current step, and its own focus management, since a
	 * client-side page change moves no focus on its own (verified on
	 * `prototype/372-intake-screen`, #372's resolution).
	 *
	 * The save waits until page three: it needs all four of ADR-0017's
	 * match keys (name, date of birth, email, phone), because everything
	 * after the save crosses #495's edit path, which blocks a name
	 * substitution and offers only "a different person" -- a key deferred
	 * past the save would make a duplicate nothing can undo.
	 */
	import { goto } from '$app/navigation';
	import { page } from '$app/state';
	import { resolve } from '$app/paths';
	import { apiFetchWithSession } from '#lib/api.js';
	import {
		createClient,
		editClient,
		type ClientCreateFields,
		type ClientEditFields,
		type ClientMatch
	} from '#lib/client.js';
	import { displayName } from '#lib/clientDetail.js';
	import PageTitle from '#lib/components/PageTitle.svelte';
	import Heading from '#lib/components/atoms/Heading.svelte';
	import Text from '#lib/components/atoms/Text.svelte';
	import TextInput from '#lib/components/atoms/TextInput.svelte';
	import Button from '#lib/components/atoms/Button.svelte';
	import LabeledField from '#lib/components/molecules/LabeledField.svelte';
	import ErrorSummary from '#lib/components/molecules/ErrorSummary.svelte';
	import DescriptionList from '#lib/components/molecules/DescriptionList.svelte';
	import ConfirmDialog from '#lib/components/molecules/ConfirmDialog.svelte';
	import { SERVICE_PROBLEM, type FormError } from '#lib/formErrors.js';

	type Step = 'name' | 'contact' | 'dob' | 'match' | 'review';

	const givenNameId = 'intake-given-name';
	const familyNameId = 'intake-family-name';
	const preferredNameId = 'intake-preferred-name';
	const phoneId = 'intake-phone';
	const emailId = 'intake-email';
	const dateOfBirthId = 'intake-date-of-birth';

	/*
	 * The empty state of the search that fronts intake (#498) carries
	 * whatever a staff member typed into it, so a genuinely new Client
	 * costs nothing to search first (ADR-0017, "Finding a returning
	 * Client"). The handoff carries all four of the search's keys, not
	 * only the name: a staff member who searched by phone alone found
	 * nothing and would otherwise retype the one thing she had. Each
	 * lands on the page that asks for it -- the name here, phone and
	 * email on page two, the date of birth on page three -- so a carried
	 * value is still shown and still editable before the save.
	 */
	function carried(key: string): string {
		return page.url.searchParams.get(key)?.trim() ?? '';
	}

	let givenName = $state(carried('name'));
	let familyName = $state('');
	let preferredName = $state('');
	let phone = $state(carried('phone'));
	let email = $state(carried('email'));
	let dateOfBirth = $state(carried('dateOfBirth'));

	let step = $state<Step>('name');
	let pageErrors = $state<FormError[]>([]);
	let isSubmitting = $state(false);
	let matches = $state<ClientMatch[]>([]);
	let reviewMatch = $state<ClientMatch | undefined>();
	let reviewConflictMatches = $state<ClientMatch[]>([]);
	let isReviewConflictOpen = $state(false);

	/*
	 * Page one names the Client because she has no name yet; every screen
	 * after it uses the name she was given (#372's resolution). Falling
	 * back to a domain noun rather than a pronoun keeps that rule even
	 * before a given name exists to read.
	 */
	const knownAs = $derived(preferredName.trim() || givenName.trim() || 'the Client');

	// A client-side step change moves no focus on its own -- see the
	// module comment. Each branch below binds this to its own <h1>
	// wrapper; the effect re-focuses it whenever the step (or which match
	// is under review) changes. `pageErrors` is deliberately not a
	// dependency: ErrorSummary already focuses itself when a refusal
	// appears without a step change (page one's blank-name refusal, or a
	// thrown save error), so re-running this effect for the same reason
	// would only fight it for focus.
	let pageStart = $state<HTMLElement | undefined>();
	$effect(() => {
		void step;
		void matches.length;
		void reviewMatch;
		pageStart?.focus();
	});

	function errorFor(targetId: string): string | undefined {
		return pageErrors.find((entry) => entry.targetId === targetId)?.message;
	}

	function detailHref(clientId: string): string {
		return resolve('/practices/[practiceId]/clients/[clientId]', {
			practiceId: page.params.practiceId!,
			clientId
		});
	}

	async function finish(clientId: string): Promise<void> {
		await goto(detailHref(clientId));
	}

	function currentCreateFields(): ClientCreateFields {
		return {
			givenName: givenName.trim(),
			familyName: familyName.trim(),
			preferredName: preferredName.trim(),
			email: email.trim(),
			phone: phone.trim(),
			dateOfBirth: dateOfBirth.trim()
		};
	}

	interface ProposedChange {
		label: string;
		onFile: string;
		typed: string;
	}

	/*
	 * ADR-0017's "This is her": nothing typed on a blank intake field
	 * overwrites what is already on file, and a field typed the same as
	 * what is on file is not a change worth listing. Only a non-blank,
	 * genuinely different typed value becomes a proposed edit -- the same
	 * rule the save-time prompt's prototype used (`differences()` in
	 * `prototype/372-intake-screen`).
	 */
	function proposedChanges(match: ClientMatch): ProposedChange[] {
		const rows: ProposedChange[] = [];
		function consider(label: string, typedValue: string, onFileValue: string) {
			const typedTrimmed = typedValue.trim();
			if (typedTrimmed && typedTrimmed !== onFileValue) {
				rows.push({ label, onFile: onFileValue || '—', typed: typedTrimmed });
			}
		}
		consider('Given name', givenName, match.givenName);
		consider('Family name', familyName, match.familyName);
		consider('Preferred name', preferredName, match.preferredName);
		consider('Email', email, match.email);
		consider('Phone', phone, match.phone);
		consider('Date of birth', dateOfBirth, match.dateOfBirth);
		return rows;
	}

	// The full-object PUT edit.go expects: the matched Client's own
	// address and Practice-defined values ride through unchanged (#495's
	// hazard -- an edit that does not round-trip `fieldValues` silently
	// wipes them), and only the fields intake actually typed something
	// different into are overridden.
	function mergedEditFields(match: ClientMatch): ClientEditFields {
		return {
			givenName: givenName.trim() || match.givenName,
			familyName: familyName.trim() || match.familyName,
			preferredName: preferredName.trim() || match.preferredName,
			email: email.trim() || match.email,
			phone: phone.trim() || match.phone,
			addressLine1: match.addressLine1,
			addressLine2: match.addressLine2,
			addressLocality: match.addressLocality,
			addressRegion: match.addressRegion,
			addressPostalCode: match.addressPostalCode,
			dateOfBirth: dateOfBirth.trim() || match.dateOfBirth,
			fieldValues: match.fieldValues
		};
	}

	function handleContinueFromName(event: SubmitEvent) {
		event.preventDefault();
		pageErrors = [];
		if (givenName.trim() === '') {
			pageErrors = [{ message: "Enter the Client's given name", targetId: givenNameId }];
			return;
		}
		step = 'contact';
	}

	function handleContinueFromContact(event: SubmitEvent) {
		event.preventDefault();
		step = 'dob';
	}

	async function handleSave(event: SubmitEvent) {
		event.preventDefault();
		pageErrors = [];
		isSubmitting = true;
		try {
			const result = await createClient(apiFetchWithSession, page.params.practiceId!, currentCreateFields(), false);
			if (result.conflict) {
				// ADR-0017: any hit stops the write and forces a choice.
				// Nothing has been saved yet.
				matches = result.matches;
				step = 'match';
				return;
			}
			await finish(result.record.id);
		} catch (error_) {
			pageErrors = [{ message: error_ instanceof Error && error_.message ? error_.message : SERVICE_PROBLEM }];
		} finally {
			isSubmitting = false;
		}
	}

	// "This is <name>": if what was typed matches what is on file exactly
	// (or was left blank), there is nothing to propose -- go straight to
	// her record rather than showing an empty review screen.
	function handleThisIsHer(match: ClientMatch) {
		if (proposedChanges(match).length === 0) {
			void finish(match.id);
			return;
		}
		reviewMatch = match;
		step = 'review';
	}

	// "No, a different person": the one deliberate override on this path
	// (ADR-0017). Override skips CreateHandler's match query entirely, so
	// a conflict here would mean something else refused the write.
	async function handleDifferentPerson() {
		pageErrors = [];
		isSubmitting = true;
		try {
			const result = await createClient(apiFetchWithSession, page.params.practiceId!, currentCreateFields(), true);
			if (result.conflict) {
				pageErrors = [{ message: 'The Client record could not be saved.' }];
				return;
			}
			await finish(result.record.id);
		} catch (error_) {
			pageErrors = [{ message: error_ instanceof Error && error_.message ? error_.message : SERVICE_PROBLEM }];
		} finally {
			isSubmitting = false;
		}
	}

	async function handleSaveReview() {
		if (!reviewMatch) return;
		pageErrors = [];
		isSubmitting = true;
		try {
			const result = await editClient(
				apiFetchWithSession,
				page.params.practiceId!,
				reviewMatch.id,
				mergedEditFields(reviewMatch),
				false
			);
			if (result.conflict) {
				// The typed values also match a third Client -- the match
				// query excludes only reviewMatch's own row (edit.go). Reuse
				// #495's override wiring rather than inventing a second one.
				reviewConflictMatches = result.matches;
				isReviewConflictOpen = true;
				return;
			}
			await finish(reviewMatch.id);
		} catch (error_) {
			pageErrors = [{ message: error_ instanceof Error && error_.message ? error_.message : SERVICE_PROBLEM }];
		} finally {
			isSubmitting = false;
		}
	}

	async function handleReviewOverrideConfirm() {
		if (!reviewMatch) return;
		try {
			const result = await editClient(
				apiFetchWithSession,
				page.params.practiceId!,
				reviewMatch.id,
				mergedEditFields(reviewMatch),
				true
			);
			if (result.conflict) {
				pageErrors = [{ message: 'The Client record could not be saved.' }];
				throw new Error('intake review: unexpected conflict with override set');
			}
			await finish(reviewMatch.id);
		} catch (error_) {
			if (pageErrors.length === 0) {
				pageErrors = [{ message: error_ instanceof Error && error_.message ? error_.message : SERVICE_PROBLEM }];
			}
			throw error_;
		}
	}

	function handleReviewConflictCancel() {
		reviewConflictMatches = [];
	}

	function reviewConflictNames(): string {
		return reviewConflictMatches.map((match) => displayName(match)).join(', ');
	}

	// The question each step's `<h1>` asks -- also this ticket's #487
	// obligation, the browser-tab title, since a client-side step change
	// carries no navigation to hang one off automatically.
	function questionText(): string {
		if (step === 'name') return "What is the Client's name?";
		if (step === 'contact') return `How do you contact ${knownAs}?`;
		if (step === 'dob') return `What is ${knownAs}'s date of birth?`;
		if (step === 'match') return 'Before this is saved';
		return reviewMatch ? `Save changes to ${displayName(reviewMatch)}'s record` : '';
	}

	function stepNumber(): number {
		if (step === 'name') return 1;
		if (step === 'contact') return 2;
		return 3;
	}
</script>

<PageTitle page={questionText()} isError={pageErrors.length > 0} />

<container-l>
	<center-l max="var(--form-max)" gutters="var(--page-gutter)">
		<stack-l space="var(--space-7)">
			{#if (['name', 'contact', 'dob'] as Step[]).includes(step)}
				<!--
					Goal-Gradient (the brief): a three-page sequence has to show
					where the end is. Not a full StepRail -- see the module
					comment on why this route carries no per-step `href` for one
					to link to.

					Stays raw rather than moving onto `Text` (#599): meta size at
					weight 500, uppercased, matches no `step`/`tone` pair, and one
					consumer is under the bar for widening the atom's API -- the
					same single-consumer call #189 made for `invite`'s <code> and
					#182 made for DataTable's per-row action.
				-->
				<p class="crumb">Adding a Client — step {stepNumber()} of 3</p>
			{/if}

			{#if pageErrors.length > 0}
				<ErrorSummary errors={pageErrors} />
			{/if}

			{#if step === 'name'}
				<div bind:this={pageStart} id="intake-heading" tabindex="-1">
					<Heading level={1} variant="page" text={questionText()} />
				</div>
				<form onsubmit={handleContinueFromName} novalidate>
					<stack-l space="var(--space-5)">
						<LabeledField id={givenNameId} label="Given name" error={errorFor(givenNameId)}>
							{#snippet children({ id, describedBy, invalid })}
								<TextInput
									{id}
									{describedBy}
									{invalid}
									value={givenName}
									onInput={(v) => (givenName = v)}
									required
									autocomplete="off"
								/>
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
							hint="What the Client is called day to day, if different"
						>
							{#snippet children({ id, describedBy })}
								<TextInput
									{id}
									{describedBy}
									value={preferredName}
									onInput={(v) => (preferredName = v)}
									autocomplete="off"
								/>
							{/snippet}
						</LabeledField>
						<cluster-l space="var(--space-3)" align="center">
							<Button type="submit" label="Add contact details" />
						</cluster-l>
					</stack-l>
				</form>
			{:else if step === 'contact'}
				<Button variant="secondary" size="sm" label="Back to the name" onClick={() => (step = 'name')} />
				<div bind:this={pageStart} id="intake-heading" tabindex="-1">
					<Heading level={1} variant="page" text={questionText()} />
				</div>
				<Text tone="variant" text="Either one is enough. The other can be added later." />
				<form onsubmit={handleContinueFromContact} novalidate>
					<stack-l space="var(--space-5)">
						<LabeledField id={phoneId} label="Phone">
							{#snippet children({ id, describedBy })}
								<TextInput {id} {describedBy} type="tel" value={phone} onInput={(v) => (phone = v)} autocomplete="off" />
							{/snippet}
						</LabeledField>
						<LabeledField id={emailId} label="Email">
							{#snippet children({ id, describedBy })}
								<TextInput {id} {describedBy} type="email" value={email} onInput={(v) => (email = v)} autocomplete="off" />
							{/snippet}
						</LabeledField>
						<cluster-l space="var(--space-3)" align="center">
							<Button type="submit" label="Add {knownAs}'s date of birth" />
						</cluster-l>
					</stack-l>
				</form>
			{:else if step === 'dob'}
				<Button
					variant="secondary"
					size="sm"
					label="Back to the contact details"
					onClick={() => (step = 'contact')}
				/>
				<div bind:this={pageStart} id="intake-heading" tabindex="-1">
					<Heading level={1} variant="page" text={questionText()} />
				</div>
				<Text
					tone="variant"
					text="This is what separates two Clients with the same name, next year and the year after. It is the last thing asked before the record is saved."
				/>
				<form onsubmit={handleSave} novalidate>
					<stack-l space="var(--space-5)">
						<LabeledField id={dateOfBirthId} label="Date of birth">
							{#snippet children({ id, describedBy })}
								<TextInput
									{id}
									{describedBy}
									type="date"
									value={dateOfBirth}
									onInput={(v) => (dateOfBirth = v)}
									autocomplete="off"
								/>
							{/snippet}
						</LabeledField>
						<cluster-l space="var(--space-3)" align="center">
							<Button type="submit" label="Save {knownAs}'s record" loading={isSubmitting} />
						</cluster-l>
					</stack-l>
				</form>
			{:else if step === 'match'}
				<div bind:this={pageStart} id="intake-heading" tabindex="-1">
					<Heading level={1} variant="page" text={questionText()} />
				</div>
				<Text
					tone="variant"
					text="Nothing has been saved. {matches.length === 1
						? 'One record'
						: `${matches.length} records`} at this Practice matched what you typed."
				/>
				<stack-l space="var(--space-6)">
					{#each matches as match (match.id)}
						<section class="match">
							<stack-l space="var(--space-3)">
								<Heading level={2} variant="card" text={displayName(match)} />
								<DescriptionList
									items={[
										{ label: 'Date of birth', value: match.dateOfBirth || '—' },
										{ label: 'Email', value: match.email || '—' },
										{ label: 'Phone', value: match.phone || '—' }
									]}
								/>
								{#if match.engagements.length > 0}
									<ul>
										{#each match.engagements as engagement (engagement.engagementId)}
											<li>{engagement.kind === 'birth' ? 'Birth' : 'Postpartum'} · {engagement.status}</li>
										{/each}
									</ul>
								{:else}
									<Text
										step="body-sm"
										tone="muted"
										text="No Engagements with this Client yet."
									/>
								{/if}
								<Button label="This is {displayName(match)}" onClick={() => handleThisIsHer(match)} />
							</stack-l>
						</section>
					{/each}
					<Text
						step="body-sm"
						tone="muted"
						text="If none of these is the person being added, say so. This is the only way to create a second record for the same name."
					/>
					<Button
						label="No, a different person"
						variant="secondary"
						loading={isSubmitting}
						onClick={handleDifferentPerson}
					/>
				</stack-l>
			{:else if step === 'review' && reviewMatch}
				<Button variant="secondary" size="sm" label="Back to the possible matches" onClick={() => (step = 'match')} />
				<div bind:this={pageStart} id="intake-heading" tabindex="-1">
					<Heading level={1} variant="page" text={questionText()} />
				</div>
				<Text
					tone="variant"
					text="{displayName(reviewMatch)}'s record is kept. What was typed applies as these changes to it."
				/>
				<DescriptionList
					items={proposedChanges(reviewMatch).map((change) => ({
						label: change.label,
						value: `${change.onFile} → ${change.typed}`
					}))}
				/>
				<cluster-l space="var(--space-3)" align="center">
					<Button
						label="Save changes to {displayName(reviewMatch)}'s record"
						loading={isSubmitting}
						onClick={handleSaveReview}
					/>
				</cluster-l>
			{/if}
		</stack-l>
	</center-l>
</container-l>

<ConfirmDialog
	bind:open={isReviewConflictOpen}
	title="Possible duplicate Client"
	consequence={`These changes also match another existing Client at this Practice: ${reviewConflictNames()}. Saving keeps them as two separate records -- there is no way to merge them here.`}
	confirmLabel="Save as a different person"
	onConfirm={handleReviewOverrideConfirm}
	onCancel={handleReviewConflictCancel}
/>

<style>
	@layer components {
		container-l {
			padding-block: var(--space-8);
		}

		/*
		 * Focused programmatically on every step change, never by the
		 * keyboard -- so, per ErrorSummary's own note, `:focus-visible`
		 * would never fire here and the ring has to be `:focus` instead.
		 */
		[tabindex='-1']:focus {
			outline: var(--focus-ring-width) solid var(--color-primary);
			outline-offset: var(--focus-ring-offset);
		}

		.crumb {
			margin: 0;
			font-size: var(--text-meta-size);
			font-weight: var(--font-weight-medium);
			line-height: var(--text-meta-leading);
			letter-spacing: var(--text-meta-tracking);
			color: var(--color-on-surface-muted);
			text-transform: uppercase;
		}

		.match {
			padding: var(--space-4);
			border: var(--border-thin) solid var(--color-outline-variant);
			border-radius: var(--radius);
		}

		ul {
			margin: 0;
			padding-inline-start: var(--space-5);
			font-size: var(--text-body-sm-size);
		}
	}
</style>
