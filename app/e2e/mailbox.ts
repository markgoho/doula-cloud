// The sandbox mailbox: where a persona's email actually goes (#764,
// under map #759).
//
// Doula Cloud reaches a person by email and nowhere else at eleven
// points -- a Staff invitation, a Client's portal invitation, a
// verification link, an Offer, a payment receipt. Several of those are
// the first thing that ever happens to somebody, so a simulation run
// that reads a token out of `staff_invite_outbox` (the #203 shape, still
// used by stack.ts's readStaffInviteToken) has skipped the act it exists
// to observe: opening the mail and clicking what is in it.
//
// This process is a Mailgun-shaped sink. It answers the one endpoint
// mail.MailgunSender posts to -- `POST /v3/<domain>/messages` -- holds
// the message, and serves it back two ways: as JSON for the harness to
// assert against, and as an HTML inbox a persona opens in Chrome and
// clicks like webmail. Nothing leaves the machine. It also plays
// Mailgun's *event* side, signing and posting the bounce and complaint
// webhooks that no Mailgun CLI forwards to localhost.
//
// Run it with `bun app/e2e/mailbox.ts`; e2e/stack.ts starts it as a host
// process beside the BFF and the Auth emulator, and points the BFF at it
// with MAILGUN_API_BASE.
import { createHmac, randomUUID } from 'node:crypto';
import { E2E_API_HOST, E2E_API_PORT, MAILBOX_HOST, MAILBOX_PORT } from './ports';

// One captured message. `seq` is arrival order and it is deliberately
// the only ordering the inbox shows -- see the clock note on `label`.
type Captured = {
	id: string;
	seq: number;
	to: string;
	from: string;
	replyTo: string;
	subject: string;
	text: string;
	label: string;
};

const messages: Captured[] = [];
// What the run calls the moment this message was sent -- "day 0",
// "month 3, week 2". The harness sets it on each clock jump (#762).
//
// The inbox shows this and never a wall-clock timestamp. The catcher
// receives at real time while the run is six simulated months in, so a
// real timestamp would be a lie a persona could narrate confusion about,
// and #762 already rules "the notification arrived too late"
// inadmissible. Sequence plus the run's own label is everything an
// entry is allowed to say about when.
const runClock = { label: 'unlabelled' };

const BOUNCE_TARGET = `http://${E2E_API_HOST}:${E2E_API_PORT}/api/mailgun/webhook`;

// The signing key the BFF verifies the bounce/complaint webhook against
// (MAILGUN_WEBHOOK_SIGNING_KEY). Locally it is any agreed string --
// stack.ts sets the same value on both processes. Mailgun's own scheme:
// HMAC-SHA256 over timestamp+token, hex (api/internal/portalinvite/
// bounce_webhook.go). There is no skew check on the timestamp, so a
// jumped clock cannot invalidate a signature.
const SIGNING_KEY = process.env.MAILGUN_WEBHOOK_SIGNING_KEY ?? 'e2e-mailgun-signing-key';

// Mailgun addresses are matched case-insensitively, and a persona who
// types her own address into a form types it however she likes.
function normalize(address: string): string {
	return address.trim().toLowerCase();
}

// `To` arrives as Mailgun's `to` form field, which may be
// `Renata Vela <renata@sim.doula.cloud>`. The bare address is what an
// inbox is keyed on.
function bareAddress(to: string): string {
	const angled = /<([^>]+)>/.exec(to);
	return normalize(angled ? angled[1] : to);
}

function escapeHTML(value: string): string {
	return value.replaceAll('&', '&amp;').replaceAll('<', '&lt;').replaceAll('>', '&gt;').replaceAll('"', '&quot;');
}

// Notification mail is plain text carrying one or more links (ADR-0030
// rules out any tracking, so the URL in the body is the URL she
// follows). Rendering those as anchors is what makes the inbox
// clickable, and what lets a run find a broken link by clicking it
// rather than by reading a database column.
function renderBody(text: string): string {
	return escapeHTML(text).replaceAll(/https?:\/\/[^\s<]+/g, (url) => `<a href="${url}">${url}</a>`);
}

function page(title: string, body: string): Response {
	return new Response(
		`<!doctype html><html lang="en"><head><meta charset="utf-8"><title>${escapeHTML(title)}</title>` +
			'<style>body{font:16px/1.5 system-ui,sans-serif;margin:2rem auto;max-width:44rem;padding:0 1rem}' +
			'ul{padding-left:1rem}li{margin-bottom:.75rem}pre{white-space:pre-wrap;word-break:break-word}' +
			'.meta{color:#555;font-size:.875rem}</style></head><body>' +
			body +
			'</body></html>',
		{ headers: { 'content-type': 'text/html; charset=utf-8' } }
	);
}

function json(value: unknown, status = 200): Response {
	return Response.json(value, { status });
}

function inboxFor(address: string): Captured[] {
	return messages.filter((message) => bareAddress(message.to) === normalize(address));
}

// Mailgun's webhook body, signed the way Mailgun signs it. `failed` +
// `permanent` is a hard bounce and `complained` is a spam report; the
// BFF writes an email_suppressions row for either, after which every one
// of the eleven mail kinds refuses that address through
// mailsuppress.Sender. That is the whole loop #743 proved against the
// deployed service, reproduced here because Mailgun has no CLI forwarder
// that could reach a localhost endpoint.
async function fireDeliveryEvent(recipient: string, event: string, reason: string): Promise<Response> {
	const timestamp = String(Math.floor(Date.now() / 1000));
	const token = randomUUID().replaceAll('-', '');
	const signature = createHmac('sha256', SIGNING_KEY).update(timestamp + token).digest('hex');
	const body = {
		signature: { timestamp, token, signature },
		'event-data': {
			id: randomUUID(),
			event,
			recipient: normalize(recipient),
			severity: event === 'failed' ? 'permanent' : '',
			reason
		}
	};
	const delivered = await fetch(BOUNCE_TARGET, {
		method: 'POST',
		headers: { 'content-type': 'application/json' },
		body: JSON.stringify(body)
	});
	return json({ posted: BOUNCE_TARGET, status: delivered.status, event: body['event-data'] });
}

// The Mailgun endpoint mail.MailgunSender posts to. The domain segment
// is whatever MAILGUN_DOMAIN is set to and is not checked: this sink
// holds mail for every domain a run cares to use, and a mismatch would
// only produce a failure Mailgun itself would not produce.
async function capture(request: Request): Promise<Response> {
	const form = await request.formData();
	const field = (name: string) => String(form.get(name) ?? '');
	const captured: Captured = {
		id: randomUUID(),
		seq: messages.length + 1,
		to: field('to'),
		from: field('from'),
		replyTo: field('h:Reply-To'),
		subject: field('subject'),
		text: field('text'),
		label: runClock.label
	};
	messages.push(captured);
	// Mailgun's own 200 body. mail.MailgunSender discards it, but a sink
	// that answers in the vendor's shape is one less difference between
	// a run and production.
	return json({ id: `<${captured.id}@mailbox.local>`, message: 'Queued. Thank you.' });
}

function indexPage(): Response {
	const addresses = [...new Set(messages.map((message) => bareAddress(message.to)))].toSorted((a, b) => a.localeCompare(b));
	const items = addresses
		.map((address) => {
			const count = inboxFor(address).length;
			return `<li><a href="/inbox/${encodeURIComponent(address)}">${escapeHTML(address)}</a> <span class="meta">${count} message${count === 1 ? '' : 's'}</span></li>`;
		})
		.join('');
	return page(
		'Sandbox mailboxes',
		`<h1>Sandbox mailboxes</h1><p class="meta">Run clock: ${escapeHTML(runClock.label)}. Nothing here left this machine.</p>` +
			(addresses.length > 0 ? `<ul>${items}</ul>` : '<p>No mail yet.</p>')
	);
}

function inboxPage(address: string): Response {
	const items = inboxFor(address)
		.map(
			(message) =>
				`<li><a href="/inbox/${encodeURIComponent(address)}/${message.id}">${escapeHTML(message.subject)}</a>` +
				`<div class="meta">#${message.seq} &middot; from ${escapeHTML(message.from)} &middot; ${escapeHTML(message.label)}</div></li>`
		)
		.join('');
	return page(
		`Inbox — ${address}`,
		`<h1>Inbox</h1><p class="meta">${escapeHTML(address)}</p>` +
			(items ? `<ul>${items}</ul>` : '<p>No mail yet.</p>')
	);
}

function messagePage(address: string, id: string): Response {
	const message = inboxFor(address).find((candidate) => candidate.id === id);
	if (!message) return page('Not found', '<h1>No such message</h1>');
	return page(
		message.subject,
		`<p><a href="/inbox/${encodeURIComponent(address)}">&larr; Inbox</a></p>` +
			`<h1>${escapeHTML(message.subject)}</h1>` +
			`<p class="meta">From ${escapeHTML(message.from)} &middot; to ${escapeHTML(message.to)} &middot; ` +
			`reply-to ${escapeHTML(message.replyTo)} &middot; #${message.seq} &middot; ${escapeHTML(message.label)}</p>` +
			`<pre>${renderBody(message.text)}</pre>`
	);
}

// A persona's act is opening the inbox in Chrome; the JSON side is the
// harness asserting, and never itself an observed act (docs/simulation/
// README.md's admissibility rules).
async function route(request: Request): Promise<Response> {
	const url = new URL(request.url);
	const { pathname } = url;

	if (request.method === 'POST' && /^\/v3\/[^/]+\/messages$/.test(pathname)) return capture(request);

	// #744's BounceClearer travels with MAILGUN_API_BASE, so the "clear
	// this address from Mailgun's own bounce list" call arrives here too.
	// Nothing local keeps such a list -- the suppression that matters is
	// the `email_suppressions` row, and the BFF clears that itself -- so
	// the honest answer is Mailgun's own: acknowledged.
	if (request.method === 'DELETE' && /^\/v3\/[^/]+\/bounces\/.+$/.test(pathname)) {
		return json({ message: 'Bounced address has been removed' });
	}

	if (pathname === '/api/messages') {
		if (request.method === 'DELETE') {
			messages.length = 0;
			return json({ cleared: true });
		}
		const to = url.searchParams.get('to');
		return json(to ? inboxFor(to) : messages);
	}

	if (pathname === '/api/clock' && request.method === 'POST') {
		const requested = (await request.json()) as { label?: string };
		runClock.label = String(requested.label ?? 'unlabelled');
		return json({ label: runClock.label });
	}

	if (pathname === '/api/delivery-event' && request.method === 'POST') {
		const payload = (await request.json()) as { to?: string; event?: string; reason?: string };
		if (!payload.to) return json({ error: 'to is required' }, 400);
		return fireDeliveryEvent(payload.to, payload.event ?? 'failed', payload.reason ?? 'generic');
	}

	if (pathname === '/') return indexPage();

	const inbox = /^\/inbox\/([^/]+)(?:\/([^/]+))?$/.exec(pathname);
	if (inbox) {
		const address = decodeURIComponent(inbox[1]);
		return inbox[2] ? messagePage(address, inbox[2]) : inboxPage(address);
	}

	return new Response('not found', { status: 404 });
}

Bun.serve({ hostname: MAILBOX_HOST, port: MAILBOX_PORT, fetch: route });
console.log(`sandbox mailbox listening on http://${MAILBOX_HOST}:${MAILBOX_PORT}`);
