<script lang="ts">
	// One input for a 6-digit authenticator code, or (with allowRecovery) a
	// recovery code like "k7m2p-xq4ra". Calls oncomplete as soon as six digits
	// are typed, so people don't have to press the button.
	let {
		value = $bindable(''),
		id = 'code',
		allowRecovery = false,
		disabled = false,
		autofocus = false,
		oncomplete
	}: {
		value?: string;
		id?: string;
		allowRecovery?: boolean;
		disabled?: boolean;
		autofocus?: boolean;
		oncomplete?: (code: string) => void;
	} = $props();

	function oninput(e: Event) {
		const el = e.currentTarget as HTMLInputElement;
		if (!allowRecovery) el.value = el.value.replace(/\D/g, '').slice(0, 6);
		value = el.value;
		if (/^\d{6}$/.test(value.replace(/\s/g, ''))) oncomplete?.(value);
	}

	function focusOnMount(el: HTMLInputElement) {
		if (autofocus) el.focus();
	}
</script>

<input
	{id}
	{value}
	{oninput}
	{disabled}
	use:focusOnMount
	type="text"
	inputmode={allowRecovery ? 'text' : 'numeric'}
	autocomplete="one-time-code"
	autocapitalize="off"
	spellcheck="false"
	maxlength={allowRecovery ? 20 : 6}
	placeholder={allowRecovery ? '000000 or recovery code' : '000000'}
	class={[
		'block h-12 w-full rounded-lg border border-ink-700 bg-ink-950/60 px-3 text-center font-mono text-ink-100 placeholder:text-ink-600 transition outline-none focus:border-primary-400/70 focus:ring-3 focus:ring-primary-400/15 disabled:opacity-60',
		allowRecovery ? 'text-lg tracking-[0.2em] placeholder:text-sm placeholder:tracking-normal placeholder:font-sans' : 'text-xl tracking-[0.5em]'
	]}
/>
