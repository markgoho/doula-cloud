<script lang="ts">
	// PROTOTYPE (#372) -- throwaway. Decision 1 of 4: the shape, and nothing
	// else. The case and what the form demands are pinned so there is one dial,
	// not three. fixtures.ts still holds the other cases -- decision 4 walks the
	// winning shape through them. Mounted by the real "Add a Client" route
	// behind `?variant=`.
	import PrototypeSwitcher from '#lib/prototype/PrototypeSwitcher.svelte';
	import Harness from './Harness.svelte';
	import VariantA from './VariantA.svelte';
	import VariantB from './VariantB.svelte';
	import VariantC from './VariantC.svelte';
	import { cases, emptyClient, emptyRequest, type ClientDraft, type RequestDraft } from './fixtures.js';

	interface Properties {
		variant: string;
	}

	let { variant }: Properties = $props();

	const variants = [
		{ key: 'A', name: 'One page, one submit' },
		{ key: 'B', name: 'Two steps, saved between' },
		{ key: 'C', name: 'Minimal, finish on her page' }
	];

	const blurbs: Record<string, string> = {
		A: 'A — One page, one submit. Her record and the ask for work sit on one screen and commit together. Detail is behind two disclosures. Order: names → how to reach her → date of birth.',
		B: 'B — Two steps, with a real save between them. Step 1 commits her record on its own; step 2 is the ask, and you can walk away before it. Date of birth sits WITH the names, as an identity key.',
		C: 'C — Minimal create, finish on her page. Two fields while you are on the phone, then her record tells you what is missing. The Request is a separate visit, never chained.'
	};

	// Pinned: the ordinary birth intake, and a form that demands a first name
	// alone. Both become their own decision once the shape is settled.
	const pinnedCase = cases[0];
	const demands = 'name-only' as const;

	let client = $state<ClientDraft>({ ...emptyClient(), ...pinnedCase.seed });
	let request = $state<RequestDraft>({ ...emptyRequest(), kind: pinnedCase.kind });
	let custom = $state<Record<string, string | boolean>>({});
	let written = $state<
		{ client?: ClientDraft; request?: RequestDraft; reused?: string; note: string } | undefined
	>();
	function finish(result: { reused?: string; note: string; withRequest: boolean }) {
		written = {
			client: result.reused ? undefined : { ...client },
			request: result.withRequest ? { ...request } : undefined,
			reused: result.reused,
			note: result.note
		};
	}

	const shared = $derived({
		client,
		request,
		custom,
		demands,
		onClient: (patch: Partial<ClientDraft>) => {
			client = { ...client, ...patch };
		},
		onRequest: (patch: Partial<RequestDraft>) => {
			request = { ...request, ...patch };
		},
		onCustom: (id: string, value: string | boolean) => {
			custom = { ...custom, [id]: value };
		},
		onDone: finish
	});
</script>

<Harness blurb={blurbs[variant] ?? blurbs.A} {written}>
	{#key variant}
		{#if variant === 'B'}
			<VariantB {...shared} />
		{:else if variant === 'C'}
			<VariantC {...shared} />
		{:else}
			<VariantA {...shared} />
		{/if}
	{/key}
</Harness>

<PrototypeSwitcher {variants} current={variant} />
