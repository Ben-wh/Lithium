<script lang="ts">
	import '../app.css';
	import { page } from '$app/state';
	import SensitiveBanner from '$lib/components/SensitiveBanner.svelte';

	let { data, children } = $props();

	// Signed-in pages show the banner with the navbar (see (app)/+layout.svelte);
	// the sign-in pages show it here.
	const showBanner = $derived(data.banner.sensitiveMode && !page.route.id?.startsWith('/(app)'));
</script>

{#if showBanner}
	<div class="sticky top-0 z-40">
		<SensitiveBanner message={data.banner.sensitiveMessage} />
	</div>
{/if}
<!-- Pages that fill the screen subtract the banner's height via --banner-h. -->
<div style:--banner-h={showBanner ? '2rem' : '0px'} class="contents">
	{@render children()}
</div>
