<script lang="ts">
	import { goto } from '$app/navigation';
	import { ApiError, changePassword, logout } from '$lib/api';
	import Logo from '$lib/components/Logo.svelte';
	import ThemeSwitcher from '$lib/components/ThemeSwitcher.svelte';
	import PasswordField, { passwordsMatch } from '$lib/components/PasswordField.svelte';

	let { data } = $props();

	let password = $state('');
	let confirmation = $state('');
	let loading = $state(false);
	let error = $state('');

	async function submit(e: SubmitEvent) {
		e.preventDefault();
		if (loading) return;
		error = '';
		if (password.length < 8) {
			error = 'Your new password must be at least 8 characters.';
			return;
		}
		if (!passwordsMatch(password, confirmation)) {
			error = "The passwords don't match.";
			return;
		}
		loading = true;
		try {
			await changePassword(password);
			await goto('/', { invalidateAll: true, replaceState: true });
		} catch (err) {
			error =
				err instanceof ApiError && err.status === 400
					? err.message.charAt(0).toUpperCase() + err.message.slice(1) + '.'
					: 'Something went wrong. Please try again.';
			loading = false;
		}
	}

	async function signOut() {
		await logout();
		await goto('/login', { invalidateAll: true });
	}
</script>

<svelte:head>
	<title>Choose a new password · Lithium</title>
</svelte:head>

<main class="relative flex min-h-[calc(100dvh-var(--banner-h,0px))] items-center justify-center overflow-hidden px-4 py-12">
	<div class="absolute top-4 right-4 z-10"><ThemeSwitcher /></div>

	<div
		aria-hidden="true"
		class="pointer-events-none absolute top-[-20%] left-1/2 h-130 w-180 -translate-x-1/2 rounded-full bg-primary-400/10 blur-[120px]"
	></div>

	<div class="relative w-full max-w-95">
		<div class="mb-8 flex flex-col items-center text-center">
			<Logo size={44} />
			<h1 class="mt-6 text-2xl font-semibold tracking-tight text-ink-100">Choose a new password</h1>
			<p class="mt-1.5 text-sm text-ink-400">
				Welcome{data.user ? `, ${data.user.name.split(' ')[0]}` : ''}. Your password was set by an
				administrator, so pick one only you know before continuing.
			</p>
		</div>

		<form
			onsubmit={submit}
			novalidate
			class="rounded-2xl border border-ink-800 bg-ink-900/70 p-6 shadow-2xl shadow-shade/40 backdrop-blur-sm"
		>
			{#if error}
				<p role="alert" class="mb-5 rounded-lg border border-primary-400/25 bg-primary-400/10 px-3 py-2.5 text-sm text-primary-200">
					{error}
				</p>
			{/if}

			<PasswordField id="new-password" label="New password" confirm bind:value={password} bind:confirmValue={confirmation} />

			<button
				type="submit"
				disabled={loading}
				class="mt-2 flex h-10 w-full items-center justify-center rounded-lg bg-primary-400 text-sm font-semibold text-primary-950 shadow-lg shadow-primary-400/10 transition hover:bg-primary-300 focus-visible:ring-3 focus-visible:ring-primary-400/35 focus-visible:ring-offset-2 focus-visible:ring-offset-ink-900 focus-visible:outline-none disabled:cursor-not-allowed disabled:opacity-70"
			>
				{loading ? 'Saving…' : 'Set password and continue'}
			</button>
		</form>

		<p class="mt-8 text-center text-sm">
			<button type="button" onclick={signOut} class="text-ink-400 transition hover:text-primary-300">
				Sign out
			</button>
		</p>
	</div>
</main>
