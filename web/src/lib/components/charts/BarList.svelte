<script lang="ts">
	import type { Snippet } from 'svelte';

	// Horizontal bars for one measure across categories: thin bars (≤ 24px)
	// with a rounded data-end, the value at the tip, and the share on hover.
	type Row = { key: string; label: string; value: number };
	let {
		rows,
		color = 'var(--viz-1)',
		lead,
		empty = 'Nothing to show'
	}: {
		rows: Row[];
		color?: string;
		/** Optional marker before each label (swatch, avatar, icon). */
		lead?: Snippet<[Row]>;
		empty?: string;
	} = $props();

	const max = $derived(Math.max(1, ...rows.map((r) => r.value)));
	const total = $derived(rows.reduce((n, r) => n + r.value, 0));
	let hovered = $state<string | null>(null);
</script>

{#if rows.length === 0 || total === 0}
	<p class="py-8 text-center text-sm text-ink-500">{empty}</p>
{:else}
	<ul class="space-y-2.5">
		{#each rows as r (r.key)}
			{@const share = total ? Math.round((r.value / total) * 100) : 0}
			<li
				class="grid grid-cols-[minmax(0,8rem)_1fr_auto] items-center gap-3 sm:grid-cols-[minmax(0,10rem)_1fr_auto]"
				onpointerenter={() => (hovered = r.key)}
				onpointerleave={() => (hovered = null)}
				title="{r.label}: {r.value} ({share}%)"
			>
				<span class="flex min-w-0 items-center gap-2 text-[13px] text-ink-200">
					{#if lead}{@render lead(r)}{/if}
					<span class="truncate">{r.label}</span>
				</span>
				<!-- The track is the full scale; the bar's length is the value's share of the largest. -->
				<span class="relative h-4">
					{#if r.value > 0}
						<span
							class="absolute inset-y-0 left-0 rounded-r transition-opacity"
							style:width="{Math.max(1.5, (r.value / max) * 100)}%"
							style:background={color}
							style:opacity={hovered && hovered !== r.key ? 0.45 : 1}
						></span>
					{/if}
				</span>
				<span class="min-w-6 text-right text-xs text-ink-200 tabular-nums">
					<span class="font-semibold text-ink-100">{r.value}</span>
					{#if hovered === r.key}<span class="text-ink-500"> · {share}%</span>{/if}
				</span>
			</li>
		{/each}
	</ul>
{/if}
