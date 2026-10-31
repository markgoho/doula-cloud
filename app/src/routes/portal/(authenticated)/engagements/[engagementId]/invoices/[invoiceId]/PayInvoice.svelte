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
	 * What a Client reads afterwards turns on the PaymentIntent's status
	 * (#2225), never on the mere absence of an error. `succeeded` (a card)
	 * says her payment was sent: the Invoice stays "Not yet paid" until the
	 * `invoice.paid` webhook lands, and the sentence says so rather than
	 * flipping the label early. `processing` (a bank debit, for days) is
	 * handed to the page through `onProcessing`, which says the payment is
	 * on its way and when to expect it. `requires_payment_method` did not
	 * go through, and she can try again.
	 *
	 * A method that leaves the page (Klarna, Cash App Pay) comes back to
	 * `returnUrl` with the PaymentIntent's client secret in the address.
	 * The page hands it over as `returnedClientSecret`, and the outcome is
	 * read from Stripe by that secret -- never from `redirect_status`,
	 * which is only words in a URL anyone can type.
	 */
	import { onMount } from 'svelte';
	import { loadStripe, type PaymentIntent, type Stripe, type StripeElements } from '@stripe/stripe-js';
	import type { ClientPayment } from '#lib/clientPayment.js';
	import { PAYMENT_NOT_THROUGH_MESSAGE, PAYMENT_SENT_MESSAGE, payButtonLabel } from '#lib/clientRegister.js';
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
		/**
		The client secret Stripe put in the address on her way back from a method that left the page, if she is
		returning from one.
		*/
		returnedClientSecret?: string;
		/**
		Her payment is on its way: Stripe reported it `processing`.
		*/
		onProcessing: () => void;
	}

	let { payment, amount, returnUrl, returnedClientSecret, onProcessing }: Properties = $props();

	let mountNode: HTMLElement;
	let stripe: Stripe | undefined;
	let elements: StripeElements | undefined;
	let isReady = $state(false);
	let isPaying = $state(false);
	let isSent = $state(false);
	let error = $state('');

	/** States the outcome Stripe reports for her payment. A status this
	 * screen has no sentence for is read as sent, the claim that holds for
	 * every outcome that is not a failure: the label stays the model's. */
	function settle(status: PaymentIntent.Status | undefined) {
		if (status === 'processing') {
			onProcessing();
		} else if (status === 'requires_payment_method') {
			error = PAYMENT_NOT_THROUGH_MESSAGE;
		} else {
			isSent = true;
		}
	}

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
			return;
		}
		if (returnedClientSecret) {
			try {
				const result = await stripe.retrievePaymentIntent(returnedClientSecret);
				settle(result.paymentIntent?.status ?? 'requires_payment_method');
			} catch {
				error = PAYMENT_NOT_THROUGH_MESSAGE;
			}
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
				error = result.error.message ?? PAYMENT_NOT_THROUGH_MESSAGE;
			} else {
				settle(result.paymentIntent?.status);
			}
		} catch {
			error = PAYMENT_NOT_THROUGH_MESSAGE;
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
