<script lang="ts">
	import StaffFeedback from '#lib/components/organisms/StaffFeedback.svelte';

	/*
	 * Static wiring, no real send: `onSend` never reaches `#lib/api.js`
	 * (StaffFeedback owns no network call of its own, #1527), so a click
	 * on Send here is harmless -- the same shape the Refund payment form
	 * page takes for its own `onConfirm`.
	 */
</script>

<stack-l space="var(--space-6)">
	<h1>Staff feedback</h1>

	<section>
		<h2>Under a Practice</h2>
		<p>The banner, the drawer it opens, and the role/Practice line in the disclosure.</p>
		<StaffFeedback
			email="jordan@fingerlakesbirth.example"
			practiceName="Finger Lakes Birth Collective"
			roles={['owner']}
			onSend={async () => {}}
		/>
	</section>

	<section>
		<h2>Under /account, with no Practice</h2>
		<p>No role or Practice in the disclosure.</p>
		<StaffFeedback email="jordan@fingerlakesbirth.example" onSend={async () => {}} />
	</section>
</stack-l>
