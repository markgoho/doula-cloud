<script lang="ts">
	/*
	 * The search that fronts intake (#498, ADR-0017): "Clients -> Add a
	 * Client -> search -> her record -> Request Engagement start". There
	 * is no top-level "Add a Client" action anywhere else in the product
	 * -- the Clients list's own button lands here, not on intake directly
	 * (`clients/+page.svelte`), so a returning Client is found rather
	 * than retyped, and searching costs nothing when she genuinely is
	 * new.
	 *
	 * A miss carries the typed name into intake's first page via `?name=`
	 * -- `clients/new/+page.svelte`'s own module comment reads that
	 * param. Only `name` carries: intake's first page asks for a name
	 * alone, so an email, phone or date of birth typed here has no field
	 * on that page to land in.
	 */
	import { page } from '$app/state';
	import { resolve } from '$app/paths';
	import { apiFetchWithSession } from '#lib/api.js';
	import { searchClients, type ClientMatch, type ClientSearchFields } from '#lib/client.js';
	import { displayName } from '#lib/clientDetail.js';
	import PageTitle from '#lib/components/PageTitle.svelte';
	import Heading from '#lib/components/atoms/Heading.svelte';
	import TextInput from '#lib/components/atoms/TextInput.svelte';
	import Button from '#lib/components/atoms/Button.svelte';
	import Link from '#lib/components/atoms/Link.svelte';
	import LabeledField from '#lib/components/molecules/LabeledField.svelte';
	import ErrorSummary from '#lib/components/molecules/ErrorSummary.svelte';
	import DescriptionList from '#lib/components/molecules/DescriptionList.svelte';
	import { SERVICE_PROBLEM, type FormError } from '#lib/formErrors.js';
	import type { PageProps as PageProperties } from './$types';

	// #501 (ADR-0017): +page.ts's load already decided, before this
	// component mounted, whether the caller is a contractor Doula with
	// neither the owner nor the admin role. `data` is optional only so
	// this component's own spec (which renders it directly, bypassing
	// SvelteKit's load cycle) keeps working without a fixture -- every
	// real navigation always supplies it.
	let { data }: { data?: PageProperties['data'] } = $props();
	const isContractorDoor = $derived(data?.isContractor ?? false);

	const nameId = 'client-search-name';
	const dateOfBirthId = 'client-search-date-of-birth';
	const emailId = 'client-search-email';
	const phoneId = 'client-search-phone';

	let name = $state('');
	let dateOfBirth = $state('');
	let email = $state('');
	let phone = $state('');

	let pageErrors = $state<FormError[]>([]);
	let isSearching = $state(false);
	let hasSearched = $state(false);
	let matches = $state<ClientMatch[]>([]);
	let searchToken = $state(0);

	/*
	 * A client-side result set carries no navigation, so nothing moves
	 * focus to it on its own (the same fact intake's own module comment
	 * names for a step change). `searchToken` increments on every
	 * completed search -- including a repeat search with the same typed
	 * values -- so the results heading is re-focused and re-announced
	 * every time, not only the first.
	 */
	let resultsStart = $state<HTMLElement | undefined>();
	$effect(() => {
		void searchToken;
		resultsStart?.focus();
	});

	function errorFor(targetId: string): string | undefined {
		return pageErrors.find((entry) => entry.targetId === targetId)?.message;
	}

	function currentFields(): ClientSearchFields {
		return {
			name: name.trim(),
			dateOfBirth: dateOfBirth.trim(),
			email: email.trim(),
			phone: phone.trim()
		};
	}

	function detailHref(clientId: string): string {
		return resolve('/practices/[practiceId]/clients/[clientId]', {
			practiceId: page.params.practiceId!,
			clientId
		});
	}

	/*
	 * The miss's next step: intake's first page, carrying every key that
	 * was typed here so none of it is retyped (see the module comment).
	 * All four, not only the name -- a staff member with nothing but a
	 * phone number searches on it, finds no one, and would otherwise lose
	 * the one thing she had. Intake reads each on mount and shows it on
	 * the page that asks for it.
	 */
	function startIntakeHref(): string {
		const base = resolve('/practices/[practiceId]/clients/new', {
			practiceId: page.params.practiceId!
		});
		const fields = currentFields();
		// Built by hand rather than through URLSearchParams: this is a
		// throwaway local, and svelte/prefer-svelte-reactivity rightly
		// refuses a mutable one. encodeURIComponent also writes a space as
		// %20 rather than "+", which reads better in the address bar.
		const query = (
			[
				['name', fields.name],
				['dateOfBirth', fields.dateOfBirth],
				['email', fields.email],
				['phone', fields.phone]
			] as const
		)
			.filter(([, value]) => value)
			.map(([key, value]) => `${key}=${encodeURIComponent(value)}`)
			.join('&');
		return query ? `${base}?${query}` : base;
	}

	async function handleSearch(event: SubmitEvent) {
		event.preventDefault();
		pageErrors = [];
		const fields = currentFields();
		if (!fields.name && !fields.dateOfBirth && !fields.email && !fields.phone) {
			pageErrors = [
				{ message: 'Enter a name, date of birth, email or phone to search', targetId: nameId }
			];
			return;
		}
		isSearching = true;
		try {
			matches = await searchClients(apiFetchWithSession, page.params.practiceId!, fields);
			hasSearched = true;
			searchToken += 1;
		} catch (error_) {
			// SearchHandler refuses a contractor Doula with a readable 403
			// body (client/search.go), which lands here as plain error text
			// rather than as a raw crash. #501's load-gate above intercepts
			// her before this branch is ever reached in practice; this stays
			// as the fallback for any other refusal this form can hit.
			pageErrors = [
				{ message: error_ instanceof Error && error_.message ? error_.message : SERVICE_PROBLEM }
			];
		} finally {
			isSearching = false;
		}
	}

	function resultsHeading(): string {
		if (matches.length === 0) return 'No matches';
		return matches.length === 1 ? '1 match' : `${matches.length} matches`;
	}
</script>

{#if isContractorDoor}
	<!--
		#501 (ADR-0017): "a contractor originates nothing" -- a contractor
		Doula's Add a Client is a door that only explains, in place of the
		search screen SearchHandler would otherwise refuse her from with a
		403. Every route below still exists, and this branch never calls
		any of them.
	-->
	<PageTitle page="Add a Client" />

	<container-l>
		<center-l max="var(--form-max)" gutters="var(--page-gutter)">
			<stack-l space="var(--space-6)">
				<Heading level={1} variant="page" text="Add a Client" />
				<p class="lede">
					Work at this Practice reaches you as an Offer, so there is no Client to search for or
					add here. To take on Clients of your own, set up a Practice.
				</p>
				<cluster-l space="var(--space-3)" align="center">
					<Link href={resolve('/(signed-out)/signup')} label="Set up a Practice" />
				</cluster-l>
			</stack-l>
		</center-l>
	</container-l>
{:else}
	<PageTitle page="Find a Client" isError={pageErrors.length > 0} />

	<container-l>
		<center-l max="var(--form-max)" gutters="var(--page-gutter)">
			<stack-l space="var(--space-7)">
				<Heading level={1} variant="page" text="Find a Client" />
				<p class="lede">
					Search for a Client already on file before adding her as new. Name, date of birth,
					email and phone all match — use whatever you have, none of them is required on its own.
				</p>

				{#if pageErrors.length > 0}
					<ErrorSummary errors={pageErrors} />
				{/if}

				<form onsubmit={handleSearch} novalidate>
					<stack-l space="var(--space-5)">
						<LabeledField id={nameId} label="Name" error={errorFor(nameId)}>
							{#snippet children({ id, describedBy, invalid })}
								<TextInput
									{id}
									{describedBy}
									{invalid}
									value={name}
									onInput={(v) => (name = v)}
									autocomplete="off"
								/>
							{/snippet}
						</LabeledField>
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
						<LabeledField id={emailId} label="Email">
							{#snippet children({ id, describedBy })}
								<TextInput
									{id}
									{describedBy}
									type="email"
									value={email}
									onInput={(v) => (email = v)}
									autocomplete="off"
								/>
							{/snippet}
						</LabeledField>
						<LabeledField id={phoneId} label="Phone">
							{#snippet children({ id, describedBy })}
								<TextInput
									{id}
									{describedBy}
									type="tel"
									value={phone}
									onInput={(v) => (phone = v)}
									autocomplete="off"
								/>
							{/snippet}
						</LabeledField>
						<cluster-l space="var(--space-3)" align="center">
							<Button type="submit" label="Search" loading={isSearching} />
						</cluster-l>
					</stack-l>
				</form>

				{#if hasSearched}
					<section aria-labelledby="client-search-results-heading">
						<stack-l space="var(--space-6)">
							<div bind:this={resultsStart} id="client-search-results" tabindex="-1">
								<Heading
									level={2}
									variant="section"
									text={resultsHeading()}
									id="client-search-results-heading"
								/>
							</div>

							{#if matches.length > 0}
								<stack-l space="var(--space-6)">
									{#each matches as match (match.id)}
										<section class="match">
											<stack-l space="var(--space-3)">
												<Heading level={3} variant="card" text={displayName(match)} />
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
															<li>
																{engagement.kind === 'birth' ? 'Birth' : 'Postpartum'} · {engagement.status}
															</li>
														{/each}
													</ul>
												{:else}
													<p class="quiet">No Engagements with this Client yet.</p>
												{/if}
												<Link href={detailHref(match.id)} label="Open {displayName(match)}'s record" />
											</stack-l>
										</section>
									{/each}
								</stack-l>
							{:else}
								<stack-l space="var(--space-4)">
									<p class="lede">
										{#if name.trim()}
											Nothing at this Practice matches what was typed. Add her as a new Client
											instead — the name typed here carries onto intake's first page, so it
											does not have to be retyped.
										{:else}
											Nothing at this Practice matches what was typed. Add her as a new Client
											instead.
										{/if}
									</p>
									<cluster-l space="var(--space-3)" align="center">
										<Link href={startIntakeHref()} label="Add a new Client" />
									</cluster-l>
								</stack-l>
							{/if}
						</stack-l>
					</section>
				{/if}
				</stack-l>
			</center-l>
		</container-l>
{/if}

<style>
	@layer components {
		container-l {
			padding-block: var(--space-8);
		}

		/*
		 * Focused programmatically after every search, never by the
		 * keyboard -- so, per ErrorSummary's own note, `:focus-visible`
		 * would never fire here and the ring has to be `:focus` instead.
		 */
		[tabindex='-1']:focus {
			outline: var(--focus-ring-width) solid var(--color-primary);
			outline-offset: var(--focus-ring-offset);
		}

		.lede {
			margin: 0;
			max-inline-size: 62ch;
			color: var(--color-on-surface-variant);
		}

		.quiet {
			margin: 0;
			font-size: var(--text-body-sm-size);
			color: var(--color-on-surface-muted);
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
