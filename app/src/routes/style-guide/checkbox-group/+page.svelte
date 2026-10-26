<script lang="ts">
	import CheckboxGroup from '#lib/components/molecules/CheckboxGroup.svelte';
	import RadioGroup from '#lib/components/molecules/RadioGroup.svelte';
	import { ROLE_LABELS, ROLE_DESCRIPTIONS } from '#lib/roles.js';

	/* The longest realistic option text (ADR-0025): a Practice writes its
	   own options, and a Client's answer can be a sentence. */
	const roomOptions = [
		{ value: 'partner', label: 'My partner, for the whole labor and the first hours after the birth' },
		{ value: 'doula', label: 'My Doula from Highland Midwifery' },
		{ value: 'mother', label: 'My mother' },
		{ value: 'photographer', label: 'A birth photographer' }
	];

	/*
	roles.ts is the one place these are put into words.
	*/
	const roleOptions = ROLE_LABELS.map((option) => ({ ...option, description: ROLE_DESCRIPTIONS[option.value] }));

	const yesNo = [
		{ value: 'yes', label: 'Yes' },
		{ value: 'no', label: 'No' }
	];

	let room = $state<string[]>(['partner']);
	let nothing = $state<string[]>([]);
	let roles = $state<string[]>(['doula']);
	let answer = $state<string | undefined>('yes');
</script>

<stack-l space="var(--space-6)">
	<h1>Checkbox group</h1>

	<section>
		<h2>Default, with long labels</h2>
		<CheckboxGroup legend="Who do you want in the room with you?" options={roomOptions} value={room} onChange={(next) => (room = next)} />
	</section>

	<section>
		<h2>With a hint under the question</h2>
		<CheckboxGroup legend="Who do you want in the room with you?" hint="Select all that apply" options={roomOptions} value={nothing} onChange={(next) => (nothing = next)} />
	</section>

	<section>
		<h2>With a hint per option</h2>
		<CheckboxGroup legend="Roles" options={roleOptions} value={roles} onChange={(next) => (roles = next)} />
	</section>

	<section>
		<h2>Error</h2>
		<CheckboxGroup legend="Roles" options={roleOptions} value={[]} onChange={() => {}} error="Select at least one role" />
	</section>

	<section>
		<h2>Disabled</h2>
		<p>No caller disables a box. A disabled fieldset around a form still reaches one, and this is how it looks.</p>
		<fieldset disabled class="plain-fieldset">
			<CheckboxGroup legend="Roles" options={roleOptions} value={['admin']} onChange={() => {}} />
		</fieldset>
	</section>

	<section>
		<h2>Beside a radio group</h2>
		<p>The box and the circle are one size, and the gap to the text is one value, so the two kinds of question read as one product.</p>
		<stack-l space="var(--space-7)">
			<CheckboxGroup legend="Roles" options={roleOptions} value={roles} onChange={(next) => (roles = next)} />
			<RadioGroup legend="Is this the same person?" options={yesNo} value={answer} onChange={(next) => (answer = next)} />
		</stack-l>
	</section>

	<section>
		<h2>Hover and focus</h2>
		<p>Point at an option to see its halo. Tab to the group to see its focus outline.</p>
	</section>
</stack-l>

<style>
	@layer components {
		.plain-fieldset {
			margin: 0;
			padding: 0;
			border: none;
		}
	}
</style>
