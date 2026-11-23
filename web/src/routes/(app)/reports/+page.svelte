<script lang="ts">
	import { page } from '$app/state';
	import { reports, type NamedCount, type Priority, type TicketStatus } from '$lib/api';
	import Avatar from '$lib/components/Avatar.svelte';
	import BarList from '$lib/components/charts/BarList.svelte';
	import LineChart from '$lib/components/charts/LineChart.svelte';
	import SegmentedBar from '$lib/components/charts/SegmentedBar.svelte';
	import Dropdown, { type DropdownOption } from '$lib/components/Dropdown.svelte';
	import Icon from '$lib/components/Icon.svelte';
	import PriorityIcon from '$lib/components/PriorityIcon.svelte';
	import { ACTIVE_STATUSES, priorityLabel, statusLabels } from '$lib/tickets';

	let { data } = $props();
	const r = $derived(data.report);

	const periodHref = (d: number) => {
		const url = new URL(page.url);
		url.searchParams.set('days', String(d));
		return url.pathname + url.search;
	};

	const dayFmt = new Intl.DateTimeFormat(undefined, { day: 'numeric', month: 'short' });
	const formatDay = (iso: string) => dayFmt.format(new Date(iso + 'T00:00:00'));

	function duration(hours: number | null) {
		if (hours === null) return '—';
		if (hours < 1) return `${Math.max(1, Math.round(hours * 60))}m`;
		if (hours < 48) return `${hours.toFixed(1).replace(/\.0$/, '')}h`;
		return `${(hours / 24).toFixed(1).replace(/\.0$/, '')}d`;
	}

	const tiles = $derived([
		{ label: 'Raised', value: r.totals.raised.toLocaleString(), hint: `in the last ${r.days} days` },
		{ label: 'Resolved', value: r.totals.resolved.toLocaleString(), hint: `in the last ${r.days} days` },
		{ label: 'Open now', value: r.totals.open.toLocaleString(), hint: 'not yet resolved or closed' },
		{ label: 'First response', value: duration(r.totals.medianFirstResponseHours), hint: 'median, tickets raised in period' },
		{ label: 'Time to resolve', value: duration(r.totals.medianResolutionHours), hint: 'median, resolved in period' }
	]);

	// Status parts take the categorical slots in a fixed order. Text inside a
	// part is white, except dark on the yellow slot.
	const statusColors: Record<TicketStatus, string> = {
		unassigned: 'var(--viz-1)',
		in_progress: 'var(--viz-2)',
		pending: 'var(--viz-3)',
		resolved: 'var(--viz-4)',
		closed: 'var(--viz-5)'
	};
	const statusParts = $derived(
		r.byStatus.map((s) => ({
			key: s.key,
			label: statusLabels[s.key as TicketStatus],
			value: s.count,
			color: statusColors[s.key as TicketStatus],
			ink: s.key === 'resolved' ? ('dark' as const) : ('light' as const)
		}))
	);

	const rowsOf = (list: NamedCount[], label: (n: NamedCount) => string = (n) => n.name) =>
		list.map((n) => ({ key: n.key || 'none', label: label(n), value: n.count }));
	const priorityRows = $derived(rowsOf(r.byPriority, (n) => priorityLabel(n.key as Priority)));
	const categoryRows = $derived(rowsOf(r.byCategory));
	const assigneeRows = $derived(rowsOf(r.byAssignee));
	const byKey = (list: NamedCount[]) => new Map(list.map((n) => [n.key || 'none', n]));
	const categoryByKey = $derived(byKey(r.byCategory));
	const assigneeByKey = $derived(byKey(r.byAssignee));

	// "Show as table" toggles, per chart.
	let tables = $state<Record<string, boolean>>({});
	const toggleTable = (id: string) => (tables[id] = !tables[id]);

	// --- Export ---
	const iso = (d: Date) => `${d.getFullYear()}-${String(d.getMonth() + 1).padStart(2, '0')}-${String(d.getDate()).padStart(2, '0')}`;
	const today = new Date();
	let from = $state(iso(new Date(today.getFullYear(), today.getMonth(), today.getDate() - 29)));
	let to = $state(iso(today));
	let exportStatus = $state('');
	let exportCategory = $state<number | ''>('');
	let withDescription = $state(false);
	let format = $state<'csv' | 'json'>('csv');
	const rangeValid = $derived(!!from && !!to && from <= to);
	const exportHref = $derived(
		reports.exportUrl({ from, to, status: exportStatus, category: exportCategory, description: withDescription, format })
	);

	const statusFilterOptions: DropdownOption<string>[] = [
		{ value: '', label: 'Any status' },
		{ value: ACTIVE_STATUSES, label: 'Active (not resolved or closed)' },
		...(Object.keys(statusLabels) as TicketStatus[]).map((s) => ({ value: s, label: statusLabels[s] }))
	];
	const categoryFilterOptions = $derived<DropdownOption<number | ''>[]>([
		{ value: '', label: 'Any category' },
		...data.categories.map((c) => ({ value: c.id, label: c.name }))
	]);

	const card = 'rounded-xl border border-ink-800 bg-ink-900/60 p-5';
	const tableBtn = 'text-xs font-medium text-ink-500 transition hover:text-ink-100';
	const th = 'py-1.5 pr-3 text-left text-xs font-medium text-ink-500';
	const td = 'py-1.5 pr-3 text-ink-200';
</script>

<svelte:head>
	<title>Reports · Lithium</title>
</svelte:head>

{#snippet cardHeader(id: string, title: string, subtitle: string)}
	<div class="mb-4 flex items-start justify-between gap-3">
		<div>
			<h2 class="text-sm font-semibold text-ink-100">{title}</h2>
			<p class="mt-0.5 text-xs text-ink-500">{subtitle}</p>
		</div>
		<button type="button" onclick={() => toggleTable(id)} class={tableBtn} aria-pressed={!!tables[id]}>
			{tables[id] ? 'Show chart' : 'Show table'}
		</button>
	</div>
{/snippet}

{#snippet countTable(rows: { label: string; value: number }[], heading: string)}
	<table class="w-full text-sm">
		<thead><tr class="border-b border-ink-800"><th class={th}>{heading}</th><th class="{th} text-right">Tickets</th></tr></thead>
		<tbody>
			{#each rows as row (row.label)}
				<tr class="border-b border-ink-800/60 last:border-0"><td class={td}>{row.label}</td><td class="{td} text-right tabular-nums">{row.value}</td></tr>
			{/each}
		</tbody>
	</table>
{/snippet}

<div class="mx-auto max-w-7xl px-4 py-10 sm:px-6">
	<div class="flex flex-wrap items-end justify-between gap-4">
		<div>
			<h1 class="text-2xl font-semibold tracking-tight">Reports</h1>
			<p class="mt-1 text-sm text-ink-400">How the queue is doing, and an export of the tickets behind it.</p>
		</div>
		<!-- Period: one filter row above the charts. -->
		<div role="radiogroup" aria-label="Period" class="inline-flex gap-0.5 rounded-lg border border-ink-800 bg-ink-950/60 p-0.5">
			{#each [7, 30, 90] as d (d)}
				<a
					href={periodHref(d)}
					role="radio"
					aria-checked={data.days === d}
					data-sveltekit-noscroll
					data-sveltekit-replacestate
					class={['rounded-md px-3 py-1.5 text-xs font-medium transition', data.days === d ? 'bg-ink-850 text-ink-100' : 'text-ink-500 hover:text-ink-200']}
				>
					Last {d} days
				</a>
			{/each}
		</div>
	</div>

	<!-- Headline numbers -->
	<dl class="mt-8 grid grid-cols-2 gap-3 md:grid-cols-3 lg:grid-cols-5">
		{#each tiles as t (t.label)}
			<div class="rounded-xl border border-ink-800 bg-ink-900/60 p-4">
				<dt class="text-[13px] text-ink-400">{t.label}</dt>
				<dd class="mt-2 text-2xl font-semibold tracking-tight text-ink-100">{t.value}</dd>
				<dd class="mt-0.5 text-xs text-ink-500">{t.hint}</dd>
			</div>
		{/each}
	</dl>

	<!-- Raised vs resolved -->
	<section class="mt-6 {card}">
		{@render cardHeader('trend', 'Raised vs resolved', `Tickets per day, last ${r.days} days`)}
		{#if tables.trend}
			<div class="max-h-80 overflow-y-auto">
				<table class="w-full text-sm">
					<thead class="sticky top-0 bg-ink-900"><tr class="border-b border-ink-800"><th class={th}>Day</th><th class="{th} text-right">Raised</th><th class="{th} text-right">Resolved</th></tr></thead>
					<tbody>
						{#each [...r.series].reverse() as p (p.date)}
							<tr class="border-b border-ink-800/60 last:border-0"><td class={td}>{formatDay(p.date)}</td><td class="{td} text-right tabular-nums">{p.raised}</td><td class="{td} text-right tabular-nums">{p.resolved}</td></tr>
						{/each}
					</tbody>
				</table>
			</div>
		{:else}
			<LineChart
				labels={r.series.map((p) => p.date)}
				series={[
					{ name: 'Raised', color: 'var(--viz-1)', values: r.series.map((p) => p.raised) },
					{ name: 'Resolved', color: 'var(--viz-2)', values: r.series.map((p) => p.resolved) }
				]}
				formatLabel={formatDay}
				ariaLabel="Tickets raised and resolved per day over the last {r.days} days. Use left and right arrow keys to read each day."
			/>
		{/if}
	</section>

	<!-- Current state -->
	<section class="mt-6 {card}">
		{@render cardHeader('status', 'Tickets by status', 'Every ticket, as it stands now')}
		{#if tables.status}
			{@render countTable(statusParts, 'Status')}
		{:else}
			<SegmentedBar parts={statusParts} ariaLabel="Tickets by current status: {statusParts.map((p) => `${p.label} ${p.value}`).join(', ')}" />
		{/if}
	</section>

	<div class="mt-6 grid gap-6 lg:grid-cols-3">
		<section class={card}>
			{@render cardHeader('priority', 'Active by priority', 'Open, in progress and pending')}
			{#if tables.priority}
				{@render countTable(priorityRows, 'Priority')}
			{:else}
				<BarList rows={priorityRows} empty="No active tickets">
					{#snippet lead(row)}<PriorityIcon priority={row.key as Priority} class="size-3.5 shrink-0 text-ink-400" />{/snippet}
				</BarList>
			{/if}
		</section>

		<section class={card}>
			{@render cardHeader('category', 'Active by category', 'Where the work is')}
			{#if tables.category}
				{@render countTable(categoryRows, 'Category')}
			{:else}
				<BarList rows={categoryRows} empty="No active tickets">
					{#snippet lead(row)}
						{@const c = categoryByKey.get(row.key)}
						{#if c?.color}
							<span class="tag-{c.color} flex size-2.5 shrink-0"><span class="tag-dot size-2.5 rounded-full"></span></span>
						{:else}
							<span class="size-2.5 shrink-0 rounded-full border border-dashed border-ink-500"></span>
						{/if}
					{/snippet}
				</BarList>
			{/if}
		</section>

		<section class={card}>
			{@render cardHeader('assignee', 'Workload', 'Active tickets per assignee')}
			{#if tables.assignee}
				{@render countTable(assigneeRows, 'Assignee')}
			{:else}
				<BarList rows={assigneeRows} empty="No active tickets">
					{#snippet lead(row)}
						{@const a = assigneeByKey.get(row.key)}
						{#if a && a.key}
							<Avatar name={a.name} src={a.avatarUrl} size={18} />
						{:else}
							<span class="size-[18px] shrink-0 rounded-full border border-dashed border-ink-500"></span>
						{/if}
					{/snippet}
				</BarList>
			{/if}
		</section>
	</div>

	<!-- Export -->
	<section class="mt-6 {card}" aria-labelledby="export-heading">
		<div class="flex flex-wrap items-start justify-between gap-3">
			<div>
				<h2 id="export-heading" class="flex items-center gap-2 text-sm font-semibold text-ink-100"><Icon name="upload" class="size-4 rotate-180" /> Export tickets</h2>
				<p class="mt-0.5 text-xs text-ink-500">Download tickets raised in a date range. Only includes tickets you're allowed to see.</p>
			</div>
		</div>
		<div class="mt-4 grid gap-4 sm:grid-cols-2 lg:grid-cols-[auto_auto_1fr_1fr]">
			<div>
				<label for="export-from" class="mb-1 block text-xs text-ink-500">From</label>
				<input id="export-from" type="date" bind:value={from} max={to} class="field h-9 text-[13px]" />
			</div>
			<div>
				<label for="export-to" class="mb-1 block text-xs text-ink-500">To</label>
				<input id="export-to" type="date" bind:value={to} min={from} class="field h-9 text-[13px]" />
			</div>
			<div>
				<label for="export-status" class="mb-1 block text-xs text-ink-500">Status</label>
				<Dropdown id="export-status" size="sm" bind:value={exportStatus} options={statusFilterOptions}>
					{#snippet item(opt)}<span class={['truncate text-[13px]', opt.value ? 'text-ink-100' : 'text-ink-400']}>{opt.label}</span>{/snippet}
				</Dropdown>
			</div>
			<div>
				<label for="export-category" class="mb-1 block text-xs text-ink-500">Category</label>
				<Dropdown id="export-category" size="sm" bind:value={exportCategory} options={categoryFilterOptions}>
					{#snippet item(opt)}<span class={['truncate text-[13px]', opt.value ? 'text-ink-100' : 'text-ink-400']}>{opt.label}</span>{/snippet}
				</Dropdown>
			</div>
		</div>
		<div class="mt-4 flex flex-wrap items-center justify-between gap-4 border-t border-ink-800 pt-4">
			<div class="flex flex-wrap items-center gap-5">
				<div role="radiogroup" aria-label="Format" class="inline-flex gap-0.5 rounded-lg border border-ink-800 bg-ink-950/60 p-0.5">
					{#each [['csv', 'CSV (Excel, Sheets)'], ['json', 'JSON']] as [value, label] (value)}
						<button
							type="button"
							role="radio"
							aria-checked={format === value}
							onclick={() => (format = value as 'csv' | 'json')}
							class={['rounded-md px-3 py-1.5 text-xs font-medium transition', format === value ? 'bg-ink-850 text-ink-100' : 'text-ink-500 hover:text-ink-200']}
						>
							{label}
						</button>
					{/each}
				</div>
				<label class="flex cursor-pointer items-center gap-2 text-sm text-ink-200">
					<input type="checkbox" bind:checked={withDescription} class="size-4 accent-[var(--color-primary-400)]" />
					Include descriptions
				</label>
			</div>
			{#if rangeValid}
				<a
					href={exportHref}
					download
					class="inline-flex h-9 items-center gap-1.5 rounded-lg bg-primary-400 px-3.5 text-sm font-semibold text-primary-950 transition hover:bg-primary-300"
				>
					<Icon name="upload" class="size-4 rotate-180" /> Download {format.toUpperCase()}
				</a>
			{:else}
				<span class="text-xs text-primary-300">Choose a “to” date on or after the “from” date.</span>
			{/if}
		</div>
	</section>
</div>
