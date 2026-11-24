<script lang="ts">
	import { onMount } from 'svelte';
	import { ApiError, twoFactor, type TwoFactorSetup } from '$lib/api';
	import CodeInput from './CodeInput.svelte';
	import RecoveryCodes from './RecoveryCodes.svelte';
	import Icon from './Icon.svelte';

	// Setting up an authenticator app: scan the QR code, confirm with a code,
	// then save the recovery codes.
	let {
		email,
		ondone,
		oncancel,
		doneLabel = 'Continue'
	}: { email: string; ondone: () => void; oncancel?: () => void; doneLabel?: string } = $props();

	let setup = $state<TwoFactorSetup | null>(null);
	let loadError = $state('');
	let code = $state('');
	let verifying = $state(false);
	let error = $state('');
	let recoveryCodes = $state<string[] | null>(null);
	let saved = $state(false);
	let showSecret = $state(false);

	onMount(async () => {
		try {
			setup = await twoFactor.start();
		} catch {
			loadError = "Couldn't start setup. Refresh the page to try again.";
		}
	});

	// The secret in groups of four is easier to type into an app by hand.
	const groupedSecret = $derived(setup?.secret.match(/.{1,4}/g)?.join(' ') ?? '');

	async function confirm(e?: SubmitEvent) {
		e?.preventDefault();
		if (verifying) return;
		if (!/^\d{6}$/.test(code)) {
			error = 'Enter the 6-digit code from your app.';
			return;
		}
		error = '';
		verifying = true;
		try {
			recoveryCodes = (await twoFactor.enable(code)).recoveryCodes;
		} catch (err) {
			error =
				err instanceof ApiError && err.status === 400
					? "That code didn't match. Check your phone's clock is set automatically, then try the next code."
					: 'Something went wrong. Please try again.';
			code = '';
		} finally {
			verifying = false;
		}
	}
</script>

{#if recoveryCodes}
	<div>
		<p class="flex items-center gap-2 text-sm font-medium text-ink-100">
			<span class="flex size-5 items-center justify-center rounded-full bg-primary-400 text-primary-950">
				<Icon name="check" class="size-3" />
			</span>
			Two-factor authentication is on
		</p>
		<p class="mt-2 mb-4 text-sm text-ink-400">
			If you lose your phone, sign in with one of these recovery codes instead. Each works once.
		</p>
		<RecoveryCodes codes={recoveryCodes} {email} bind:saved />
		<button
			type="button"
			disabled={!saved}
			onclick={ondone}
			class="mt-5 flex h-10 w-full items-center justify-center rounded-lg bg-primary-400 text-sm font-semibold text-primary-950 transition hover:bg-primary-300 disabled:cursor-not-allowed disabled:opacity-50"
		>
			{doneLabel}
		</button>
	</div>
{:else if loadError}
	<p role="alert" class="rounded-lg border border-danger-400/25 bg-danger-400/10 px-3 py-2.5 text-sm text-danger-200">
		{loadError}
	</p>
{:else}
	<form onsubmit={confirm} novalidate>
		<ol class="space-y-5 text-sm">
			<li>
				<p class="font-medium text-ink-100">1. Scan this with your authenticator app</p>
				<p class="mt-1 text-ink-400">Google Authenticator, Microsoft Authenticator, 1Password, Authy or similar.</p>
				<div class="mt-3 flex items-center gap-4">
					<div class="size-36 shrink-0 overflow-hidden rounded-lg bg-white p-1">
						{#if setup}
							<img src={setup.qr} alt="QR code for your authenticator app" class="size-full [image-rendering:pixelated]" />
						{:else}
							<div class="size-full animate-pulse rounded bg-ink-200/40"></div>
						{/if}
					</div>
					<div class="min-w-0 text-xs text-ink-400">
						Can't scan it?
						{#if showSecret && setup}
							<span class="mt-1 block">Enter this key in your app:</span>
							<code class="mt-1.5 block rounded-md border border-ink-800 bg-ink-950/60 px-2 py-1.5 font-mono text-[13px] break-all text-ink-100 select-all">{groupedSecret}</code>
						{:else}
							<button type="button" onclick={() => (showSecret = true)} class="mt-1 block font-medium text-primary-300 hover:text-primary-200">
								Show the setup key
							</button>
						{/if}
					</div>
				</div>
			</li>
			<li>
				<label for="enrol-code" class="font-medium text-ink-100">2. Enter the 6-digit code it shows</label>
				<div class="mt-2.5">
					<CodeInput id="enrol-code" bind:value={code} disabled={!setup || verifying} oncomplete={() => confirm()} />
				</div>
			</li>
		</ol>

		{#if error}
			<p role="alert" class="mt-4 rounded-lg border border-danger-400/25 bg-danger-400/10 px-3 py-2.5 text-sm text-danger-200">
				{error}
			</p>
		{/if}

		<div class="mt-5 flex gap-2">
			{#if oncancel}
				<button type="button" onclick={oncancel} class="h-10 rounded-lg px-3.5 text-sm text-ink-400 transition hover:bg-ink-850 hover:text-ink-100">
					Cancel
				</button>
			{/if}
			<button
				type="submit"
				disabled={!setup || verifying}
				class="flex h-10 flex-1 items-center justify-center rounded-lg bg-primary-400 text-sm font-semibold text-primary-950 transition hover:bg-primary-300 disabled:cursor-not-allowed disabled:opacity-60"
			>
				{verifying ? 'Checking…' : 'Turn on two-factor'}
			</button>
		</div>
	</form>
{/if}
