<script lang="ts">
	import { onMount } from 'svelte';
	import { invalidate, invalidateAll } from '$app/navigation';
	import { adminSettings, getSettings, type Settings } from '$lib/api';
	import Icon from '$lib/components/Icon.svelte';
	import Toggle from '$lib/components/Toggle.svelte';

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
				<span class="text-primary-300">Couldn't save. Try again.</span>
			{/if}
		</span>
	</div>
	<div class="px-5 py-4">
		{#if loadFailed}
			<p class="text-sm text-primary-200">Couldn't load settings.</p>
		{:else if !settings}
			<p class="text-sm text-ink-500">Loading…</p>
		{:else}
			<Toggle
				bind:checked={settings.requireTwoFactor}
				disabled={saving}
				onchange={(on) => save({ requireTwoFactor: on }, () => settings && (settings.requireTwoFactor = !on))}
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
