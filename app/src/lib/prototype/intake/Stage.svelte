<script lang="ts">
	// PROTOTYPE (#372) -- throwaway. Decision 1 of 4: the shape, and nothing
	// else. The case and what the form demands are pinned so there is one dial,
	// not three. fixtures.ts still holds the other cases -- decision 4 walks the
	// winning shape through them. Mounted by the real "Add a Client" route
	// behind `?variant=`.
	import Harness from './Harness.svelte';
	import VariantD from './VariantD.svelte';
	import { cases, emptyClient, emptyRequest, type ClientDraft, type RequestDraft } from './fixtures.js';

	interface Properties {
		variant: string;
	}

	let { variant }: Properties = $props();

	const blurb =
		'D — one thing per page, then a hub. C’s fast front door with B’s ability to keep going: three short pages, a save, and a task list she can work down or walk away from. A, B and C are in this branch’s first commit; this shape replaced them.';

	// Pinned: the ordinary birth intake, seen from an employee Doula's seat.
	const pinnedCase = cases[0];

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

<Harness {blurb} {written}>
	{#key variant}
		<VariantD {...shared} />
	{/key}
</Harness>
