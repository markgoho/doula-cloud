<script lang="ts">
	import Checkbox from '#lib/components/atoms/Checkbox.svelte';
	import LabeledField from '#lib/components/molecules/LabeledField.svelte';

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
		<LabeledField label="Unchecked" orientation="inline">{#snippet children(control)}<Checkbox checked={isUnchecked} onChange={(value) => (isUnchecked = value)} {...control} />{/snippet}</LabeledField>
		<LabeledField label="I have read this Contract and I am signing it electronically" orientation="inline">{#snippet children(control)}<Checkbox checked={isChecked} onChange={(value) => (isChecked = value)} {...control} />{/snippet}</LabeledField>
	</section>

	<section>
		<h2>Focus</h2>
		<p>Tab to any control on this page to see its focus outline.</p>
	</section>

	<section>
		<h2>Invalid</h2>
		<LabeledField label="I agree to sign electronically, continue" orientation="inline">{#snippet children(control)}<Checkbox {...control} checked={false} onChange={noop} invalid />{/snippet}</LabeledField>
	</section>

	<section>
		<h2>Disabled</h2>
		<LabeledField label="Disabled, unchecked" orientation="inline">{#snippet children(control)}<Checkbox checked={false} onChange={noop} disabled {...control} />{/snippet}</LabeledField>
		<LabeledField label="Disabled, checked" orientation="inline">{#snippet children(control)}<Checkbox checked={true} onChange={noop} disabled {...control} />{/snippet}</LabeledField>
	</section>

	<section>
		<h2>Toggle variant</h2>
		<LabeledField label="Off" orientation="inline">{#snippet children(control)}<Checkbox variant="toggle" checked={isToggleOff} onChange={(value) => (isToggleOff = value)} {...control} />{/snippet}</LabeledField>
		<LabeledField label="On" orientation="inline">{#snippet children(control)}<Checkbox variant="toggle" checked={isToggleOn} onChange={(value) => (isToggleOn = value)} {...control} />{/snippet}</LabeledField>
	</section>
</stack-l>
