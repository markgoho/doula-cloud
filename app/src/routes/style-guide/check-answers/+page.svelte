<script module lang="ts">
	import type { DemoVariant } from '../drag-surface/dragSurface.js';

	interface Properties {
		isWide?: boolean;
		hasDeclaration?: boolean;
	}

	/*
	 * The other state this page renders (#1638, ADR-0025): the answer
	 * column at the wide cap and not the form-width one. Its own subject
	 * for the continuum check and the drag surface.
	 */
	export const variants: readonly DemoVariant<Properties>[] = [
		{ name: 'Check answers, with the wide column', props: { isWide: true } },
		// #2049: GOV.UK's declaration, the words the button agrees to.
		{ name: 'Check answers, with a declaration', props: { hasDeclaration: true } }
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

	/*
	 * The real check page's shape (#2033): the built-in questions are one
	 * group with no heading, the address one row, and a Practice's own
	 * section is a headed group under them.
	 */
	const sections: AnswerSection[] = [
		{
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
					value: 'Feb 4, 1990',
					changeHref: here,
					changes: 'date of birth'
				},
				{
					label: 'Email address',
					value: 'anne-marie.ochieng-whitfield@highland-midwifery-group.example.org',
					changeHref: here,
					changes: 'email address'
				},
				{ label: 'Phone number', value: '(585) 555 0142', changeHref: here, changes: 'phone number' },
				{
					label: 'Address',
					value: '1180 East Henrietta Road, Apartment 4B, Rochester, NY, 14623',
					changeHref: here,
					changes: 'address'
				}
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

	let { isWide = false, hasDeclaration = false }: Properties = $props();

	const noop = () => {};
</script>

{#snippet declaration()}
	<p id="style-guide-check-answers-declaration">
		By adding Anne-Marie Ochieng-Whitfield, you confirm that Highland Midwifery has the Client's
		permission to keep these details.
	</p>
{/snippet}

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
	declaration={hasDeclaration ? declaration : undefined}
	{actions}
/>
