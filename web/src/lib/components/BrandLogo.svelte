<script lang="ts">
	import { page } from '$app/state';
	import Logo from './Logo.svelte';

	// The workspace's logo: the company logo an admin uploaded (Lithium
	// settings → Branding), or the Lithium tile. Uploaded logos keep their
	// shape: `size` is the height, and wide logos get up to `maxWidth`.
	let { size = 36, maxWidth = size * 5 }: { size?: number; maxWidth?: number } = $props();

	const logoUrl = $derived(page.data.branding?.logoUrl ?? '');
	let failed = $state(false);
	$effect(() => {
		logoUrl; // try again whenever the logo changes
		failed = false;
	});
</script>

{#if logoUrl && !failed}
	<img
		src={logoUrl}
		alt=""
		style:height="{size}px"
		style:max-width="{maxWidth}px"
		class="w-auto shrink-0 object-contain"
		onerror={() => (failed = true)}
	/>
{:else}
	<Logo {size} />
{/if}
