// The Auth emulator has no TOTP, so a local operator's second factor is its phone factor. Deployed, the operator tenant holds TOTP only.
export function isPhoneHint(hint: { factorId: string }): boolean {
	return hint.factorId === 'phone';
}

// Where the emulator lists the codes it would have texted; undefined when no emulator is configured.
export function codeListURL(emulatorHost: string | undefined, tenantId: string): string | undefined {
	if (!emulatorHost) return undefined;
	return `http://${emulatorHost}/emulator/v1/projects/doula-cloud/tenants/${tenantId}/verificationCodes`;
}
