<script lang="ts">
	import { onMount } from 'svelte';
	import { invalidate } from '$app/navigation';
	import {
		adminSettings,
		ApiError,
		emailSettings,
		getSettings,
		sendTestEmail,
		type EmailEncryption,
		type EmailSettings,
		type Settings
	} from '$lib/api';
	import Icon from '$lib/components/Icon.svelte';
	import Toggle from '$lib/components/Toggle.svelte';
	import Modal from '$lib/components/Modal.svelte';

	let current = $state<EmailSettings | null>(null);
	let settings = $state<Settings | null>(null);
	let loadFailed = $state(false);

	// The form, filled from the saved setup.
	let host = $state('');
	let port = $state(587);
	let tls = $state<EmailEncryption>('starttls');
	let username = $state('');
	let password = $state('');
	let from = $state('');
	let baseUrl = $state('');
	// The saved password is only kept while the server and username stay the
	// same (the server refuses to send it anywhere else).
	const keepsPassword = $derived(
		!!current?.hasPassword &&
			host.trim().toLowerCase() === current.host.toLowerCase() &&
			Number(port) === current.port &&
			username.trim() === current.username
	);

	function fill(es: EmailSettings) {
		current = es;
		host = es.host;
		port = es.port;
		tls = es.tls;
		username = es.username;
		password = '';
		from = es.from;
		// Suggest the address this page is open on; it's usually right.
		baseUrl = es.baseUrl || (es.source === '' ? location.origin : '');
	}

	onMount(async () => {
		try {
			const [es, st] = await Promise.all([emailSettings.get(), getSettings()]);
			fill(es);
			settings = st;
		} catch {
			loadFailed = true;
		}
	});

	const fromEnv = $derived(current?.source === 'environment');
	const configured = $derived(!!current?.source);

	// --- Encryption: picking one moves the port to its usual number, unless
	// the admin typed a different port themselves ---
	const encryptions: { value: EmailEncryption; label: string; port: number; hint: string }[] = [
		{ value: 'starttls', label: 'STARTTLS', port: 587, hint: 'Most providers. Usually port 587.' },
		{ value: 'tls', label: 'TLS', port: 465, hint: 'Encrypted from the start. Usually port 465.' },
		{ value: 'none', label: 'None', port: 1025, hint: 'Only for a local test server such as Mailpit. No login is sent.' }
	];
	const usualPorts = new Set([587, 465, 1025, 25]);

	function chooseEncryption(value: EmailEncryption) {
		tls = value;
		if (usualPorts.has(Number(port))) port = encryptions.find((e) => e.value === value)!.port;
		if (value === 'none') {
			username = '';
			password = '';
		}
	}

	// --- Saving ---
	let saving = $state(false);
	let formError = $state('');
	let savedFlash = $state(false);
	let flashTimer: ReturnType<typeof setTimeout>;

	async function save(e: SubmitEvent) {
		e.preventDefault();
		saving = true;
		formError = '';
		try {
			const es = await emailSettings.save({
				host: host.trim(),
				port: Number(port),
				tls,
				username: username.trim(),
				...(password ? { password } : {}),
				from: from.trim(),
				baseUrl: current?.baseUrlFromEnv ? '' : baseUrl.trim()
			});
			fill(es);
			testState = 'idle';
			await invalidate('app:settings');
			settings = await getSettings();
			clearTimeout(flashTimer);
			savedFlash = true;
			flashTimer = setTimeout(() => (savedFlash = false), 2500);
		} catch (err) {
			const msg = err instanceof ApiError ? err.message : 'Something went wrong, please try again';
			formError = msg.charAt(0).toUpperCase() + msg.slice(1) + '.';
		} finally {
			saving = false;
		}
	}

	// --- Turning email off ---
	let confirmRemove = $state(false);
	async function remove() {
		confirmRemove = false;
		saving = true;
		formError = '';
		try {
			fill(await emailSettings.remove());
			testState = 'idle';
			await invalidate('app:settings');
			settings = await getSettings();
		} catch {
			formError = "Couldn't turn email off. Try again.";
		} finally {
			saving = false;
		}
	}

	// --- Test email: sent straight away, so mistakes show up here ---
	let testState = $state<'idle' | 'sending' | 'sent' | 'error'>('idle');
	let testMessage = $state('');

	async function testEmail() {
		testState = 'sending';
		try {
			const { sentTo } = await sendTestEmail();
			testState = 'sent';
			testMessage = `Sent to ${sentTo}. If it doesn't arrive, check your spam folder.`;
		} catch (e) {
			testState = 'error';
			const reason = e instanceof ApiError ? e.message : 'something went wrong';
			testMessage = reason.charAt(0).toUpperCase() + reason.slice(1) + '.';
		}
	}

	// --- Notification emails switch ---
	let toggleSaving = $state(false);
	let toggleError = $state(false);
	async function setNotifications(on: boolean) {
		toggleSaving = true;
		toggleError = false;
		try {
			settings = await adminSettings.update({ emailNotifications: on });
			await invalidate('app:settings');
		} catch {
			if (settings) settings.emailNotifications = !on;
			toggleError = true;
		} finally {
			toggleSaving = false;
		}
	}

	const label = 'mb-1.5 block text-[13px] font-medium text-ink-200';
	const hint = 'mt-1.5 text-xs text-ink-500';
</script>

{#if loadFailed}
	<p class="mt-5 text-sm text-danger-200">Couldn't load the email settings.</p>
{:else if !current}
	<p class="mt-5 text-sm text-ink-500">Loading…</p>
{:else}
	<!-- Status -->
	{#if !configured}
		<div class="mt-5 flex items-start gap-3 rounded-xl border border-danger-400/25 bg-danger-400/10 px-4 py-3.5">
			<Icon name="mail" class="mt-0.5 size-4 shrink-0 text-danger-300" />
			<div class="text-sm">
				<p class="font-medium text-danger-200">Email isn't set up yet</p>
				<p class="mt-0.5 text-ink-300">
					People are only notified inside Lithium for now. Enter your mail provider's SMTP details below to send
					email notifications too. You'll find them in your provider's help pages, often under "SMTP" or "app passwords".
				</p>
			</div>
		</div>
	{:else if fromEnv}
		<div class="mt-5 flex items-start gap-3 rounded-xl border border-ink-800 bg-ink-900/60 px-4 py-3.5">
			<Icon name="lock" class="mt-0.5 size-4 shrink-0 text-ink-400" />
			<div class="text-sm">
				<p class="font-medium text-ink-100">Set by the server</p>
				<p class="mt-0.5 text-ink-400">
					This server's <code class="font-mono text-[12px]">LITHIUM_SMTP_*</code> environment variables set up email,
					so it can't be changed here. Edit them (for example in <code class="font-mono text-[12px]">.env</code>, or
					by running <code class="font-mono text-[12px]">install.sh</code> again) and restart Lithium. Remove them to
					manage email from this page instead.
				</p>
			</div>
		</div>
	{:else}
		<div class="mt-5 flex items-start gap-3 rounded-xl border border-primary-400/25 bg-primary-400/10 px-4 py-3.5">
			<Icon name="check" class="mt-0.5 size-4 shrink-0 text-primary-300" />
			<p class="text-sm text-ink-200">
				Email is set up. Sending through <span class="font-medium">{current.host}:{current.port}</span> as
				<span class="font-medium">{current.from}</span>.
			</p>
		</div>
	{/if}

	<!-- SMTP server -->
	<div class="mt-5 rounded-xl border border-ink-800 bg-ink-900/60">
		<div class="flex items-center justify-between gap-3 border-b border-ink-800 px-5 py-4">
			<div>
				<h3 class="text-sm font-semibold">Mail server (SMTP)</h3>
				<p class="mt-0.5 text-sm text-ink-400">How Lithium sends email. Changes apply straight away.</p>
			</div>
			<span class="shrink-0 text-xs" aria-live="polite">
				{#if saving}
					<span class="text-ink-500">Saving…</span>
				{:else if savedFlash}
					<span class="flex items-center gap-1 text-ink-400"><Icon name="check" class="size-3.5 text-primary-300" /> Saved</span>
				{/if}
			</span>
		</div>

		{#if fromEnv}
			<dl class="grid gap-x-6 gap-y-3 px-5 py-4 text-sm sm:grid-cols-[max-content_1fr]">
				<dt class="text-ink-500">SMTP server</dt>
				<dd class="text-ink-200">{current.host}:{current.port}</dd>
				<dt class="text-ink-500">Encryption</dt>
				<dd class="text-ink-200">{encryptions.find((e) => e.value === current?.tls)?.label ?? current.tls}</dd>
				<dt class="text-ink-500">Username</dt>
				<dd class="text-ink-200">{current.username || 'None'}</dd>
				<dt class="text-ink-500">From address</dt>
				<dd class="text-ink-200">{current.from}</dd>
				<dt class="text-ink-500">Lithium address</dt>
				<dd class="text-ink-200">{current.baseUrl || 'Not set, so emails can\'t link to tickets'}</dd>
			</dl>
		{:else}
			<form onsubmit={save} class="space-y-5 px-5 py-4" autocomplete="off">
				<div class="grid gap-4 sm:grid-cols-[1fr_8rem]">
					<div>
						<label for="smtp-host" class={label}>SMTP server</label>
						<input id="smtp-host" bind:value={host} required placeholder="smtp.example.com" spellcheck="false" class="field h-9 w-full text-[13px]" />
					</div>
					<div>
						<label for="smtp-port" class={label}>Port</label>
						<input id="smtp-port" type="number" inputmode="numeric" min="1" max="65535" bind:value={port} required class="field h-9 w-full text-[13px]" />
					</div>
				</div>

				<fieldset>
					<legend class={label}>Encryption</legend>
					<div class="inline-flex rounded-lg border border-ink-700 p-0.5" role="radiogroup">
						{#each encryptions as e (e.value)}
							<button
								type="button"
								role="radio"
								aria-checked={tls === e.value}
								onclick={() => chooseEncryption(e.value)}
								class={[
									'h-8 rounded-md px-3 text-[13px] transition',
									tls === e.value ? 'bg-ink-800 font-medium text-ink-100' : 'text-ink-400 hover:text-ink-100'
								]}
							>
								{e.label}
							</button>
						{/each}
					</div>
					<p class={hint}>{encryptions.find((e) => e.value === tls)?.hint}</p>
				</fieldset>

				{#if tls !== 'none'}
					<div class="grid gap-4 sm:grid-cols-2">
						<div>
							<label for="smtp-user" class={label}>Username</label>
							<input id="smtp-user" bind:value={username} autocomplete="off" spellcheck="false" placeholder="Often your email address" class="field h-9 w-full text-[13px]" />
						</div>
						<div>
							<label for="smtp-pass" class={label}>Password</label>
							<input
								id="smtp-pass"
								type="password"
								bind:value={password}
								autocomplete="new-password"
								disabled={!username.trim()}
								placeholder={keepsPassword ? 'Saved (leave blank to keep)' : current.hasPassword ? 'Enter it again for the new server' : ''}
								class="field h-9 w-full text-[13px] disabled:opacity-50"
							/>
						</div>
					</div>
					<p class="-mt-3 text-xs text-ink-500">
						Leave the username blank if your server doesn't need a login. Some providers want an app password rather than
						your normal one.
					</p>
				{/if}

				<div>
					<label for="smtp-from" class={label}>From address</label>
					<input id="smtp-from" type="email" bind:value={from} required placeholder="help@example.com" class="field h-9 w-full text-[13px]" />
					<p class={hint}>
						Emails come from this address, shown with your company name. Your provider may only allow addresses it
						knows about.
					</p>
				</div>

				<div>
					<label for="smtp-base" class={label}>Lithium address</label>
					{#if current.baseUrlFromEnv}
						<p class="text-sm text-ink-200">{current.baseUrl}</p>
						<p class={hint}>Set by <code class="font-mono text-[11px]">LITHIUM_BASE_URL</code> on the server.</p>
					{:else}
						<input id="smtp-base" type="url" bind:value={baseUrl} placeholder="https://help.example.com" class="field h-9 w-full text-[13px]" />
						<p class={hint}>Where people open Lithium, so emails can link straight to the ticket. Leave blank for no links.</p>
					{/if}
				</div>

				{#if formError}
					<p class="rounded-lg border border-danger-400/25 bg-danger-400/10 px-3 py-2 text-sm text-danger-200" role="alert">{formError}</p>
				{/if}

				<div class="flex flex-wrap items-center justify-between gap-3 pt-1">
					<button
						type="submit"
						disabled={saving}
						class="h-9 rounded-lg bg-primary-400 px-4 text-[13px] font-semibold text-primary-950 transition hover:bg-primary-300 disabled:cursor-not-allowed disabled:opacity-40"
					>
						{configured ? 'Save changes' : 'Set up email'}
					</button>
					{#if configured}
						<button
							type="button"
							onclick={() => (confirmRemove = true)}
							disabled={saving}
							class="flex h-9 items-center gap-2 rounded-lg border border-ink-700 px-3 text-[13px] text-ink-300 transition hover:border-danger-400/50 hover:text-danger-200 disabled:opacity-40"
						>
							<Icon name="trash" class="size-4" />
							Turn off email
						</button>
					{/if}
				</div>
			</form>
		{/if}
	</div>

	<!-- Notifications and testing, once there's somewhere to send from -->
	{#if configured && settings}
		<div class="mt-5 rounded-xl border border-ink-800 bg-ink-900/60">
			<div class="flex items-center justify-between gap-3 border-b border-ink-800 px-5 py-4">
				<h3 class="text-sm font-semibold">Notifications</h3>
				<span class="text-xs" aria-live="polite">
					{#if toggleSaving}
						<span class="text-ink-500">Saving…</span>
					{:else if toggleError}
						<span class="text-danger-300">Couldn't save. Try again.</span>
					{/if}
				</span>
			</div>
			<div class="px-5 py-4">
				<Toggle bind:checked={settings.emailNotifications} disabled={toggleSaving} onchange={setNotifications}>
					Email notifications
					{#snippet description()}
						Email people about replies, assignments, mentions and changes on their tickets, as well as notifying them
						in the app. Everyone can turn their own emails off from their profile. Emails never include reply text,
						and leave out ticket subjects while sensitive mode is on.
					{/snippet}
				</Toggle>
				<div class="mt-4 flex flex-wrap items-center gap-3">
					<button
						type="button"
						onclick={testEmail}
						disabled={testState === 'sending'}
						class="inline-flex h-8 items-center gap-1.5 rounded-lg border border-ink-700 px-3 text-[13px] text-ink-200 transition hover:border-ink-500 hover:text-ink-100 disabled:cursor-not-allowed disabled:opacity-60"
					>
						<Icon name="mail" class="size-3.5" />
						{testState === 'sending' ? 'Sending…' : 'Send test email'}
					</button>
					<span aria-live="polite" class="min-w-0 text-xs">
						{#if testState === 'sent'}
							<span class="text-ink-400">{testMessage}</span>
						{:else if testState === 'error'}
							<span class="break-words text-danger-300">{testMessage}</span>
						{/if}
					</span>
				</div>
			</div>
		</div>
	{/if}
{/if}

<Modal
	bind:open={confirmRemove}
	title="Turn off email?"
	description="Lithium will stop sending emails, and the saved SMTP details are removed. People still get notifications in the app."
>
	<div class="mt-2 flex justify-end gap-2">
		<button
			type="button"
			onclick={() => (confirmRemove = false)}
			class="inline-flex h-9 items-center rounded-lg border border-ink-700 px-3.5 text-sm font-medium text-ink-200 transition hover:border-ink-500 hover:text-ink-100"
		>
			Keep it
		</button>
		<button
			type="button"
			onclick={remove}
			class="inline-flex h-9 items-center rounded-lg bg-danger-600 px-3.5 text-sm font-semibold text-white transition hover:bg-danger-500"
		>
			Turn off email
		</button>
	</div>
</Modal>
