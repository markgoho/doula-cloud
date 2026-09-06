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
		isPushPermissionGranted,
		portalNotificationPreferencePath,
		portalPushSubscriptionsPath,
		registerPushSubscription,
		unregisterPushSubscription
	} from '#lib/pushRegistration.js';
	import FormPage from '#lib/components/templates/FormPage.svelte';
	import Button from '#lib/components/atoms/Button.svelte';
	import Notice from '#lib/components/atoms/Notice.svelte';
	import Text from '#lib/components/atoms/Text.svelte';

	// The durable preference ("wants notifications") and this device's
	// actual ability to receive a push are two separate facts (#715) --
	// collapsed into one type rather than two booleans so "on but this
	// device can't receive" is a state this type names, not an invariant
	// two variables have to be kept in step to represent.
	type NotificationStatus = 'off' | 'on' | 'blocked';

	let isLoaded = $state(false);
	let loadError = $state('');
	let notificationStatus: NotificationStatus = $state('off');
	let isSaving = $state(false);
	let saveError = $state('');
	let savedNotice = $state('');

	const isEnabled = $derived(notificationStatus !== 'off');

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
		if (data.enabled) {
			// A read of the device's current permission, never a subscribe
			// attempt -- catches permission having been revoked since this
			// device last subscribed, without the mere mount ever risking the
			// browser's own permission prompt (see registerPushSubscriptionIfEnabled's
			// doc comment for why that prompt is reserved for an explicit toggle).
			notificationStatus = isPushPermissionGranted() ? 'on' : 'blocked';
		} else {
			notificationStatus = 'off';
		}
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

			const subscriptionsURL = portalPushSubscriptionsPath(page.params.engagementId!);
			if (saved.enabled) {
				const isSubscribed = await registerPushSubscription(subscriptionsURL, apiFetchWithSession);
				notificationStatus = isSubscribed ? 'on' : 'blocked';
				// A failed subscribe attempt must never claim success (#715) --
				// the status snippet below shows the device-level caveat instead.
				savedNotice = isSubscribed ? 'Notifications are on for this device.' : '';
			} else {
				await unregisterPushSubscription(subscriptionsURL, apiFetchWithSession);
				notificationStatus = 'off';
				savedNotice = 'Notifications are off.';
			}
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
	{#if notificationStatus === 'off'}
		<Text text="Notifications are currently off." />
	{:else if notificationStatus === 'on'}
		<Text text="Notifications are currently on for this device." />
	{:else}
		<Notice
			variant="info"
			message="Notifications are turned on, but this device is not receiving them. To fix this, allow notifications for this site in your browser's own settings."
		/>
	{/if}
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
