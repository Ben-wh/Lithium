<script lang="ts">
	// Lithium as a periodic table tile, in the workspace accent colour. The
	// atomic number is hidden at small sizes, where it would just be a speck.
	let { size = 36 }: { size?: number } = $props();
	const detailed = $derived(size >= 28);
</script>

<svg width={size} height={size} viewBox="0 0 32 32" aria-hidden="true" class="shrink-0">
	<rect width="32" height="32" rx="5" class="fill-primary-400" />
	{#if detailed}
		<text x="4" y="9.5" font-size="7" font-weight="600" class="fill-primary-950" fill-opacity="0.65" font-family="var(--font-sans)">3</text>
	{/if}
	<text
		x="16"
		y={detailed ? 24.5 : 22.5}
		text-anchor="middle"
		font-size={detailed ? 15 : 17}
		font-weight="700"
		letter-spacing="-0.4"
		class="fill-primary-950"
		font-family="var(--font-sans)"
	>
		Li
	</text>
</svg>
