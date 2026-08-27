<script lang="ts">
	// PROTOTYPE (#372) -- throwaway. Holds the draft state the variants share, so
	// switching variant or case resets cleanly. Mounted by the real
	// "Add a Client" route behind `?variant=`.
	import PrototypeSwitcher from '#lib/prototype/PrototypeSwitcher.svelte';
	import Harness from './Harness.svelte';
	import VariantA from './VariantA.svelte';
	import VariantB from './VariantB.svelte';
	import VariantC from './VariantC.svelte';
	import {
		cases,
		emptyClient,
		emptyRequest,
		type Case,
		type ClientDraft,
		type Demands,
		type RequestDraft
	} from './fixtures.js';

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

	let activeCase = $state<Case>(cases[0]);
	let demands = $state<Demands>('name-only');
	let client = $state<ClientDraft>({ ...emptyClient(), ...cases[0].seed });
	let request = $state<RequestDraft>({ ...emptyRequest(), kind: cases[0].kind });
	let custom = $state<Record<string, string | boolean>>({});
	let written = $state<
		{ client?: ClientDraft; request?: RequestDraft; reused?: string; note: string } | undefined
	>();
	let generation = $state(0);

	function reset(next: Case = activeCase) {
		activeCase = next;
		client = { ...emptyClient(), ...next.seed };
		request = { ...emptyRequest(), kind: next.kind };
		custom = {};
		written = undefined;
		generation += 1;
	}

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

<Harness
	blurb={blurbs[variant] ?? blurbs.A}
	{activeCase}
	{demands}
	onCase={(next) => reset(next)}
	onDemands={(next) => {
		demands = next;
		reset();
	}}
	{written}
>
	{#key `${variant}-${generation}`}
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
