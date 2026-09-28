<script lang="ts">
	import PortalFeedback from '#lib/components/organisms/PortalFeedback.svelte';

	/*
	 * Static wiring, no real send: `onSend` never reaches `#lib/api.js`
	 * (PortalFeedback owns no network call of its own, #1528), the same
	 * shape `/style-guide/staff-feedback` (#1527) takes for its own
	 * `onSend`.
	 */
</script>

<stack-l space="var(--space-6)">
	<h1>Portal feedback</h1>

	<section>
		<h2>Inside one Engagement</h2>
		<p>The banner, the drawer it opens, and the role/Practice line in the disclosure.</p>
		<PortalFeedback
			email="alex.rivera@example.com"
			practiceName="Finger Lakes Birth Collective"
			onSend={async () => {}}
		/>
	</section>

	<section>
		<h2>With no single Practice to name</h2>
		<p>
			A Portal Account reaches Clients at more than one Practice (ADR-0015); the copy names "your doula's Practice"
			and the disclosure carries no role/Practice line.
		</p>
		<PortalFeedback email="alex.rivera@example.com" onSend={async () => {}} />
	</section>
</stack-l>
