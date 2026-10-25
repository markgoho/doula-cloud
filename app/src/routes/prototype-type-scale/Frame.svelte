<script lang="ts">
	/*
	 * PROTOTYPE (#1926) -- one column the screens render in. It is its own
	 * containment context, so every fluid step inside it answers this
	 * column's width rather than the window's, which is what lets a narrow
	 * and a wide column sit side by side at the sizes each would really get.
	 */
	import StaffTopBar from '#lib/components/organisms/StaffTopBar.svelte';
	import ClientsScreen from './ClientsScreen.svelte';
	import OverviewScreen from './OverviewScreen.svelte';
	import QuestionScreen from './QuestionScreen.svelte';
	import RecordScreen from './RecordScreen.svelte';
	import { practices, staffNav, stay, type ScreenKey } from './fixtures.js';

	interface Properties {
		name: string;
		prefix: string;
		kind: 'narrow' | 'wide';
		narrowPx?: number;
		screens: readonly ScreenKey[];
		width?: number;
	}

	let { name, prefix, kind, narrowPx, screens, width = $bindable(0) }: Properties = $props();

	const signOut = async () => ({ ok: true }) as const;
</script>

<figure class={kind} style={kind === 'narrow' ? `inline-size: ${narrowPx}px` : undefined}>
	<figcaption>{name}, {Math.round(width)}px wide</figcaption>
	<div class="frame" bind:clientWidth={width}>
		<div class="column">
			<StaffTopBar
				navItems={staffNav}
				{practices}
				currentPracticeId="p_7f3a"
				firstName="Jordan"
				lastName="Blake"
				email="jordan@highland-midwifery.example.test"
				accountHref={stay('account')}
				{signOut}
			/>
			{#each screens as screen (screen)}
				<div class="screen">
					{#if screen === 'clients'}
						<ClientsScreen />
					{:else if screen === 'question'}
						<QuestionScreen {prefix} />
					{:else if screen === 'record'}
						<RecordScreen />
					{:else}
						<OverviewScreen />
					{/if}
				</div>
			{/each}
		</div>
	</div>
</figure>

<style>
	figure {
		display: grid;
		align-content: start;
		gap: var(--space-2);
		margin: 0;
		min-inline-size: 0;
	}

	.narrow {
		flex: none;
		max-inline-size: 100%;
	}

	.wide {
		flex: 1 1 var(--form-max);
	}

	figcaption {
		font-size: var(--text-meta-size);
		color: var(--color-on-surface-variant);
		font-variant-numeric: tabular-nums;
	}

	.frame {
		container-type: inline-size;
		overflow-x: auto;
		background-color: var(--color-surface);
		outline: var(--border-thin) solid var(--color-outline);
	}

	/* The base size re-resolved inside the frame (#544), as the drag surface does. */
	.frame > :global(*) {
		font-size: var(--text-body-size);
	}

	.screen + .screen {
		border-block-start: var(--border-active) dashed var(--color-outline);
	}
</style>
