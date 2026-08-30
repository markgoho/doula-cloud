<script lang="ts">
	import CheckAnswers, { type AnswerSection } from '#lib/components/templates/CheckAnswers.svelte';
	import type { JourneyStep } from '#lib/components/organisms/StepRail.svelte';
	import Button from '#lib/components/atoms/Button.svelte';
	import Link from '#lib/components/atoms/Link.svelte';

	const here = '/style-guide/check-answers';

	const steps: JourneyStep[] = [
		{
			label: 'Who Sarah is',
			href: here,
			status: 'completed',
			questions: [
				{ label: "Sarah's name", href: here },
				{ label: 'Date of birth', href: here }
			]
		},
		{
			label: 'How to reach Sarah',
			href: `${here}#reach`,
			status: 'completed',
			questions: [
				{ label: 'Email address', href: here },
				{ label: 'Phone number', href: here }
			]
		},
		{ label: 'Where Sarah lives', href: `${here}#lives`, status: 'completed' },
		{ label: 'Birth preferences', href: `${here}#prefs`, status: 'completed' },
		{ label: 'Check your answers', href: `${here}#check`, status: 'current' }
	];

	const sections: AnswerSection[] = [
		{
			heading: 'Who Sarah is',
			answers: [
				{ label: 'Given name', value: 'Sarah', changeHref: here, changes: 'given name' },
				{ label: 'Family name', value: 'Whitfield', changeHref: here, changes: 'family name' },
				{ label: 'Preferred name', value: 'Sarah', changeHref: here, changes: 'preferred name' },
				{ label: 'Date of birth', value: '4 February 1990', changeHref: here, changes: 'date of birth' }
			]
		},
		{
			heading: 'How to reach Sarah',
			answers: [
				{ label: 'Email address', value: 'sarah@example.com', changeHref: here, changes: 'email address' },
				{ label: 'Phone number', value: '(585) 555 0142', changeHref: here, changes: 'phone number' }
			]
		},
		{
			heading: 'Birth preferences',
			answers: [
				{ label: 'Planned place of birth', value: 'Strong Memorial', changeHref: here, changes: 'planned place of birth' }
			]
		}
	];

	let isWide = $state(false);

	const noop = () => {};
</script>

{#snippet actions()}
	<Button label="Save this client" type="submit" onClick={noop} />
	<Link href={here} label="Save and come back later" />
{/snippet}

<div class="controls">
	<Button
		label={isWide ? 'Show the form-width column' : 'Show the wide column'}
		variant="secondary"
		size="sm"
		onClick={() => (isWide = !isWide)}
	/>
</div>

<CheckAnswers
	journey="Adding a client"
	{steps}
	allStepsHref={here}
	backHref={here}
	title="Check your answers before adding Sarah"
	caption="Adding a client"
	{sections}
	{isWide}
	{actions}
/>

<style>
	@layer components {
		/* Not part of the Template -- a switch so both column widths can be
		   seen without editing this file. */
		.controls {
			padding: var(--space-3) var(--space-4);
			border-block-end: var(--border-thin) solid var(--color-outline-variant);
			background-color: var(--color-surface-container);
		}
	}
</style>
