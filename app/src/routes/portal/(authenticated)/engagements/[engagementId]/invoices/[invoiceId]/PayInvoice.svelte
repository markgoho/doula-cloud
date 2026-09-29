<script lang="ts">
	/*
	 * The pay affordance on an open Stripe-rail Invoice (#983, #1020): the
	 * Stripe Payment Element in our own chrome, and one primary action that
	 * carries the figure. Stripe.js is loaded from js.stripe.com by
	 * `loadStripe`, never bundled, and constructed with the Practice's
	 * `stripeAccount`, because the Invoice is paid as a direct charge on
	 * the connected account (#980).
	 *
	 * This is the SAQ A boundary: the card fields are Stripe's iframe, so
	 * nothing typed into them reaches this code. The only thing we hold is
	 * the client secret, which the server hands out per Invoice.
	 *
	 * What a Client reads after `confirmPayment` succeeds claims only what
	 * we hold: her payment was sent. The Invoice stays "Not yet paid" until
	 * the `invoice.paid` webhook lands, and the sentence says so rather than
	 * flipping the label early. #333 records the wording of that moment as
	 * still open.
	 */
	import { onMount } from 'svelte';
	import { loadStripe, type Stripe, type StripeElements } from '@stripe/stripe-js';
	import type { ClientPayment } from '#lib/clientPayment.js';
	import { PAYMENT_SENT_MESSAGE, payButtonLabel } from '#lib/clientRegister.js';
	import Button from '#lib/components/atoms/Button.svelte';
	import Notice from '#lib/components/atoms/Notice.svelte';

	interface Properties {
		payment: ClientPayment;
		/**
		The Invoice's amount, already formatted -- the figure in the button.
		*/
		amount: string;
		/**
		Where a payment method that leaves the page brings her back.
		*/
		returnUrl: string;
	}

	let { payment, amount, returnUrl }: Properties = $props();

	let mountNode: HTMLElement;
	let stripe: Stripe | undefined;
	let elements: StripeElements | undefined;
	let isReady = $state(false);
	let isPaying = $state(false);
	let isSent = $state(false);
	let error = $state('');

	onMount(async () => {
		try {
			stripe = (await loadStripe(payment.publishableKey, { stripeAccount: payment.stripeAccountId })) ?? undefined;
			if (!stripe) throw new Error('Stripe could not be loaded');
			elements = stripe.elements({ clientSecret: payment.clientSecret });
			const element = elements.create('payment');
			element.on('ready', () => (isReady = true));
			element.mount(mountNode);
		} catch {
			error = 'We could not load the payment form. Try again.';
		}
	});

	async function pay() {
		error = '';
		isPaying = true;
		try {
			const result = await stripe!.confirmPayment({
				elements: elements!,
				confirmParams: { return_url: returnUrl },
				redirect: 'if_required'
			});
			if (result.error) {
				// Stripe's own message is written for the person paying.
				error = result.error.message ?? 'Your payment did not go through. Try again.';
			} else {
				isSent = true;
			}
		} catch {
			error = 'Your payment did not go through. Try again.';
		} finally {
			isPaying = false;
		}
	}
</script>

<!--
	The marked place #1564 drew and #1020 fills: Stripe mounts its Element
	into this node.
-->
{#if isSent}
	<Notice variant="status" message={PAYMENT_SENT_MESSAGE} />
{/if}
<div class="element" hidden={isSent} data-payment-element-mount bind:this={mountNode}></div>
{#if error}
	<Notice variant="error" message={error} />
{/if}
{#if !isSent}
	<Button
		label={payButtonLabel(amount)}
		variant="primary"
		disabled={!isReady}
		loading={isPaying}
		onClick={pay}
	/>
{/if}

<style>
	@layer components {
		/* Stripe sizes its own iframe to the width it is given. */
		.element {
			min-inline-size: 0;
		}
	}
</style>
