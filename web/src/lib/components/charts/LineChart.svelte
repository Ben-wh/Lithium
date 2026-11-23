<script lang="ts">
	import { niceTicks, compact } from './scale';

	// A time-series line chart: one y-axis, 2px lines, recessive grid,
	// a legend, end labels when they don't collide, and a crosshair tooltip.
	type Series = { name: string; color: string; values: number[] };
	let {
		labels,
		series,
		height = 240,
		formatLabel = (l: string) => l,
		ariaLabel
	}: {
		/** One per point, e.g. dates. */
		labels: string[];
		series: Series[];
		height?: number;
		/** Short x-axis / tooltip label. */
		formatLabel?: (l: string) => string;
		ariaLabel: string;
	} = $props();

	let width = $state(600);
	const M = { top: 12, right: 64, bottom: 26, left: 36 };
	const innerW = $derived(Math.max(10, width - M.left - M.right));
	const innerH = $derived(height - M.top - M.bottom);

	const max = $derived(Math.max(0, ...series.flatMap((s) => s.values)));
	const ticks = $derived(niceTicks(max));
	const top = $derived(ticks[ticks.length - 1] || 1);
	const n = $derived(labels.length);
	const x = (i: number) => M.left + (n <= 1 ? innerW / 2 : (i / (n - 1)) * innerW);
	const y = (v: number) => M.top + innerH - (v / top) * innerH;

	const paths = $derived(
		series.map((s) => ({ ...s, d: s.values.map((v, i) => `${i ? 'L' : 'M'}${x(i).toFixed(1)} ${y(v).toFixed(1)}`).join(' ') }))
	);

	// About six evenly spaced x labels.
	const xTicks = $derived.by(() => {
		const every = Math.max(1, Math.ceil(n / 6));
		return labels.map((l, i) => ({ i, l })).filter(({ i }) => i % every === 0 || i === n - 1);
	});

	// End labels only when they won't collide.
	const endLabels = $derived.by(() => {
		const ends = series.map((s) => ({ name: s.name, value: s.values[n - 1] ?? 0, y: y(s.values[n - 1] ?? 0) }));
		const collide = ends.some((a, i) => ends.some((b, j) => i < j && Math.abs(a.y - b.y) < 16));
		return collide ? [] : ends;
	});

	// Hover / keyboard crosshair.
	let active = $state<number | null>(null);
	function onMove(e: PointerEvent) {
		const rect = (e.currentTarget as SVGElement).getBoundingClientRect();
		const px = e.clientX - rect.left;
		active = n <= 1 ? 0 : Math.max(0, Math.min(n - 1, Math.round(((px - M.left) / innerW) * (n - 1))));
	}
	function onKey(e: KeyboardEvent) {
		if (e.key === 'ArrowRight') active = Math.min(n - 1, (active ?? -1) + 1);
		else if (e.key === 'ArrowLeft') active = Math.max(0, (active ?? n) - 1);
		else if (e.key === 'Escape') active = null;
		else return;
		e.preventDefault();
	}
	const tipLeft = $derived(active === null ? 0 : x(active));
</script>

<div class="relative" bind:clientWidth={width}>
	<!-- Legend: always shown for two or more series. -->
	{#if series.length > 1}
		<ul class="mb-2 flex flex-wrap gap-x-4 gap-y-1 text-xs text-ink-400">
			{#each series as s (s.name)}
				<li class="flex items-center gap-1.5">
					<span class="h-0.5 w-3.5 rounded-full" style:background={s.color}></span>{s.name}
				</li>
			{/each}
		</ul>
	{/if}

	<!-- Focusable so keyboard users can step through days with the arrow keys. -->
	<!-- svelte-ignore a11y_no_noninteractive_tabindex, a11y_no_noninteractive_element_interactions -->
	<svg
		{width}
		{height}
		role="img"
		aria-label={ariaLabel}
		tabindex="0"
		onpointermove={onMove}
		onpointerleave={() => (active = null)}
		onkeydown={onKey}
		onblur={() => (active = null)}
		class="block touch-none overflow-visible outline-none focus-visible:rounded focus-visible:ring-2 focus-visible:ring-primary-400/50"
	>
		<!-- Grid and y-axis -->
		{#each ticks as t (t)}
			<line x1={M.left} x2={M.left + innerW} y1={y(t)} y2={y(t)} style:stroke={t === 0 ? 'var(--viz-axis)' : 'var(--viz-grid)'} stroke-width="1" />
			<text x={M.left - 8} y={y(t)} dy="0.32em" text-anchor="end" class="fill-ink-500 text-[11px] tabular-nums">{compact(t)}</text>
		{/each}
		{#each xTicks as { i, l } (i)}
			<text x={x(i)} y={height - 6} text-anchor={i === 0 ? 'start' : i === n - 1 ? 'end' : 'middle'} class="fill-ink-500 text-[11px]">{formatLabel(l)}</text>
		{/each}

		<!-- Crosshair -->
		{#if active !== null}
			<line x1={x(active)} x2={x(active)} y1={M.top} y2={M.top + innerH} style:stroke="var(--viz-axis)" stroke-width="1" />
		{/if}

		<!-- Lines -->
		{#each paths as p (p.name)}
			<path d={p.d} fill="none" style:stroke={p.color} stroke-width="2" stroke-linejoin="round" stroke-linecap="round" />
		{/each}

		<!-- End dots and labels; or hover dots -->
		{#each series as s (s.name)}
			{@const i = active ?? n - 1}
			<circle cx={x(i)} cy={y(s.values[i] ?? 0)} r="4" style:fill={s.color} style:stroke="var(--color-ink-900)" stroke-width="2" />
		{/each}
		{#if active === null}
			{#each endLabels as e (e.name)}
				<text x={x(n - 1) + 10} y={e.y} dy="0.32em" class="fill-ink-200 text-[11px]">
					<tspan class="font-semibold">{e.value}</tspan> <tspan class="fill-ink-500">{e.name}</tspan>
				</text>
			{/each}
		{/if}
	</svg>

	<!-- Tooltip -->
	{#if active !== null}
		<div
			role="status"
			class="pointer-events-none absolute top-6 z-10 min-w-36 -translate-x-1/2 rounded-lg border border-ink-700 bg-ink-900 px-3 py-2 text-xs shadow-lg shadow-shade/30"
			style:left="{Math.max(80, Math.min(width - 80, tipLeft))}px"
		>
			<p class="mb-1 font-medium text-ink-100">{formatLabel(labels[active])}</p>
			{#each series as s (s.name)}
				<p class="flex items-center gap-2 text-ink-400">
					<span class="size-2 rounded-full" style:background={s.color}></span>
					<span class="flex-1">{s.name}</span>
					<span class="font-semibold text-ink-100 tabular-nums">{s.values[active]}</span>
				</p>
			{/each}
		</div>
	{/if}
</div>
