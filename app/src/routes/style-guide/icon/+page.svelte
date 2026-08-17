<script lang="ts">
	import Icon, { SIZE_THRESHOLD_FOR_DUOTONE } from '#lib/components/atoms/Icon.svelte';
	import { iconManifest } from '#lib/components/atoms/Icon/manifest.js';
</script>

<stack-l space="var(--space-6)">
	<h1>Icon</h1>

	<section>
		<h2>Manifest (default 24px, duotone)</h2>
		<cluster-l space="var(--space-4)">
			{#each iconManifest as name (name)}
				<stack-l space="var(--space-1)">
					<Icon {name} label={name} />
					<span>{name}</span>
				</stack-l>
			{/each}
		</cluster-l>
	</section>

	<section>
		<h2>Weight threshold ({SIZE_THRESHOLD_FOR_DUOTONE}px)</h2>
		<cluster-l space="var(--space-4)" align="center">
			<stack-l space="var(--space-1)">
				<Icon
					name="check"
					size={SIZE_THRESHOLD_FOR_DUOTONE - 1}
					label="check, {SIZE_THRESHOLD_FOR_DUOTONE - 1}px"
				/>
				<span>{SIZE_THRESHOLD_FOR_DUOTONE - 1}px — light</span>
			</stack-l>
			<stack-l space="var(--space-1)">
				<Icon
					name="check"
					size={SIZE_THRESHOLD_FOR_DUOTONE}
					label="check, {SIZE_THRESHOLD_FOR_DUOTONE}px"
				/>
				<span>{SIZE_THRESHOLD_FOR_DUOTONE}px — duotone</span>
			</stack-l>
			<stack-l space="var(--space-1)">
				<Icon name="check" size={48} label="check, 48px" />
				<span>48px — duotone</span>
			</stack-l>
		</cluster-l>
	</section>

	<section>
		<h2>Decorative vs. labeled</h2>
		<cluster-l space="var(--space-4)">
			<Icon name="info" />
			<span>Decorative (no label, aria-hidden)</span>
		</cluster-l>
		<cluster-l space="var(--space-4)">
			<Icon name="info" label="Information" />
			<span>Labeled (role="img", aria-label)</span>
		</cluster-l>
	</section>

	<section>
		<h2>Layer styling via --icon-fg / --icon-bg</h2>
		<cluster-l space="var(--space-4)">
			<div style:--icon-fg="var(--color-error)" style:--icon-bg="var(--color-accent)">
				<Icon name="warning" size={48} label="Warning, custom colors" />
			</div>
			<span>--icon-fg: var(--color-error); --icon-bg: var(--color-accent);</span>
		</cluster-l>
	</section>
</stack-l>
