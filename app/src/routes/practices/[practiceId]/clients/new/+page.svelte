<script lang="ts">
	// PROTOTYPE (#371) -- throwaway. Without `?variant=`, this is the real
	// "Add a Client" page, untouched. With it, the page hosts four
	// structurally different answers to "how does a staff member land on
	// an existing Client instead of making a second record?". Data is
	// stubbed in #lib/prototype/reuse/fixtures.ts -- no API, no DB, so
	// `bun run dev` alone is enough to click through this.
	import { goto } from '$app/navigation';
	import { page } from '$app/state';
	import { resolve } from '$app/paths';
	import { apiFetchWithSession } from '#lib/api.js';
	import Heading from '#lib/components/atoms/Heading.svelte';
	import TextInput from '#lib/components/atoms/TextInput.svelte';
	import Button from '#lib/components/atoms/Button.svelte';
	import Notice from '#lib/components/atoms/Notice.svelte';
	import LabeledField from '#lib/components/molecules/LabeledField.svelte';
	import PrototypeSwitcher from '#lib/prototype/PrototypeSwitcher.svelte';
	import Harness from '#lib/prototype/reuse/Harness.svelte';
	import VariantA from '#lib/prototype/reuse/VariantA.svelte';
	import VariantB from '#lib/prototype/reuse/VariantB.svelte';
	import VariantC from '#lib/prototype/reuse/VariantC.svelte';
	import VariantD from '#lib/prototype/reuse/VariantD.svelte';
	import { scenarios, type Disclosure, type Scenario } from '#lib/prototype/reuse/fixtures.js';

	const variants = [
		{ key: 'A', name: 'Match at intake' },
		{ key: 'B', name: 'Search first' },
		{ key: 'C', name: 'Start from her page' },
		{ key: 'D', name: 'One box + review sheet' }
	];

	const blurbs: Record<string, string> = {
		A: 'A — Match at intake. Today’s form. The match arrives as a banner while she types; the Credit is confirmed in the submit label.',
		B: 'B — Search first. The route opens as a search. Reuse is the default path; intake is where the search sends you when nobody matches. The Credit gets its own step.',
		C: 'C — Start from her page. Intake is only for a new person. Reuse is an action on the Client’s own record, and the intake form hard-stops on an exact match.',
		D: 'D — One box + review sheet. A find-or-create field at the top of the form; ignoring the suggestions is how you create. One confirmation at the end, which is also where the Credit is named.'
	};

	const variant = $derived(page.url.searchParams.get('variant') ?? '');
	let scenario = $state<Scenario>(scenarios[0]);
	let disclosure = $state<Disclosure>('named');

	let name = $state('');
	let email = $state('');
	let error = $state('');
	let isSubmitting = $state(false);

	async function handleSubmit(event: SubmitEvent) {
		event.preventDefault();
		error = '';
		isSubmitting = true;
		try {
			const response = await apiFetchWithSession(`/api/practices/${page.params.practiceId}/clients`, {
				method: 'POST',
				headers: { 'Content-Type': 'application/json' },
				body: JSON.stringify({ name, email })
			});
			if (!response.ok) {
				error = await response.text();
				return;
			}

			const created: { engagementId: string } = await response.json();
			await goto(
				resolve('/practices/[practiceId]/engagements/[engagementId]', {
					practiceId: page.params.practiceId!,
					engagementId: created.engagementId
				})
			);
		} catch (error_) {
			error = error_ instanceof Error ? error_.message : 'Failed to add Client';
		} finally {
			isSubmitting = false;
		}
	}
</script>

{#if variant}
	<Harness
		{scenario}
		{disclosure}
		blurb={blurbs[variant] ?? 'Unknown variant — try ?variant=A'}
		onScenario={(s) => (scenario = s)}
		onDisclosure={(d) => (disclosure = d)}
	/>

	{#key `${variant}-${scenario.key}`}
		{#if variant === 'A'}
			<VariantA {scenario} {disclosure} />
		{:else if variant === 'B'}
			<VariantB {scenario} {disclosure} />
		{:else if variant === 'C'}
			<VariantC {scenario} {disclosure} />
		{:else if variant === 'D'}
			<VariantD {scenario} {disclosure} />
		{/if}
	{/key}

	<PrototypeSwitcher {variants} current={variant} />
{:else}
	<Heading level={1} text="Add a Client" />

	<form onsubmit={handleSubmit}>
		<LabeledField label="Their name">
			{#snippet children({ id, describedBy, invalid })}
				<TextInput
					{id}
					{describedBy}
					{invalid}
					value={name}
					onInput={(value) => (name = value)}
					required
				/>
			{/snippet}
		</LabeledField>
		<LabeledField label="Their email">
			{#snippet children({ id, describedBy, invalid })}
				<TextInput
					{id}
					{describedBy}
					{invalid}
					type="email"
					value={email}
					onInput={(value) => (email = value)}
					required
				/>
			{/snippet}
		</LabeledField>
		<Button type="submit" label="Add Client" loading={isSubmitting} />
		{#if error}
			<Notice variant="error" message={error} />
		{/if}
	</form>
{/if}
