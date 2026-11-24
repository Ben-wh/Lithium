<script lang="ts">
	import { goto } from '$app/navigation';
	import { logout } from '$lib/api';
	import BrandLogo from '$lib/components/BrandLogo.svelte';
	import ThemeSwitcher from '$lib/components/ThemeSwitcher.svelte';
	import TwoFactorEnrol from '$lib/components/TwoFactorEnrol.svelte';

	let { data } = $props();

	async function signOut() {
		await logout();
		await goto('/login', { invalidateAll: true });
	}
</script>

<svelte:head>
	<title>Set up two-factor · Lithium</title>
</svelte:head>

<main class="relative flex min-h-[calc(100dvh-var(--banner-h,0px))] items-center justify-center overflow-hidden px-4 py-12">
	<div class="absolute top-4 right-4 z-10"><ThemeSwitcher /></div>
	<div
		aria-hidden="true"
		class="pointer-events-none absolute top-[-20%] left-1/2 h-130 w-180 -translate-x-1/2 rounded-full bg-primary-400/10 blur-[120px]"
	></div>

	<div class="relative w-full max-w-105">
		<div class="mb-8 flex flex-col items-center text-center">
			<BrandLogo size={44} maxWidth={240} />
			<h1 class="mt-6 text-2xl font-semibold tracking-tight text-ink-100">Set up two-factor authentication</h1>
			<p class="mt-1.5 text-sm text-ink-400">
				Your workspace requires a code from an authenticator app when you sign in. It takes about a minute.
			</p>
		</div>

		<div class="rounded-2xl border border-ink-800 bg-ink-900/70 p-6 shadow-2xl shadow-shade/40 backdrop-blur-sm">
			{#if data.user}
				<TwoFactorEnrol email={data.user.email} ondone={() => goto('/', { invalidateAll: true, replaceState: true })} />
			{/if}
		</div>

		<p class="mt-8 text-center text-sm">
			<button type="button" onclick={signOut} class="text-ink-400 transition hover:text-primary-300">Sign out</button>
		</p>
	</div>
</main>
