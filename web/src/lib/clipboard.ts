/**
 * Copies text, returning whether it worked.
 *
 * navigator.clipboard only exists in a secure context (HTTPS, or localhost),
 * so on a self-hosted Lithium reached over plain http://<server>:8080 it's
 * missing. Then we fall back to the older execCommand('copy') on a hidden
 * textarea, which browsers still allow during a click.
 */
export async function copyText(text: string): Promise<boolean> {
	if (navigator.clipboard && window.isSecureContext) {
		try {
			await navigator.clipboard.writeText(text);
			return true;
		} catch {
			/* blocked (e.g. permissions); try the fallback */
		}
	}
	const el = document.createElement('textarea');
	el.value = text;
	el.setAttribute('readonly', '');
	// Off-screen but still selectable; fixed so the page doesn't jump.
	el.style.position = 'fixed';
	el.style.top = '0';
	el.style.left = '-9999px';
	document.body.appendChild(el);
	const focused = document.activeElement as HTMLElement | null;
	el.select();
	let ok = false;
	try {
		ok = document.execCommand('copy');
	} catch {
		ok = false;
	}
	el.remove();
	focused?.focus();
	return ok;
}
