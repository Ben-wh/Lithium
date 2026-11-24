<script lang="ts">
	import { ApiError, changePassword } from '$lib/api';
	import Icon from '$lib/components/Icon.svelte';
	import ProfilePicture from '$lib/components/ProfilePicture.svelte';
	import TwoFactorCard from '$lib/components/TwoFactorCard.svelte';
	import EmailNotificationsCard from '$lib/components/EmailNotificationsCard.svelte';
	import PasswordField, { passwordsMatch } from '$lib/components/PasswordField.svelte';

	let { data } = $props();

	const roleLabels = { admin: 'Admin', agent: 'Agent', user: 'User' } as const;

	let current = $state('');
	let password = $state('');
	let confirmation = $state('');
	let saving = $state(false);
	let error = $state('');
	let success = $state(false);

	async function submit(e: SubmitEvent) {
		e.preventDefault();
		if (saving) return;
		error = '';
		success = false;
		if (!current) {
			error = 'Enter your current password.';
			return;
		}
		if (password.length < 8) {
			error = 'Your new password must be at least 8 characters.';
			return;
		}
		if (!passwordsMatch(password, confirmation)) {
			error = "The new passwords don't match.";
			return;
		}
		saving = true;
		try {
			await changePassword(password, current);
			current = password = confirmation = '';
			success = true;
		} catch (err) {
			error =
				err instanceof ApiError && err.status === 400
					? err.message.charAt(0).toUpperCase() + err.message.slice(1) + '.'
					: 'Something went wrong. Please try again.';
		} finally {
			saving = false;
		}
	}
</script>

<svelte:head>
	<title>Your profile · Lithium</title>
</svelte:head>

<div class="mx-auto max-w-3xl px-4 py-10 sm:px-6">
	<h1 class="text-2xl font-semibold tracking-tight">Your profile</h1>

	{#if data.user}
		<section class="mt-8 rounded-xl border border-ink-800 bg-ink-900/60 p-6">
			<h2 class="text-sm font-semibold">Account</h2>
			<dl class="mt-4 grid gap-4 text-sm sm:grid-cols-3">
				<div>
					<dt class="text-ink-500">Name</dt>
					<dd class="mt-1 text-ink-100">{data.user.name}</dd>
				</div>
				<div class="min-w-0">
					<dt class="text-ink-500">Email</dt>
					<dd class="mt-1 truncate text-ink-100">{data.user.email}</dd>
				</div>
				<div>
					<dt class="text-ink-500">Role</dt>
					<dd class="mt-1 text-ink-100">{roleLabels[data.user.role]}</dd>
				</div>
			</dl>
		</section>
		<ProfilePicture user={data.user} customDisabled={data.settings.disableCustomAvatars} />
	{/if}

	<section class="mt-6 rounded-xl border border-ink-800 bg-ink-900/60 p-6">
		<h2 class="text-sm font-semibold">Change password</h2>
		<p class="mt-1 text-sm text-ink-400">You'll stay signed in here. Other devices will be signed out.</p>

		<form onsubmit={submit} novalidate class="mt-5 max-w-sm space-y-4">
			{#if error}
				<p role="alert" class="rounded-lg border border-danger-400/25 bg-danger-400/10 px-3 py-2 text-sm text-danger-200">
					{error}
				</p>
			{:else if success}
				<p role="status" class="flex items-center gap-2 rounded-lg border border-ink-700 bg-ink-900 px-3 py-2 text-sm text-ink-200">
					<Icon name="check" class="size-4 text-primary-300" /> Password updated.
				</p>
			{/if}

			<PasswordField
				id="current-password"
				label="Current password"
				autocomplete="current-password"
				placeholder=""
				bind:value={current}
			/>
			<PasswordField id="new-password" label="New password" confirm bind:value={password} bind:confirmValue={confirmation} />

			<button
				type="submit"
				disabled={saving}
				class="inline-flex h-9 items-center justify-center rounded-lg bg-primary-400 px-3.5 text-sm font-semibold text-primary-950 transition hover:bg-primary-300 disabled:cursor-not-allowed disabled:opacity-70"
			>
				{saving ? 'Saving…' : 'Update password'}
			</button>
		</form>
	</section>

	{#if data.user}
		<TwoFactorCard email={data.user.email} />
		<EmailNotificationsCard user={data.user} />
	{/if}
</div>
