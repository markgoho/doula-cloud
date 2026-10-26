<script lang="ts">
	import ContractStatus from '#lib/components/molecules/ContractStatus.svelte';
</script>

<stack-l space="var(--space-6)">
	<h1>Contract status</h1>

	<!--
		No hostile value exists for this component (ADR-0025): it takes a
		status and a hasSignedPdf flag, not free text, so its longest
		realistic value is the longest of the five sections below. #1229
		added the second one -- not a fourth status (there are still only
		three: draft, signed, voided) but the other value hasSignedPdf can
		take on a Draft, which is what the notice's presence or absence
		demonstrates. #2104's sent and signed Stamps carry a person's name,
		so the signed and voided sections give the longest a roster holds.
	-->

	<section>
		<h2>Draft</h2>
		<ContractStatus status="draft" hasSignedPdf={false} />
	</section>

	<section>
		<h2>Draft, beside an earlier signed-and-voided Contract (#1229) &mdash; the PDF is named as the earlier Contract's</h2>
		<ContractStatus status="draft" hasSignedPdf onDownloadPdf={async () => {}} />
	</section>

	<section>
		<h2>Signed, seen by an Owner or Admin &mdash; download and Void are both offered</h2>
		<ContractStatus
			status="signed"
			hasSignedPdf
			amountChangedAt="2026-08-19T00:00:00Z"
			sent={{ at: '2026-08-12T15:00:00Z', by: 'Anne-Marie Ochieng-Whitfield' }}
			signed={{ at: '2026-08-14T20:12:00Z', by: 'Oluwaseun Adebayo-Montgomery Fitzwilliam' }}
			rows={[{ label: 'Your fee', value: '$12,620.00, the fee you accepted on Jul 22, 2026' }]}
			onVoid={async () => {}}
			onDownloadPdf={async () => {}}
		/>
	</section>

	<section>
		<h2>Signed, seen by a Doula &mdash; the PDF endpoint refuses the Doula, so no download button</h2>
		<ContractStatus status="signed" hasSignedPdf onVoid={async () => {}} />
	</section>

	<section>
		<h2>Voided, the terminal state &mdash; the agreement is over, the copy of it is not</h2>
		<ContractStatus
			status="voided"
			hasSignedPdf
			sent={{ at: '2026-08-12T15:00:00Z', by: 'Anne-Marie Ochieng-Whitfield' }}
			signed={{ at: '2026-08-14T20:12:00Z', by: 'Oluwaseun Adebayo-Montgomery Fitzwilliam' }}
			onDownloadPdf={async () => {}}
		/>
	</section>
</stack-l>
