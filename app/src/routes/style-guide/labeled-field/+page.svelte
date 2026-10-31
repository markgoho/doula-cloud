<script lang="ts">
	import LabeledField from '#lib/components/molecules/LabeledField.svelte';
	import TextInput from '#lib/components/atoms/TextInput.svelte';
	import Checkbox from '#lib/components/atoms/Checkbox.svelte';
	import { SIGN_CONSENT_LABEL } from '#lib/clientRegister.js';

	let name = $state('');
	let email = $state('');
	let code = $state('123456');
	let isAttestation = $state(false);
</script>

<stack-l space="var(--space-6)">
	<h1>Labeled field</h1>

	<section>
		<h2>Default</h2>
		<!--
			The longest realistic value, not a representative one (ADR-0025): the
			label is the longest question this field asks, and the error is
			GOV.UK's own format-error wording, which is the longest message the
			component ever carries.
		-->
		<LabeledField label="Full legal name, as it appears on the Contract">
			{#snippet children({ id, describedBy, invalid })}
				<TextInput {id} {describedBy} {invalid} value={name} onInput={(value) => (name = value)} />
			{/snippet}
		</LabeledField>
	</section>

	<section>
		<h2>Focus</h2>
		<p>Tab to any control on this page to see its focus outline.</p>
	</section>

	<section>
		<h2>Error</h2>
		<LabeledField label="Email address we send the portal invite to" error="Enter an email address in the correct format, like name@example.com">
			{#snippet children({ id, describedBy, invalid })}
				<TextInput
					{id}
					{describedBy}
					{invalid}
					type="email"
					value={email}
					onInput={(value) => (email = value)}
				/>
			{/snippet}
		</LabeledField>
	</section>

	<!--
		The width is stated on the field and passed through to the control
		(#1216); the label, the hint and the error keep the column.
	-->
	<section>
		<h2>With a width</h2>
		<LabeledField
			label="Authenticator app code"
			hint="The six-digit code your authenticator app shows now."
			error="Enter the six-digit code from your authenticator app"
			width={10}
		>
			{#snippet children({ id, describedBy, invalid, width })}
				<TextInput {id} {describedBy} {invalid} {width} inputmode="numeric" value={code} onInput={(value) => (code = value)} />
			{/snippet}
		</LabeledField>
	</section>

	<section>
		<h2>Inline orientation</h2>
		<stack-l space="var(--space-4)">
			<LabeledField label="Full legal name, as it appears on the Contract" orientation="inline">
				{#snippet children({ id, describedBy, invalid })}
					<TextInput {id} {describedBy} {invalid} value={name} onInput={(value) => (name = value)} />
				{/snippet}
			</LabeledField>
			<!--
				A checkbox, not only a text input (#510): a checkbox's own fixed
				width is what exposed the orphaned-label defect, and this is the
				actual consent wording from SignContract -- the longest label an
				inline field carries anywhere in the app (ADR-0025).
			-->
			<LabeledField
				label={SIGN_CONSENT_LABEL}
				orientation="inline"
			>
				{#snippet children({ id, describedBy, invalid })}
					<Checkbox {id} {describedBy} {invalid} checked={isAttestation} onChange={(checked) => (isAttestation = checked)} />
				{/snippet}
			</LabeledField>
		</stack-l>
	</section>
</stack-l>
