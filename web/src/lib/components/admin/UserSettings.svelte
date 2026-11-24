<script lang="ts">
	import { onMount } from 'svelte';
	import { invalidate, invalidateAll } from '$app/navigation';
	import { adminSettings, ApiError, getSettings, sendTestEmail, type Settings } from '$lib/api';
	import Icon from '$lib/components/Icon.svelte';
	import Toggle from '$lib/components/Toggle.svelte';
	import Modal from '$lib/components/Modal.svelte';

	let settings = $state<Settings | null>(null);
	let loadFailed = $state(false);
	let saving = $state(false);
	let status = $state<'saved' | 'error' | null>(null);
	let statusTimer: ReturnType<typeof setTimeout>;

	onMount(async () => {
		try {
			settings = await getSettings();
		} catch {
			loadFailed = true;
		}
	});

	// --- Test email: sent straight away, so SMTP mistakes show up here ---
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

	// Turning mandatory 2FA off weakens everyone's sign-in, so it asks first.
	// The switch stays on until the admin confirms.
	let confirmDisable2FA = $state(false);

	function onTwoFactorChange(on: boolean) {
		if (on) {
			save({ requireTwoFactor: true }, () => settings && (settings.requireTwoFactor = false));
			return;
		}
		if (settings) settings.requireTwoFactor = true;
		confirmDisable2FA = true;
	}

	function disableTwoFactor() {
		confirmDisable2FA = false;
		if (settings) settings.requireTwoFactor = false;
		save({ requireTwoFactor: false }, () => settings && (settings.requireTwoFactor = true));
	}

	// Switches save as soon as they're changed; on failure they flip back.
	async function save(patch: Partial<Settings>, revert: () => void) {
		saving = true;
		clearTimeout(statusTimer);
		try {
			settings = await adminSettings.update(patch);
			status = 'saved';
			// Turning on mandatory 2FA sends an admin without it straight to setup.
			if (patch.requireTwoFactor) await invalidateAll();
			else await invalidate('app:settings');
		} catch {
			revert();
			status = 'error';
		} finally {
			saving = false;
			statusTimer = setTimeout(() => (status = null), status === 'saved' ? 2000 : 5000);
		}
	}
</script>

<div class="mt-5 rounded-xl border border-ink-800 bg-ink-900/60">
	<div class="flex items-center justify-between gap-3 border-b border-ink-800 px-5 py-4">
		<h3 class="text-sm font-semibold">Sign-in, profiles and access</h3>
		<span class="text-xs" aria-live="polite">
			{#if saving}
				<span class="text-ink-500">Saving…</span>
			{:else if status === 'saved'}
				<span class="flex items-center gap-1 text-ink-400"><Icon name="check" class="size-3.5 text-primary-300" /> Saved</span>
			{:else if status === 'error'}
				<span class="text-danger-300">Couldn't save. Try again.</span>
			{/if}
		</span>
	</div>
	<div class="px-5 py-4">
		{#if loadFailed}
			<p class="text-sm text-danger-200">Couldn't load settings.</p>
		{:else if !settings}
			<p class="text-sm text-ink-500">Loading…</p>
		{:else}
			<Toggle
				bind:checked={settings.requireTwoFactor}
				disabled={saving}
				onchange={onTwoFactorChange}
			>
				Mandatory two-factor authentication
				<span class="ml-1.5 text-xs font-semibold text-red-500">Highly recommended</span>
				{#snippet description()}
					Everyone must set up an authenticator app and enter a code when they sign in. Anyone without it,
					including you, is asked to set it up before they can continue. When off, people can still turn it on
					from their profile.
				{/snippet}
			</Toggle>
			<div class="my-4 h-px bg-ink-800"></div>
			<Toggle
				bind:checked={settings.emailNotifications}
				disabled={saving || !settings.emailConfigured}
				onchange={(on) => save({ emailNotifications: on }, () => settings && (settings.emailNotifications = !on))}
			>
				Email notifications
				{#snippet description()}
					{#if settings?.emailConfigured}
						Email people about replies, assignments, mentions and changes on their tickets, as well as notifying
						them in the app. Everyone can turn their own emails off from their profile. Emails never include reply
						text, and leave out ticket subjects while sensitive mode is on.
					{:else}
						Email isn't set up on this server. Set <code class="font-mono text-[12px]">LITHIUM_SMTP_HOST</code> and
						the other <code class="font-mono text-[12px]">LITHIUM_SMTP_*</code> settings, then restart Lithium.
					{/if}
				{/snippet}
			</Toggle>
			{#if settings.emailConfigured}
				<div class="mt-3 flex flex-wrap items-center gap-3">
					<button
						type="button"
						onclick={testEmail}
						disabled={testState === 'sending'}
						class="inline-flex h-8 items-center gap-1.5 rounded-lg border border-ink-700 px-3 text-[13px] text-ink-200 transition hover:border-ink-500 hover:text-ink-100 disabled:cursor-not-allowed disabled:opacity-60"
					>
						<Icon name="bell" class="size-3.5" />
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
			{/if}
			<div class="my-4 h-px bg-ink-800"></div>
			<Toggle
				bind:checked={settings.disableCustomAvatars}
				disabled={saving}
				onchange={(on) => save({ disableCustomAvatars: on }, () => settings && (settings.disableCustomAvatars = !on))}
			>
				Disable custom profile pictures
				{#snippet description()}
					Users can't upload their own picture, and any already uploaded are hidden. The 10 built-in avatars
					stay available. Turning this off again brings uploaded pictures back.
				{/snippet}
			</Toggle>
			<div class="my-4 h-px bg-ink-800"></div>
			<Toggle
				bind:checked={settings.allowUserMentions}
				disabled={saving}
				onchange={(on) => save({ allowUserMentions: on }, () => settings && (settings.allowUserMentions = !on))}
			>
				Allow users to @mention
				{#snippet description()}
					Users can type @ in replies to mention and notify people on their ticket. Admins and agents can
					always mention.
				{/snippet}
			</Toggle>
			<div class="my-4 h-px bg-ink-800"></div>
			<Toggle
				bind:checked={settings.agentGroups}
				disabled={saving}
				onchange={(on) => save({ agentGroups: on }, () => settings && (settings.agentGroups = !on))}
			>
				Agent groups
				{#snippet description()}
					Agents only see tickets in their groups' categories, plus uncategorised tickets and ones assigned to them.
					Admins always see everything. Set up groups in <a href="?tab=groups" class="text-primary-300 hover:text-primary-200">Admin → Groups</a>.
				{/snippet}
			</Toggle>
		{/if}
	</div>
</div>

<Modal
	bind:open={confirmDisable2FA}
	title="Turn off mandatory two-factor?"
	description="People will be able to sign in with just a password unless they've set up two-factor themselves."
>
	<div class="flex items-start gap-3 rounded-lg border border-danger-400/25 bg-danger-400/10 px-3 py-2.5 text-sm text-danger-200">
		<Icon name="shieldAlert" class="mt-0.5 size-4 shrink-0" />
		<p>
			Accounts without an authenticator app become easier to break into with a stolen or guessed password.
			Anyone who already uses two-factor keeps it.
		</p>
	</div>
	<div class="mt-5 flex justify-end gap-2">
		<button
			type="button"
			onclick={() => (confirmDisable2FA = false)}
			class="inline-flex h-9 items-center rounded-lg border border-ink-700 px-3.5 text-sm font-medium text-ink-200 transition hover:border-ink-500 hover:text-ink-100"
		>
			Keep it on
		</button>
		<button
			type="button"
			onclick={disableTwoFactor}
			class="inline-flex h-9 items-center rounded-lg bg-danger-600 px-3.5 text-sm font-semibold text-white transition hover:bg-danger-500"
		>
			Turn off
		</button>
	</div>
</Modal>
