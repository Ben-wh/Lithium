<script lang="ts">
	import '../app.css';
	import { page } from '$app/state';
	import SensitiveBanner from '$lib/components/SensitiveBanner.svelte';
	import LockdownFrame from '$lib/components/LockdownFrame.svelte';
	import { applyAccent } from '$lib/accents';

	let { data, children } = $props();

	// The admin's accent colour, on every page including sign-in.
	$effect(() => applyAccent(data.branding.accent));

	// Signed-in pages show the banner with the navbar (see (app)/+layout.svelte);
	// the sign-in pages show it here.
	const showBanner = $derived(data.banner.sensitiveMode && !page.route.id?.startsWith('/(app)'));
</script>

{#if data.banner.lockdown}
	<LockdownFrame isAdmin={data.user?.role === 'admin'} />
{/if}
<!-- In lockdown, everything is inset inside the red frame (--lockdown-inset);
     sticky bars stick just below it. -->
<div
	style:--lockdown-inset={data.banner.lockdown ? '1.5rem' : '0px'}
	style:padding={data.banner.lockdown ? 'var(--lockdown-inset)' : undefined}
	class={data.banner.lockdown ? 'block' : 'contents'}
>
	{#if showBanner}
		<div class="sticky top-(--lockdown-inset) z-40">
			<SensitiveBanner message={data.banner.sensitiveMessage} />
		</div>
	{/if}
	<!-- Pages that fill the screen subtract the banner's and frame's height via --banner-h. -->
	<div
		style:--banner-h={`calc(${showBanner ? '2rem' : '0px'} + 2 * var(--lockdown-inset))`}
		class="contents"
	>
		{@render children()}
	</div>
</div>
