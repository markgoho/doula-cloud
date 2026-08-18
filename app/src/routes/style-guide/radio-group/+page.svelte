<script lang="ts">
	import RadioGroup from '#lib/components/molecules/RadioGroup.svelte';

	const modeOptions: { value: 'signup' | 'login'; label: string }[] = [
		{ value: 'signup', label: "I'm new here -- create an account" },
		{ value: 'login', label: 'I already have an account -- log in' }
	];

	let mode = $state<'signup' | 'login'>('signup');
</script>

<stack-l space="var(--space-6)">
	<h1>Radio group</h1>

	<section>
		<h2>Default</h2>
		<RadioGroup legend="Account mode" options={modeOptions} value={mode} onChange={(value) => (mode = value)} />
	</section>

	<section>
		<h2>Focus</h2>
		<p>Tab to any control on this page to see its focus outline.</p>
	</section>
</stack-l>
