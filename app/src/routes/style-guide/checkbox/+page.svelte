<script lang="ts">
	import Checkbox from '#lib/components/atoms/Checkbox.svelte';

	const noop = () => {};

	let isUnchecked = $state(false);
	let isChecked = $state(true);
	let isToggleOff = $state(false);
	let isToggleOn = $state(true);
</script>

<stack-l space="var(--space-6)">
	<h1>Checkbox</h1>

	<section>
		<h2>Default</h2>
		<label><Checkbox checked={isUnchecked} onChange={(value) => (isUnchecked = value)} /> Unchecked</label>
		<label><Checkbox checked={isChecked} onChange={(value) => (isChecked = value)} /> I have read this Contract and I am signing it electronically</label>
	</section>

	<section>
		<h2>Focus</h2>
		<p>Tab to any control on this page to see its focus outline.</p>
	</section>

	<section>
		<h2>Invalid</h2>
		<label><Checkbox checked={false} onChange={noop} invalid /> I agree to sign electronically, continue</label>
	</section>

	<section>
		<h2>Disabled</h2>
		<label><Checkbox checked={false} onChange={noop} disabled /> Disabled, unchecked</label>
		<label><Checkbox checked={true} onChange={noop} disabled /> Disabled, checked</label>
	</section>

	<section>
		<h2>Toggle variant</h2>
		<label><Checkbox variant="toggle" checked={isToggleOff} onChange={(value) => (isToggleOff = value)} /> Off</label>
		<label><Checkbox variant="toggle" checked={isToggleOn} onChange={(value) => (isToggleOn = value)} /> On</label>
	</section>
</stack-l>
