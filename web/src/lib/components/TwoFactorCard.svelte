<script lang="ts">
	import { onMount } from 'svelte';
	import { invalidateAll } from '$app/navigation';
	import { ApiError, twoFactor, type TwoFactorStatus } from '$lib/api';
	import Icon from './Icon.svelte';
	import Modal from './Modal.svelte';
	import PasswordField from './PasswordField.svelte';
	import RecoveryCodes from './RecoveryCodes.svelte';
	import TwoFactorEnrol from './TwoFactorEnrol.svelte';

	// Profile → two-factor: set it up, get new recovery codes, or turn it off
	// (unless the workspace requires it).
	let { email }: { email: string } = $props();

	let status = $state<TwoFactorStatus | null>(null);
	let loadFailed = $state(false);
	let enrolling = $state(false);

	async function load() {
		try {
			status = await twoFactor.status();
		} catch {
			loadFailed = true;
		}
	}
	onMount(load);

	async function enrolled() {
		enrolling = false;
		await Promise.all([load(), invalidateAll()]);
	}

	// Both "new codes" and "turn off" ask for the password first.
	let action = $state<'codes' | 'disable' | null>(null);
	let modalOpen = $state(false);
	let password = $state('');
	let busy = $state(false);
	let error = $state('');
	let newCodes = $state<string[] | null>(null);
	let saved = $state(false);

	function open(a: 'codes' | 'disable') {
		action = a;
		password = error = '';
		newCodes = null;
		saved = false;
		modalOpen = true;
	}

	async function submit(e: SubmitEvent) {
		e.preventDefault();
		if (busy || !action) return;
		if (!password) {
			error = 'Enter your password.';
			return;
		}
		busy = true;
		error = '';
		try {
			if (action === 'codes') {
				newCodes = (await twoFactor.newRecoveryCodes(password)).recoveryCodes;
			} else {
				await twoFactor.disable(password);
				modalOpen = false;
			}
			await Promise.all([load(), action === 'disable' ? invalidateAll() : null]);
		} catch (err) {
			error =
				err instanceof ApiError && err.status < 500
					? err.message.charAt(0).toUpperCase() + err.message.slice(1) + '.'
					: 'Something went wrong. Please try again.';
		} finally {
			busy = false;
		}
	}

	const btn =
		'inline-flex h-9 items-center gap-1.5 rounded-lg border border-ink-700 px-3 text-sm font-medium text-ink-200 transition hover:border-ink-600 hover:bg-ink-850 disabled:cursor-not-allowed disabled:opacity-60';
</script>

<section class="mt-6 rounded-xl border border-ink-800 bg-ink-900/60 p-6">
	<div class="flex flex-wrap items-start justify-between gap-3">
		<div>
			<h2 class="flex items-center gap-2 text-sm font-semibold">
				Two-factor authentication
				{#if status?.enabled}
					<span class="inline-flex items-center gap-1 rounded-full border border-primary-400/30 bg-primary-400/10 px-2 py-px text-[11px] font-medium text-primary-200">
						<Icon name="check" class="size-3" /> On
					</span>
				{:else if status}
					<span class="rounded-full border border-ink-700 px-2 py-px text-[11px] font-medium text-ink-400">Off</span>
				{/if}
			</h2>
			<p class="mt-1 text-sm text-ink-400">
				{#if status?.enabled}
					You enter a code from your authenticator app when you sign in.
					{status.recoveryCodesLeft} of 10 recovery codes left.
				{:else}
					Ask for a code from an authenticator app as well as your password when you sign in.
				{/if}
			</p>
		</div>
		{#if status && !status.enabled && !enrolling}
			<button type="button" onclick={() => (enrolling = true)} class="inline-flex h-9 items-center gap-1.5 rounded-lg bg-primary-400 px-3.5 text-sm font-semibold text-primary-950 transition hover:bg-primary-300">
				<Icon name="phone" class="size-4" /> Set up
			</button>
		{/if}
	</div>

	{#if loadFailed}
		<p class="mt-4 text-sm text-primary-200">Couldn't load your two-factor settings.</p>
	{:else if enrolling}
		<div class="mt-5 max-w-sm border-t border-ink-800 pt-5">
			<TwoFactorEnrol {email} ondone={enrolled} oncancel={() => (enrolling = false)} doneLabel="Done" />
		</div>
	{:else if status?.enabled}
		{#if status.recoveryCodesLeft <= 3}
			<p class="mt-4 rounded-lg border border-primary-400/25 bg-primary-400/10 px-3 py-2 text-sm text-primary-200">
				You're running low on recovery codes. Generate new ones so you don't get locked out.
			</p>
		{/if}
		<div class="mt-5 flex flex-wrap items-center gap-2">
			<button type="button" onclick={() => open('codes')} class={btn}>
				<Icon name="refresh" class="size-3.5" /> New recovery codes
			</button>
			{#if status.required}
				<span class="text-xs text-ink-500">Required in this workspace, so it can't be turned off.</span>
			{:else}
				<button type="button" onclick={() => open('disable')} class={btn}>Turn off</button>
			{/if}
		</div>
	{/if}
</section>

<Modal
	bind:open={modalOpen}
	title={action === 'codes' ? 'New recovery codes' : 'Turn off two-factor?'}
	description={action === 'codes'
		? newCodes
			? 'Your old codes no longer work.'
			: 'This replaces your current recovery codes. Enter your password to continue.'
		: "You'll only need your password to sign in. Other devices will be signed out."}
>
	{#if newCodes}
		<RecoveryCodes codes={newCodes} {email} bind:saved />
		<div class="mt-6 flex justify-end">
			<button
				type="button"
				disabled={!saved}
				onclick={() => (modalOpen = false)}
				class="h-9 rounded-lg bg-primary-400 px-3.5 text-sm font-semibold text-primary-950 transition hover:bg-primary-300 disabled:cursor-not-allowed disabled:opacity-50"
			>
				Done
			</button>
		</div>
	{:else}
		<form onsubmit={submit} novalidate>
			{#if error}
				<p role="alert" class="mb-4 rounded-lg border border-primary-400/25 bg-primary-400/10 px-3 py-2 text-sm text-primary-200">{error}</p>
			{/if}
			<PasswordField id="tfa-password" label="Password" autocomplete="current-password" placeholder="" bind:value={password} />
			<div class="mt-6 flex justify-end gap-2">
				<button type="button" onclick={() => (modalOpen = false)} class="h-9 rounded-lg px-3.5 text-sm text-ink-400 transition hover:bg-ink-850 hover:text-ink-100">
					Cancel
				</button>
				<button
					type="submit"
					disabled={busy}
					class="h-9 rounded-lg bg-primary-400 px-3.5 text-sm font-semibold text-primary-950 transition hover:bg-primary-300 disabled:cursor-not-allowed disabled:opacity-60"
				>
					{busy ? 'Checking…' : action === 'codes' ? 'Generate codes' : 'Turn off'}
				</button>
			</div>
		</form>
	{/if}
</Modal>
