<script lang="ts">
	import RadioGroup from '#lib/components/molecules/RadioGroup.svelte';

	const modeOptions: { value: 'signup' | 'login'; label: string }[] = [
		{ value: 'signup', label: "I'm new here -- create an account with this email address" },
		{ value: 'login', label: 'I already have an account -- log in and add this Practice to it' }
	];

	/* The longest label the Start work form draws (#1596, #1518): a
	   double-barreled name and "(you)", which wraps beside its circle at
	   320px rather than under it. */
	const doulaOptions: { value: string; label: string }[] = [
		{ value: 'you', label: 'Maria-Guadalupe Hernandez-Castellanos (you)' },
		{ value: 'staff-2', label: 'Anneliese Vandenberghe-Okonkwo' },
		{ value: 'none', label: 'No Doula yet' }
	];

	const clientOptions: { value: 'existing' | 'new'; label: string; description?: string }[] = [
		{ value: 'existing', label: 'Sarah Whitfield', description: 'Added 4 March 2026. Due 12 June 2026.' },
		{ value: 'new', label: 'No, this is a different person' }
	];

	let mode = $state<'signup' | 'login'>('signup');
	let unchosenMode = $state<'signup' | 'login' | ''>('');
	let doula = $state('you');
	let client = $state<'existing' | 'new'>('existing');
</script>

<stack-l space="var(--space-6)">
	<h1>Radio group</h1>

	<section>
		<h2>Default</h2>
		<RadioGroup legend="Are you creating an account, or adding this Practice to one you already have?" options={modeOptions} value={mode} onChange={(value) => (mode = value)} />
	</section>

	<section>
		<h2>Nothing chosen</h2>
		<RadioGroup legend="Are you creating an account?" options={modeOptions} value={unchosenMode} onChange={(value) => (unchosenMode = value)} />
	</section>

	<section>
		<h2>Long labels</h2>
		<RadioGroup legend="Who is the Doula?" options={doulaOptions} value={doula} onChange={(value) => (doula = value)} />
	</section>

	<section>
		<h2>With a hint per option</h2>
		<RadioGroup legend="Is this the same person?" options={clientOptions} value={client} onChange={(value) => (client = value)} />
	</section>

	<section>
		<h2>Error</h2>
		<RadioGroup legend="Are you creating an account?" options={modeOptions} value="" onChange={() => {}} error="Select whether you are creating an account or logging in" />
	</section>

	<section>
		<h2>Disabled</h2>
		<p>No caller disables a radio. A disabled fieldset around a form still reaches one, and this is how it looks.</p>
		<fieldset disabled class="bare">
			<RadioGroup legend="Are you creating an account?" options={modeOptions} value="signup" onChange={() => {}} />
		</fieldset>
	</section>

	<section>
		<h2>Hover and focus</h2>
		<p>Point at an option to see its halo. Tab to the group to see its focus outline.</p>
	</section>
</stack-l>

<style>
	@layer components {
		.bare {
			margin: 0;
			padding: 0;
			border: none;
		}
	}
</style>
