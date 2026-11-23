<script lang="ts">
	// One whole split into parts: a tall (28px) bar, 2px gaps between parts,
	// each part's share inside it when it fits, and a legend below.
	type Part = { key: string; label: string; value: number; color: string; ink?: 'light' | 'dark' };
	let { parts, ariaLabel }: { parts: Part[]; ariaLabel: string } = $props();

	const total = $derived(parts.reduce((n, p) => n + p.value, 0));
	const shown = $derived(parts.filter((p) => p.value > 0));
	const pct = (v: number) => (total ? (v / total) * 100 : 0);
</script>

{#if total === 0}
	<p class="py-8 text-center text-sm text-ink-500">No tickets yet</p>
{:else}
	<div role="img" aria-label={ariaLabel} class="flex h-7 w-full gap-0.5">
		{#each shown as p (p.key)}
			{@const share = Math.round(pct(p.value))}
			<div
				class="flex min-w-1 items-center overflow-hidden rounded px-2 text-[11px] font-semibold"
				style:flex-grow={p.value}
				style:flex-basis="0"
				style:background={p.color}
				style:color={p.ink === 'dark' ? '#0b0b0b' : '#ffffff'}
				title="{p.label}: {p.value} ({share}%)"
			>
				<!-- Only print the share when the part is wide enough for it. -->
				{#if pct(p.value) >= 9}{share}%{/if}
			</div>
		{/each}
	</div>
	<ul class="mt-3 flex flex-wrap gap-x-5 gap-y-1.5 text-xs text-ink-400">
		{#each parts as p (p.key)}
			<li class="flex items-center gap-1.5">
				<span class="size-2.5 rounded-sm" style:background={p.color}></span>
				{p.label}
				<span class="font-semibold text-ink-100 tabular-nums">{p.value}</span>
			</li>
		{/each}
	</ul>
{/if}
