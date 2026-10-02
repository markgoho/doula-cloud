<script module lang="ts">
	import type { DemoVariant } from '../drag-surface/dragSurface.js';

	interface Properties {
		isWide?: boolean;
	}

	/*
	 * The other state this page renders (#1638, ADR-0025): the answer
	 * column at the wide cap and not the form-width one. Its own subject
	 * for the continuum check and the drag surface.
	 */
	export const variants: readonly DemoVariant<Properties>[] = [
		{ name: 'Check answers, with the wide column', props: { isWide: true } }
	];
</script>

<script lang="ts">
	import CheckAnswers, { type AnswerSection } from '#lib/components/templates/CheckAnswers.svelte';
	import type { JourneyStep } from '#lib/components/organisms/StepRail.svelte';
	import Button from '#lib/components/atoms/Button.svelte';
	import Link from '#lib/components/atoms/Link.svelte';

	const here = '/style-guide/check-answers';

	/*
	 * The longest realistic value, not a representative one (ADR-0025): every
	 * step, heading and answer names the Client in full, so the rail and the
	 * answer column are both sized by the longest name a Practice has -- and
	 * the email answer is the longest address a Practice hands out.
	 */
	const steps: JourneyStep[] = [
		{
			label: 'Who Anne-Marie Ochieng-Whitfield is',
			href: here,
			status: 'completed',
			questions: [
				{ label: "Anne-Marie Ochieng-Whitfield's full legal name", href: here },
				{ label: 'Date of birth', href: here }
			]
		},
		{
			label: 'How to reach Anne-Marie Ochieng-Whitfield',
			href: `${here}#reach`,
			status: 'completed',
			questions: [
				{ label: 'Email address we send the portal invite to', href: here },
				{ label: 'Phone number', href: here }
			]
		},
		{
			label: 'Where Anne-Marie Ochieng-Whitfield lives',
			href: `${here}#lives`,
			status: 'completed'
		},
		{ label: 'Birth preferences', href: `${here}#prefs`, status: 'completed' },
		{ label: 'Check your answers', href: `${here}#check`, status: 'current' }
	];

	const sections: AnswerSection[] = [
		{
			heading: 'Who Anne-Marie Ochieng-Whitfield is',
			answers: [
				{ label: 'Given name', value: 'Anne-Marie', changeHref: here, changes: 'given name' },
				{
					label: 'Family name',
					value: 'Ochieng-Whitfield',
					changeHref: here,
					changes: 'family name'
				},
				{
					label: 'Preferred name',
					value: 'Anne-Marie',
					changeHref: here,
					changes: 'preferred name'
				},
				{
					label: 'Date of birth',
					value: '4 February 1990',
					changeHref: here,
					changes: 'date of birth'
				}
			]
		},
		{
			heading: 'How to reach Anne-Marie Ochieng-Whitfield',
			answers: [
				{
					label: 'Email address',
					value: 'anne-marie.ochieng-whitfield@highland-midwifery-group.example.org',
					changeHref: here,
					changes: 'email address'
				},
				{ label: 'Phone number', value: '(585) 555 0142', changeHref: here, changes: 'phone number' }
			]
		},
		{
			heading: 'Birth preferences',
			answers: [
				{
					label: 'Planned place of birth',
					value: 'Rochester General Hospital Birthing Center',
					changeHref: here,
					changes: 'planned place of birth'
				}
			]
		}
	];

	let { isWide = false }: Properties = $props();

	const noop = () => {};
</script>

{#snippet actions()}
	<Button label="Save this Client and send the portal invite" type="submit" onClick={noop} />
	<Link href={here} label="Save and come back later" />
{/snippet}

<CheckAnswers
	journey="Adding a Client to Highland Midwifery"
	{steps}
	backHref={here}
	title="Check your answers before adding Anne-Marie Ochieng-Whitfield"
	caption="Adding a Client to Highland Midwifery"
	{sections}
	{isWide}
	{actions}
/>
