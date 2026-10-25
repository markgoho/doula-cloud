<script module lang="ts">
	export interface Message {
		messageId: string;
		senderType: 'staff' | 'client';
		senderId: string;
		senderName: string;
		body: string;
		attachmentFilename?: string;
		attachmentContentType?: string;
		createdAt: string;
	}
</script>

<script lang="ts">
	import { formatInstantWithClock } from '../../dates.js';
	import type { FormError } from '../../formErrors.js';
	import { FormSubmission, orThrownErrors } from '../../formSubmission.svelte.js';
	import {
		ATTACHMENT_ACCEPT,
		ATTACHMENT_HINT,
		attachmentError,
		MESSAGE_OR_FILE
	} from '../../messageAttachment.js';
	import Button from '../atoms/Button.svelte';
	import Textarea from '../atoms/Textarea.svelte';
	import ErrorSummary from '../molecules/ErrorSummary.svelte';
	import LabeledField from '../molecules/LabeledField.svelte';
	import StackedForm from '../molecules/StackedForm.svelte';
	import Notice from '../atoms/Notice.svelte';

	interface Properties {
		messages: Message[];
		/**
		 * A failure to load or download, shown above the thread. A refused
		 * send is the compose form's own.
		 */
		error: string;
		hasMore: boolean;
		isLoadingOlder: boolean;
		onLoadOlder: () => void;
		/**
		 * Sends one Message. It throws on a refusal -- a `RefusalError`
		 * carrying the BFF's `details` puts each refusal against the field
		 * it names (`body`, `attachment`).
		 */
		onSend: (body: string, attachment: File | undefined) => Promise<void>;
		onDownloadAttachment: (messageId: string, filename: string) => void;
		attachmentPreviewURLs: Record<string, string>;
	}

	let {
		messages,
		error,
		hasMore,
		isLoadingOlder,
		onLoadOlder,
		onSend,
		onDownloadAttachment,
		attachmentPreviewURLs
	}: Properties = $props();

	// The style guide mounts this organism several times on one page, so
	// the ids the error summary links to are scoped to this instance.
	const uid = $props.id();
	const fieldIds = { body: `${uid}-body`, attachment: `${uid}-attachment` };

	let body = $state('');
	let attachment = $state<File | undefined>();
	// Counts sent Messages. The file input is keyed on it, so a send gives
	// a fresh, empty input: a file input's chosen file cannot be set back
	// from a value the way a text field's can.
	let sentCount = $state(0);
	const submission = new FormSubmission();

	/*
	 * What the page can refuse before sending (#1952), in the BFF's own
	 * words. The server still sniffs the file's bytes; this saves a
	 * person a 10MB upload the server would refuse anyway.
	 */
	function composeErrors(): FormError[] {
		if (attachment) {
			const refusal = attachmentError(attachment);
			return refusal ? [{ message: refusal, targetId: fieldIds.attachment }] : [];
		}
		return body.trim() === '' ? [{ message: MESSAGE_OR_FILE, targetId: fieldIds.body }] : [];
	}

	// The draft is cleared only on success; a refused send leaves it in
	// place for the person to correct and send again.
	async function handleSubmit(event: SubmitEvent) {
		event.preventDefault();
		await submission.run(async () => {
			const errors = composeErrors();
			if (errors.length > 0) return errors;

			await onSend(body, attachment);
			body = '';
			attachment = undefined;
			sentCount += 1;
		}, orThrownErrors(fieldIds));
	}
</script>

<stack-l>
	{#if error}
		<Notice variant="error" message={error} />
	{/if}

	{#if hasMore}
		<Button label="Load older messages" variant="secondary" loading={isLoadingOlder} onClick={onLoadOlder} />
	{/if}

	{#if messages.length === 0}
		<p>No messages yet.</p>
	{:else}
		<ul>
			{#each messages as message (message.messageId)}
				<li>
					<stack-l space="var(--space-1)">
						<cluster-l space="var(--space-2)" align="baseline">
							<strong>{message.senderName}</strong>
							<!-- v8 ignore start: Svelte's compiled null-guard on these text nodes is unreachable -- senderType is a required union member and formatInstantWithClock always returns a string -->
							<span class="meta">({message.senderType}) — {formatInstantWithClock(message.createdAt)}</span>
							<!-- v8 ignore stop -->
						</cluster-l>
						{#if message.body}
							<p>{message.body}</p>
						{/if}
						{#if message.attachmentFilename}
							{@const filename = message.attachmentFilename}
							{#if attachmentPreviewURLs[message.messageId]}
								<!-- width/height are the reserved layout box, not the file's
								     own dimensions, which are unknowable before the blob
								     loads. `object-fit: contain` fits the real image inside
								     that box, so the thread never reflows when an attachment
								     arrives and nothing is cropped out of a preview. -->
								<img
									class="attachment-preview"
									src={attachmentPreviewURLs[message.messageId]}
									alt={filename}
									width="240"
									height="180"
								/>
							{/if}
							<Button
								label={filename}
								icon="paperclip"
								variant="secondary"
								size="sm"
								onClick={() => onDownloadAttachment(message.messageId, filename)}
							/>
						{/if}
					</stack-l>
				</li>
			{/each}
		</ul>
	{/if}

	<!--
		#1108: this compose box used to write its own `stack-l` at
		`var(--space-2)`, several steps tighter than every other form in
		the product and with nothing on record asking for it. It is a run
		of three controls, not a row inside a table, so it takes the
		ordinary rhythm.
	-->
	<!--
		The compose form's own refusals, in the same summary-and-field
		shape as every other form (#467): the summary above the form, each
		message also against its control. A refusal the BFF keys to
		`body` or `attachment` lands on that control too (#1952).
	-->
	<ErrorSummary errors={submission.errors} />
	<StackedForm onSubmit={handleSubmit}>
		<LabeledField id={fieldIds.body} label="Message" error={submission.errorFor(fieldIds.body)}>
			{#snippet children({ id, describedBy, invalid })}
				<Textarea
					{id}
					{describedBy}
					{invalid}
					value={body}
					onInput={(next) => (body = next)}
				/>
			{/snippet}
		</LabeledField>
		<!--
			A raw input inside LabeledField rather than a file-input atom
			(#1952): this is the only file input in the app, so an atom would
			be a tier with one user. LabeledField gives it GOV.UK's label,
			hint and error, joined by aria-describedby, like every other
			field.
		-->
		<LabeledField
			id={fieldIds.attachment}
			label="Attachment"
			hint={ATTACHMENT_HINT}
			error={submission.errorFor(fieldIds.attachment)}
		>
			{#snippet children({ id, describedBy, invalid })}
				{#key sentCount}
					<!-- eslint-disable-next-line svelte/no-restricted-html-elements -- no atom exists for a file input (#492); one user, so none is built (#1952) -->
					<input
						{id}
						type="file"
						accept={ATTACHMENT_ACCEPT}
						aria-describedby={describedBy}
						aria-invalid={invalid || undefined}
						onchange={(event) => (attachment = event.currentTarget.files?.[0])}
					/>
				{/key}
			{/snippet}
		</LabeledField>
		<Button type="submit" label="Send" loading={submission.isSubmitting} />
	</StackedForm>
</stack-l>

<style>
	@layer components {
		ul {
			list-style: none;
			margin: 0;
			padding: 0;
		}

		li {
			padding-block: var(--space-3);
			border-block-end: var(--border-thin) solid var(--color-outline-variant);
		}

		.meta {
			color: var(--color-on-surface-variant);
			font-size: var(--text-body-sm-size);
		}

		.attachment-preview {
			display: block;
			inline-size: var(--thumb-max-inline);
			block-size: var(--thumb-max-block);
			max-inline-size: 100%;
			object-fit: contain;
			background-color: var(--color-surface-container);
			border-radius: var(--radius);
		}

		/* A native file control sizes itself from its own button and its
		   "No file chosen" text, and at the 16px `body` floor (ADR-0060)
		   that is wider than a 320px screen's 288px column (#1943). Capping
		   it at its column lets the text truncate inside the control, which
		   is the browser's own behavior for a narrow file input. */
		input[type='file'] {
			display: block;
			max-inline-size: 100%;
		}
	}
</style>
