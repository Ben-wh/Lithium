<script lang="ts">
	import { ApiError, requestPasswordReset } from '$lib/api';
	import Icon from '$lib/components/Icon.svelte';
	import Logo from '$lib/components/Logo.svelte';
	import ThemeSwitcher from '$lib/components/ThemeSwitcher.svelte';

	let email = $state('');
	let loading = $state(false);
	let error = $state('');
	let sent = $state(false);

	async function submit(e: SubmitEvent) {
		e.preventDefault();
		if (loading) return;
		error = '';
		loading = true;
		try {
			await requestPasswordReset(email);
			sent = true;
		} catch (err) {
			if (err instanceof ApiError && err.status === 429)
				error = 'Too many requests. Please wait a minute and try again.';
			else if (err instanceof ApiError && err.status === 400) error = 'Enter a valid email address.';
			else error = 'Something went wrong. Please try again.';
		} finally {
			loading = false;
		}
	}
</script>

<svelte:head>
	<title>Forgot password · Lithium</title>
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
			<h1 class="mt-6 text-2xl font-semibold tracking-tight text-ink-100">
				{sent ? 'Request sent' : 'Forgot your password?'}
			</h1>
			<p class="mt-1.5 text-sm text-ink-400">
				{sent
					? 'An administrator will be in touch.'
					: "Enter your email and we'll ask an administrator to reset it."}
			</p>
		</div>

		<div class="rounded-2xl border border-ink-800 bg-ink-900/70 p-6 shadow-2xl shadow-shade/40 backdrop-blur-sm">
			{#if sent}
				<div class="flex flex-col items-center text-center">
					<span class="flex size-11 items-center justify-center rounded-xl bg-primary-400/10 text-primary-400">
						<Icon name="check" class="size-5" />
					</span>
					<p class="mt-4 text-sm text-ink-200">
						If an account exists for <span class="font-medium text-ink-100">{email}</span>, an
						administrator has been asked to reset its password. They'll give you a new one.
					</p>
				</div>
			{:else}
				<form onsubmit={submit} novalidate>
					{#if error}
						<p role="alert" class="mb-5 rounded-lg border border-primary-400/25 bg-primary-400/10 px-3 py-2.5 text-sm text-primary-200">
							{error}
						</p>
					{/if}
					<label for="email" class="mb-1.5 block text-[13px] font-medium text-ink-200">Email</label>
					<input
						id="email"
						type="email"
						autocomplete="email"
						required
						bind:value={email}
						placeholder="you@company.com"
						class="field"
					/>
					<button
						type="submit"
						disabled={loading}
						class="mt-6 flex h-10 w-full items-center justify-center rounded-lg bg-primary-400 text-sm font-semibold text-primary-950 shadow-lg shadow-primary-400/10 transition hover:bg-primary-300 focus-visible:ring-3 focus-visible:ring-primary-400/35 focus-visible:ring-offset-2 focus-visible:ring-offset-ink-900 focus-visible:outline-none disabled:cursor-not-allowed disabled:opacity-70"
					>
						{loading ? 'Sending…' : 'Request reset'}
					</button>
				</form>
			{/if}
		</div>

		<p class="mt-8 text-center text-sm">
			<a href="/login" class="text-ink-400 transition hover:text-primary-300">← Back to sign in</a>
		</p>
	</div>
</main>
