<script lang="ts">
	/*
	 * #303: what a push notification is (and is not), and a durable way to
	 * turn it on or off without signing out.
	 *
	 * The explanation sits above the action deliberately, and nothing on
	 * this route calls registerPushSubscription except handleToggle below --
	 * the browser's own permission prompt only ever fires from that one
	 * click, after she has read what it is for. #61's content-free rule
	 * (ADR-0002/ADR-0009) means the copy can describe the mechanism but
	 * never show her a worked example naming her Practice.
	 *
	 * A Button rather than a Checkbox+Save: turning push on is not a value
	 * that "saves" quietly, it is an act with an immediate side effect (the
	 * browser's own permission dialog) -- the same shape the account page's
	 * "Send a new verification link" secondary button already uses for an
	 * async action rather than a form field.
	 */
	import { onMount } from 'svelte';
	import { page } from '$app/state';
	import { apiFetchWithSession } from '#lib/api.js';
	import { refusalMessage, SERVICE_PROBLEM } from '#lib/formErrors.js';
	import {
		portalNotificationPreferencePath,
		portalPushSubscriptionsPath,
		registerPushSubscription,
		unregisterPushSubscription
	} from '#lib/pushRegistration.js';
	import FormPage from '#lib/components/templates/FormPage.svelte';
	import Button from '#lib/components/atoms/Button.svelte';
	import Notice from '#lib/components/atoms/Notice.svelte';
	import Text from '#lib/components/atoms/Text.svelte';

	let isLoaded = $state(false);
	let loadError = $state('');
	let isEnabled = $state(false);
	let isSaving = $state(false);
	let saveError = $state('');
	let savedNotice = $state('');

	function preferenceURL(): string {
		return portalNotificationPreferencePath(page.params.engagementId!);
	}

	async function loadPreference() {
		const response = await apiFetchWithSession(preferenceURL());
		if (!response.ok) {
			loadError = await refusalMessage(response);
			return;
		}
		const data: { enabled: boolean } = await response.json();
		isEnabled = data.enabled;
		isLoaded = true;
	}

	onMount(loadPreference);

	/*
	 * The durable preference is written before the device is
	 * registered/unregistered: it is the lasting truth #303's send-path
	 * filter reads, and the Message push path already refuses to send on a
	 * mute regardless of whether any subscription row exists. Writing it
	 * first means a failed subscribe/unsubscribe call never leaves the
	 * stored choice out of step with what she asked for.
	 */
	async function handleToggle(shouldEnable: boolean) {
		saveError = '';
		savedNotice = '';
		isSaving = true;
		try {
			const response = await apiFetchWithSession(preferenceURL(), {
				method: 'PUT',
				headers: { 'Content-Type': 'application/json' },
				body: JSON.stringify({ enabled: shouldEnable })
			});
			if (!response.ok) {
				saveError = await refusalMessage(response);
				return;
			}
			const saved: { enabled: boolean } = await response.json();
			isEnabled = saved.enabled;

			const subscriptionsURL = portalPushSubscriptionsPath(page.params.engagementId!);
			if (isEnabled) {
				await registerPushSubscription(subscriptionsURL, apiFetchWithSession);
			} else {
				await unregisterPushSubscription(subscriptionsURL, apiFetchWithSession);
			}

			savedNotice = isEnabled
				? 'Notifications are on for this device.'
				: 'Notifications are off.';
		} catch {
			saveError = SERVICE_PROBLEM;
		} finally {
			isSaving = false;
		}
	}
</script>

{#snippet intro()}
	<Text
		text="When you get a message, this device can show a notification to let you know one is waiting. A notification never shows who sent it or what the message says &mdash; you open the app to read it. It is not a phone call, and you should not rely on it if something is urgent."
		tone="variant"
	/>
{/snippet}

{#snippet status()}
	<Text
		text={isEnabled ? 'Notifications are currently on for this device.' : 'Notifications are currently off.'}
	/>
{/snippet}

{#snippet actions()}
	<Button
		type="button"
		label={isEnabled ? 'Turn off notifications' : 'Turn on notifications'}
		loading={isSaving}
		onClick={() => handleToggle(!isEnabled)}
	/>
	{#if savedNotice}
		<Notice variant="status" message={savedNotice} />
	{/if}
	{#if saveError}
		<Notice variant="error" message={saveError} />
	{/if}
{/snippet}

<FormPage
	title="Notifications"
	serviceName={page.data.practiceName}
	{intro}
	fieldsets={isLoaded ? [{ content: status }] : []}
	{actions}
	loading={isLoaded || loadError ? undefined : 'Loading your notification settings'}
	{loadError}
/>
