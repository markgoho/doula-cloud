import { describe, expect, it, vi } from 'vitest';
import { EmailRequestForm } from './emailRequestForm.svelte.js';

function setup(send = vi.fn().mockResolvedValue(undefined)) {
	const form = new EmailRequestForm('email-id', send);
	const event = new SubmitEvent('submit', { cancelable: true });
	vi.spyOn(event, 'preventDefault');
	return { form, event, send };
}

describe('EmailRequestForm', () => {
	it('refuses a blank address against its field and sends nothing', async () => {
		const { form, event, send } = setup();
		form.email = ' '.repeat(3);

		await form.submit(event);

		expect(form.submission.errors).toStrictEqual([{ message: 'Enter your email address', targetId: 'email-id' }]);
		expect(send).not.toHaveBeenCalled();
		expect(form.isSent).toBe(false);
	});

	it('hands the address to the screen and marks it sent', async () => {
		const { form, event, send } = setup();
		form.email = 'maya@example.test';

		await form.submit(event);

		expect(event.preventDefault).toHaveBeenCalledOnce();
		expect(send).toHaveBeenCalledWith('maya@example.test');
		expect(form.isSent).toBe(true);
	});

	it('shows what the screen says was refused and is not sent', async () => {
		const { form, event } = setup(vi.fn().mockResolvedValue([{ message: 'Try again later' }]));
		form.email = 'maya@example.test';

		await form.submit(event);

		expect(form.submission.errors).toStrictEqual([{ message: 'Try again later' }]);
		expect(form.isSent).toBe(false);
	});
});
