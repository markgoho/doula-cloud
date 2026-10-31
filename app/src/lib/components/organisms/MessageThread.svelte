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
	import Heading from '../atoms/Heading.svelte';
	import Text from '../atoms/Text.svelte';
	import Textarea from '../atoms/Textarea.svelte';
	import EmptyState from '../molecules/EmptyState.svelte';
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
		/**
		 * Who is reading the thread (#2023). A message from that side reads
		 * "You" in place of the sender's name; every other message reads its
		 * sender's name alone, never a role. Left out where the reader is
		 * one of several people on the same side, such as a Staff member
		 * among a Practice's Staff, so every message keeps its name.
		 */
		readerSenderType?: Message['senderType'];
		/**
		 * The label of one secondary button that opens the compose form
		 * (#2004), for a page where the thread is one section among many:
		 * the thread is read first, the form appears only when asked, and a
		 * send closes it with a status that takes focus. Left out, the form
		 * is always open, as the thread's own page has it.
		 */
		composeOpener?: string;
		/**
		 * Why no message can be sent here, said in place of the compose
		 * form (#2004): on the Staff side, a Client with no portal account
		 * cannot read one. Set, no form is offered at all.
		 */
		cannotSendReason?: string;
		/**
		 * What the status says once a Message is away (#2210): that it
		 * was sent, and what happens next for this reader. The portal's
		 * thread names the Practice and says a message is not for
		 * anything urgent (ADR-0002); left out, it says "Message sent.".
		 * Said only after the server has stored the Message, so it is
		 * never shown for a send that did not reach it (ADR-0059: the
		 * portal queues nothing).
		 */
		sentConfirmation?: string;
	}

	let {
		messages,
		error,
		hasMore,
		isLoadingOlder,
		onLoadOlder,
		onSend,
		onDownloadAttachment,
		attachmentPreviewURLs,
		readerSenderType,
		composeOpener,
		cannotSendReason,
		sentConfirmation = 'Message sent.'
	}: Properties = $props();

	let isComposeOpen = $state(false);
	let sentMessage = $state('');
	const isComposeShown = $derived(!cannotSendReason && (composeOpener === undefined || isComposeOpen));

	// #2103: Cancel hands focus back to the opener, drawn again once the
	// form has gone. Only Cancel sets it: a send's status takes focus.
	let isOpenerRefocused = $state(false);

	function openCompose() {
		sentMessage = '';
		isOpenerRefocused = false;
		isComposeOpen = true;
	}

	function cancelCompose() {
		submission.errors = [];
		isOpenerRefocused = true;
		isComposeOpen = false;
	}

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
		// An earlier send's status goes as this one starts, so a failure
		// never sits under a "sent", and a second send is announced again.
		sentMessage = '';
		await submission.run(async () => {
			const errors = composeErrors();
			if (errors.length > 0) return errors;

			await onSend(body, attachment);
			body = '';
			attachment = undefined;
			sentCount += 1;
			if (composeOpener !== undefined) isComposeOpen = false;
			sentMessage = sentConfirmation;
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
		<EmptyState message="No messages yet." />
	{:else}
		<ul>
			{#each messages as message (message.messageId)}
				<li>
					<stack-l space="var(--space-1)">
						<cluster-l space="var(--space-2)" align="baseline">
							<strong>{message.senderType === readerSenderType ? 'You' : message.senderName}</strong>
							<!-- v8 ignore start: Svelte's compiled null-guard on this text node is unreachable -- formatInstantWithClock always returns a string -->
							<span class="meta">{formatInstantWithClock(message.createdAt)}</span>
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
	{#if cannotSendReason}
		<Text text={cannotSendReason} />
	{/if}

	<!--
		Focused only where the form has closed, so Send is gone (GOV.UK's
		success-banner rule, #1598); with the form still open, Send keeps
		focus and the status is announced (#2210).
	-->
	{#if sentMessage}
		<Notice variant="status" message={sentMessage} isFocusedOnAppear={composeOpener !== undefined} />
	{/if}

	{#if composeOpener !== undefined && !isComposeOpen && !cannotSendReason}
		<div>
			<Button label={composeOpener} variant="secondary" onClick={openCompose} focusOnAppearing={isOpenerRefocused} />
		</div>
	{/if}

	{#if isComposeShown}
		<ErrorSummary errors={submission.errors} />
		<StackedForm onSubmit={handleSubmit}>
			{#if composeOpener !== undefined}
				<Heading level={3} text={composeOpener} focusOnAppearing />
			{/if}
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
			{#if composeOpener === undefined}
				<Button type="submit" label="Send" loading={submission.isSubmitting} />
			{:else}
				<cluster-l space="var(--space-3)">
					<Button type="submit" label="Send" loading={submission.isSubmitting} />
					<Button label="Cancel" variant="secondary" onClick={cancelCompose} />
				</cluster-l>
			{/if}
		</StackedForm>
	{/if}
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
		   is the browser's own behavior for a narrow file input. Up to a
		   reading measure, as the input atoms are (#2158): in a record
		   page's section column it was stretched to 1,044px at 1440. */
		input[type='file'] {
			display: block;
			max-inline-size: min(100%, var(--measure));
			/* "No file chosen" and the chosen file's name take the input's
			   own color, which is the browser's until it is set here. */
			color: var(--color-on-surface);
			font-family: var(--font-family-base);
			font-size: var(--text-body-size);
		}

		/* The browser's gray "Choose File" button, drawn as the secondary
		   Button (#2017): the same tokens `Button.svelte`'s `secondary`
		   and `size-md` rules spend, with CSS alone, so the native control
		   and its keyboard and screen-reader behavior stay as they are. */
		input[type='file']::file-selector-button {
			min-block-size: var(--hit-target-min);
			margin-inline-end: var(--space-3);
			padding: var(--space-2) var(--space-4);
			font-family: var(--font-family-base);
			font-size: var(--text-body-size);
			font-weight: var(--font-weight-medium);
			color: var(--color-on-surface);
			background-color: transparent;
			border: var(--border-thin) solid var(--color-outline);
			border-radius: var(--radius);
			cursor: pointer;
		}

		input[type='file']:hover::file-selector-button {
			background-color: var(--color-outline-variant);
		}

		input[type='file']:focus-visible {
			outline: var(--focus-ring-width) solid var(--color-primary);
			outline-offset: var(--focus-ring-offset);
		}
	}
</style>
