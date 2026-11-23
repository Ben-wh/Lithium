<script lang="ts">
	import { goto } from '$app/navigation';
	import { page } from '$app/state';
	import { ApiError, login, logout, twoFactor } from '$lib/api';
	import CodeInput from '$lib/components/CodeInput.svelte';
	import Logo from '$lib/components/Logo.svelte';
	import ThemeSwitcher from '$lib/components/ThemeSwitcher.svelte';

	let { data } = $props();

	// Accounts with two-factor get a second step after the password. A reload
	// mid-way lands back on it (the session is waiting for a code).
	// svelte-ignore state_referenced_locally
	let step = $state<'password' | 'code'>(data.user?.twoFactor === 'verify' ? 'code' : 'password');
	let code = $state('');
	let useRecovery = $state(false);

	let email = $state('');
	let password = $state('');
	let remember = $state(true);
	let showPassword = $state(false);
	let loading = $state(false);
	let error = $state('');

	// Only follow same-site relative paths, never absolute URLs.
	function nextPath() {
		const next = page.url.searchParams.get('next');
		return next && next.startsWith('/') && !next.startsWith('//') ? next : '/';
	}

	async function submit(e: SubmitEvent) {
		e.preventDefault();
		if (loading) return;
		error = '';
		loading = true;
		try {
			const user = await login(email, password, remember);
			if (user.twoFactor === 'verify') {
				step = 'code';
				password = '';
				loading = false;
				return;
			}
			await goto(nextPath(), { invalidateAll: true, replaceState: true });
		} catch (err) {
			if (err instanceof ApiError && err.status === 401) error = 'Incorrect email or password.';
			else if (err instanceof ApiError && err.status === 429)
				error = 'Too many attempts. Please wait a minute and try again.';
			else error = 'Something went wrong. Please try again.';
			loading = false;
		}
	}

	async function verify(e?: SubmitEvent) {
		e?.preventDefault();
		if (loading) return;
		if (!code.trim()) {
			error = useRecovery ? 'Enter one of your recovery codes.' : 'Enter the 6-digit code from your app.';
			return;
		}
		error = '';
		loading = true;
		try {
			await twoFactor.verify(code);
			await goto(nextPath(), { invalidateAll: true, replaceState: true });
		} catch (err) {
			code = '';
			loading = false;
			if (err instanceof ApiError && err.status === 400) {
				error = useRecovery ? "That recovery code isn't valid or has been used." : "That code isn't right. Try the current one.";
			} else if (err instanceof ApiError && err.status === 401) {
				error = 'Too many incorrect codes. Please sign in again.';
				step = 'password';
			} else if (err instanceof ApiError && err.status === 429) {
				error = 'Too many attempts. Please wait a minute and try again.';
			} else {
				error = 'Something went wrong. Please try again.';
			}
		}
	}

	async function startOver() {
		await logout().catch(() => {});
		step = 'password';
		code = '';
		error = '';
		useRecovery = false;
	}
</script>

<svelte:head>
	<title>Sign in · Lithium</title>
</svelte:head>

<main class="relative flex min-h-[calc(100dvh-var(--banner-h,0px))] items-center justify-center overflow-hidden px-4 py-12">
	<div class="absolute top-4 right-4 z-10"><ThemeSwitcher /></div>

	<!-- soft pastel glow -->
	<div
		aria-hidden="true"
		class="pointer-events-none absolute top-[-20%] left-1/2 h-130 w-180 -translate-x-1/2 rounded-full bg-primary-400/10 blur-[120px]"
	></div>
	<div
		aria-hidden="true"
		class="pointer-events-none absolute inset-0 bg-[radial-gradient(circle_at_center,var(--dot-grid)_1px,transparent_1px)] mask-[radial-gradient(ellipse_at_center,black_30%,transparent_75%)] bg-size-[24px_24px]"
	></div>

	<div class="relative w-full max-w-95">
		<div class="mb-8 flex flex-col items-center text-center">
			<Logo size={44} />
			{#if step === 'code'}
				<h1 class="mt-6 text-2xl font-semibold tracking-tight text-ink-100">Two-factor authentication</h1>
				<p class="mt-1.5 text-sm text-ink-400">
					{useRecovery ? 'Enter one of the recovery codes you saved' : 'Enter the code from your authenticator app'}
				</p>
			{:else}
				<h1 class="mt-6 text-2xl font-semibold tracking-tight text-ink-100">Welcome back</h1>
				<p class="mt-1.5 text-sm text-ink-400">Sign in to your Lithium workspace</p>
			{/if}
		</div>

		{#if step === 'code'}
			<form
				onsubmit={verify}
				class="rounded-2xl border border-ink-800 bg-ink-900/70 p-6 shadow-2xl shadow-shade/40 backdrop-blur-sm"
				novalidate
			>
				{#if error}
					<p role="alert" class="mb-5 rounded-lg border border-primary-400/25 bg-primary-400/10 px-3 py-2.5 text-sm text-primary-200">
						{error}
					</p>
				{/if}
				<label for="code" class="sr-only">{useRecovery ? 'Recovery code' : 'Authentication code'}</label>
				{#key useRecovery}
					<CodeInput bind:value={code} allowRecovery={useRecovery} disabled={loading} autofocus oncomplete={() => verify()} />
				{/key}
				<button
					type="submit"
					disabled={loading}
					class="mt-5 flex h-10 w-full items-center justify-center rounded-lg bg-primary-400 text-sm font-semibold text-primary-950 shadow-lg shadow-primary-400/10 transition hover:bg-primary-300 focus-visible:ring-3 focus-visible:ring-primary-400/35 focus-visible:ring-offset-2 focus-visible:ring-offset-ink-900 focus-visible:outline-none disabled:cursor-not-allowed disabled:opacity-70"
				>
					{loading ? 'Verifying…' : 'Verify'}
				</button>
				<div class="mt-4 flex items-center justify-between text-xs">
					<button
						type="button"
						onclick={() => {
							useRecovery = !useRecovery;
							code = '';
							error = '';
						}}
						class="text-ink-400 transition hover:text-primary-300"
					>
						{useRecovery ? 'Use authenticator app' : 'Use a recovery code'}
					</button>
					<button type="button" onclick={startOver} class="text-ink-400 transition hover:text-primary-300">
						Back to sign in
					</button>
				</div>
			</form>
			<p class="mt-4 text-center text-xs text-ink-500">Lost your phone and codes? Ask an administrator to reset your two-factor.</p>
		{:else}
			<form
				onsubmit={submit}
				class="rounded-2xl border border-ink-800 bg-ink-900/70 p-6 shadow-2xl shadow-shade/40 backdrop-blur-sm"
				novalidate
			>
				{#if error}
					<div
						role="alert"
						class="mb-5 flex items-start gap-2.5 rounded-lg border border-primary-400/25 bg-primary-400/10 px-3 py-2.5 text-sm text-primary-200"
					>
						<svg class="mt-0.5 size-4 shrink-0" viewBox="0 0 20 20" fill="currentColor" aria-hidden="true">
							<path
								fill-rule="evenodd"
								d="M18 10a8 8 0 1 1-16 0 8 8 0 0 1 16 0Zm-8-5a.75.75 0 0 1 .75.75v4.5a.75.75 0 0 1-1.5 0v-4.5A.75.75 0 0 1 10 5Zm0 10a1 1 0 1 0 0-2 1 1 0 0 0 0 2Z"
								clip-rule="evenodd"
							/>
						</svg>
						<span>{error}</span>
					</div>
				{/if}
	
				<div class="space-y-4">
					<div>
						<label for="email" class="mb-1.5 block text-[13px] font-medium text-ink-200">Email</label>
						<input
							id="email"
							type="email"
							autocomplete="email"
							required
							bind:value={email}
							placeholder="you@company.com"
							class="block h-10 w-full rounded-lg border border-ink-700 bg-ink-950/60 px-3 text-sm text-ink-100 placeholder:text-ink-500 transition outline-none focus:border-primary-400/70 focus:ring-3 focus:ring-primary-400/15"
						/>
					</div>
	
					<div>
						<div class="mb-1.5 flex items-center justify-between">
							<label for="password" class="text-[13px] font-medium text-ink-200">Password</label>
							<a
								href="/forgot-password"
								class="text-xs text-ink-400 transition hover:text-primary-300 focus-visible:text-primary-300 focus-visible:outline-none"
							>
								Forgot password?
							</a>
						</div>
						<div class="relative">
							<input
								id="password"
								type={showPassword ? 'text' : 'password'}
								autocomplete="current-password"
								required
								bind:value={password}
								placeholder="••••••••"
								class="block h-10 w-full rounded-lg border border-ink-700 bg-ink-950/60 pr-10 pl-3 text-sm text-ink-100 placeholder:text-ink-500 transition outline-none focus:border-primary-400/70 focus:ring-3 focus:ring-primary-400/15"
							/>
							<button
								type="button"
								onclick={() => (showPassword = !showPassword)}
								class="absolute inset-y-0 right-0 flex w-10 items-center justify-center rounded-r-lg text-ink-500 transition hover:text-ink-200 focus-visible:text-primary-300 focus-visible:outline-none"
								aria-label={showPassword ? 'Hide password' : 'Show password'}
							>
								{#if showPassword}
									<svg class="size-4" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="1.8" stroke-linecap="round" stroke-linejoin="round" aria-hidden="true">
										<path d="M3 3l18 18M10.6 10.6a2 2 0 0 0 2.8 2.8M9.9 5.1A9.8 9.8 0 0 1 12 5c5 0 9 4.5 10 7-.4 1-1.2 2.3-2.4 3.5M6.6 6.6C4.6 7.9 3.1 9.9 2 12c1 2.5 5 7 10 7 1.8 0 3.4-.6 4.8-1.4" />
									</svg>
								{:else}
									<svg class="size-4" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="1.8" stroke-linecap="round" stroke-linejoin="round" aria-hidden="true">
										<path d="M2 12c1-2.5 5-7 10-7s9 4.5 10 7c-1 2.5-5 7-10 7S3 14.5 2 12Z" />
										<circle cx="12" cy="12" r="3" />
									</svg>
								{/if}
							</button>
						</div>
					</div>
	
					<label class="flex w-fit cursor-pointer items-center gap-2.5 text-sm text-ink-400 select-none">
						<span class="relative flex size-4">
							<input
								type="checkbox"
								bind:checked={remember}
								class="peer size-4 cursor-pointer appearance-none rounded border border-ink-700 bg-ink-950 transition checked:border-primary-400 checked:bg-primary-400 focus-visible:ring-3 focus-visible:ring-primary-400/25 focus-visible:outline-none"
							/>
							<svg
								class="pointer-events-none absolute inset-0 hidden size-4 text-primary-950 peer-checked:block"
								viewBox="0 0 16 16"
								fill="none"
								stroke="currentColor"
								stroke-width="2"
								stroke-linecap="round"
								stroke-linejoin="round"
								aria-hidden="true"
							>
								<path d="M4 8.5l2.5 2.5L12 5.5" />
							</svg>
						</span>
						Keep me signed in
					</label>
				</div>
	
				<button
					type="submit"
					disabled={loading}
					class="mt-6 flex h-10 w-full items-center justify-center gap-2 rounded-lg bg-primary-400 text-sm font-semibold text-primary-950 shadow-lg shadow-primary-400/10 transition hover:bg-primary-300 focus-visible:ring-3 focus-visible:ring-primary-400/35 focus-visible:ring-offset-2 focus-visible:ring-offset-ink-900 focus-visible:outline-none active:scale-[0.99] disabled:cursor-not-allowed disabled:opacity-70"
				>
					{#if loading}
						<svg class="size-4 animate-spin" viewBox="0 0 24 24" fill="none" aria-hidden="true">
							<circle cx="12" cy="12" r="9" stroke="currentColor" stroke-opacity="0.25" stroke-width="3" />
							<path d="M21 12a9 9 0 0 0-9-9" stroke="currentColor" stroke-width="3" stroke-linecap="round" />
						</svg>
						Signing in…
					{:else}
						Sign in
					{/if}
				</button>
			</form>
		{/if}

		<p class="mt-8 text-center text-xs text-ink-500">
			Lithium · self-hosted ticketing
		</p>
	</div>
</main>
