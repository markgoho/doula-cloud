/**
 * A screen that asks for an email address and mails a link to it: the
 * Staff "Forgot your password?" and the portal sign-in. Both refuse a blank
 * address the same way, then hand the address to the screen's own request,
 * which returns what to show if the BFF refused it.
 */
import { FormSubmission, orServiceProblem, type FormError } from './formSubmission.svelte.js';

export class EmailRequestForm {
	readonly #send: (email: string) => Promise<FormError[] | void>;
	email = $state('');
	isSent = $state(false);
	readonly submission = new FormSubmission();
	readonly emailId: string;

	submit = async (event: SubmitEvent) => {
		event.preventDefault();
		await this.submission.run(async () => {
			if (this.email.trim() === '') {
				return [{ message: 'Enter your email address', targetId: this.emailId }];
			}
			const errors = await this.#send(this.email);
			if (errors) {
				return errors;
			}
			this.isSent = true;
		}, orServiceProblem);
	};

	constructor(emailId: string, send: (email: string) => Promise<FormError[] | void>) {
		this.emailId = emailId;
		this.#send = send;
	}
}
