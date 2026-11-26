<script lang="ts">
	import { goto } from '$app/navigation';
	import { page } from '$app/state';
	import { tickets, type Priority, type TicketSummary } from '$lib/api';
	import Avatar from '$lib/components/Avatar.svelte';
	import Dropdown, { type DropdownOption } from '$lib/components/Dropdown.svelte';
	import Icon from '$lib/components/Icon.svelte';
	import PriorityIcon from '$lib/components/PriorityIcon.svelte';
	import TicketRow from '$lib/components/TicketRow.svelte';
	import { priorities, TICKET_PAGE_SIZE, ticketSorts, ticketViews as views, type TicketView as View } from '$lib/tickets';

	let { data } = $props();

	const isStaff = $derived(data.user?.role === 'admin' || data.user?.role === 'agent');
	const f = $derived(data.filters);
	const filtered = $derived(f.priority !== '' || f.assignee !== '' || f.category !== '' || f.q !== '');

	// Rows shown; reset whenever the filters change, extended by "Load more".
	let items = $derived<TicketSummary[]>(data.list.tickets);
	let loadingMore = $state(false);
	let loadMoreFailed = $state(false);

	async function loadMore() {
		loadingMore = true;
		loadMoreFailed = false;
		try {
			const next = await tickets.list({ ...data.query, offset: items.length, limit: TICKET_PAGE_SIZE });
			items = [...items, ...next.tickets];
		} catch {
			loadMoreFailed = true;
		} finally {
			loadingMore = false;
		}
	}

	function setParams(changes: Record<string, string>) {
		const url = new URL(page.url);
		for (const [k, v] of Object.entries(changes)) {
			if (v) url.searchParams.set(k, v);
			else url.searchParams.delete(k);
		}
		goto(url, { replaceState: true, keepFocus: true, noScroll: true });
	}

	// Debounced search box, kept in the URL.
	let search = $state(page.url.searchParams.get('q') ?? '');
	$effect(() => {
		const q = search.trim();
		if (q === f.q) return;
		const t = setTimeout(() => setParams({ q }), 250);
		return () => clearTimeout(t);
	});

	function clearFilters() {
		search = '';
		setParams({ q: '', priority: '', assignee: '', category: '' });
	}

	const viewHref = (v: View) => {
		const url = new URL(page.url);
		if (v === 'active') url.searchParams.delete('view');
		else url.searchParams.set('view', v);
		return url.pathname + url.search;
	};

	// Filter dropdowns. "" means "any".
	const priorityOptions: DropdownOption<Priority | ''>[] = [
		{ value: '', label: 'Any priority' },
		...priorities.map((p) => ({ value: p.value, label: p.label }))
	];
	const assigneeOptions: DropdownOption<'' | 'me' | 'unassigned'>[] = [
		{ value: '', label: 'Anyone' },
		{ value: 'me', label: 'Assigned to me' },
		{ value: 'unassigned', label: 'Unassigned' }
	];
	// Sorting isn't a filter: "Clear filters" leaves it alone.
	const sortOptions = $derived(ticketSorts.filter((s) => isStaff || !('staffOnly' in s)));

	const categoryById = $derived(new Map(data.categories.map((c) => [c.id, c])));
	const categoryOptions = $derived<DropdownOption<number | ''>[]>([
		{ value: '', label: 'Any category' },
		...data.categories.map((c) => ({ value: c.id, label: c.name, keywords: c.description }))
	]);
</script>

<svelte:head>
	<title>Tickets · Lithium</title>
</svelte:head>

<div class="mx-auto max-w-7xl px-4 py-10 sm:px-6">
	<div class="flex flex-wrap items-end justify-between gap-4">
		<div>
			<h1 class="text-2xl font-semibold tracking-tight">{isStaff ? 'Tickets' : 'Your tickets'}</h1>
			<p class="mt-1 text-sm text-ink-400">
				{data.list.total.toLocaleString()}
				{f.view === 'all' ? '' : views[f.view].label.toLowerCase()}
				{data.list.total === 1 ? 'ticket' : 'tickets'}{filtered ? ' matching your filters' : ''}
			</p>
		</div>
	</div>

	<!-- Status views -->
	<nav aria-label="Ticket status" class="no-scrollbar mt-6 -mx-4 overflow-x-auto px-4 sm:mx-0 sm:px-0">
		<div class="tab-row-line flex gap-5">
			{#each Object.entries(views) as [key, v] (key)}
				{@const active = f.view === key}
				<a
					href={viewHref(key as View)}
					data-sveltekit-replacestate
					data-sveltekit-noscroll
					aria-current={active ? 'page' : undefined}
					class={[
						'relative shrink-0 pb-3 text-sm font-medium transition',
						active
							? 'text-ink-100 after:absolute after:inset-x-0 after:bottom-0 after:h-0.5 after:rounded-full after:bg-primary-400'
							: 'text-ink-400 hover:text-ink-100'
					]}
				>
					{v.label}
				</a>
			{/each}
		</div>
	</nav>

	<!-- Filters -->
	<div class="mt-4 flex flex-wrap items-center gap-2">
		<div class="relative w-full sm:w-80">
			<Icon name="search" class="pointer-events-none absolute top-1/2 left-3 size-4 -translate-y-1/2 text-ink-500" />
			<input
				type="search"
				bind:value={search}
				placeholder={isStaff
					? `Search subject, requester or ${data.settings.ticketPrefix}-42`
					: `Search subject or ${data.settings.ticketPrefix}-42`}
				aria-label="Search tickets"
				class="field h-9 pl-9 text-[13px]"
			/>
		</div>

		<div class="w-40">
			{#key f.priority}
				<Dropdown
					id="filter-priority"
					size="sm"
					label="Priority"
					value={f.priority}
					options={priorityOptions}
					onchange={(v) => setParams({ priority: v })}
				>
					{#snippet item(opt)}
						{#if opt.value}
							<PriorityIcon priority={opt.value} class="size-4 shrink-0 text-ink-400" />
						{:else}
							<Icon name="chart" class="size-4 shrink-0 text-ink-500" />
						{/if}
						<span class={['truncate text-[13px]', opt.value ? 'text-ink-100' : 'text-ink-400']}>{opt.label}</span>
					{/snippet}
				</Dropdown>
			{/key}
		</div>

		{#if data.categories.length > 0}
			<div class="w-44">
				{#key f.category}
					<Dropdown
						id="filter-category"
						size="sm"
						menu="wide"
						label="Category"
						value={f.category}
						options={categoryOptions}
						searchable={data.categories.length > 8}
						onchange={(v) => setParams({ category: v === '' ? '' : String(v) })}
					>
						{#snippet item(opt)}
							{@const c = opt.value === '' ? null : categoryById.get(opt.value)}
							{#if c}
								<span class="tag-{c.color} flex size-3 shrink-0 items-center justify-center">
									<span class="tag-dot size-2.5 rounded-full"></span>
								</span>
								<span class="truncate text-[13px] text-ink-100">{c.name}</span>
							{:else}
								<Icon name="tag" class="size-4 shrink-0 text-ink-500" />
								<span class="truncate text-[13px] text-ink-400">Any category</span>
							{/if}
						{/snippet}
					</Dropdown>
				{/key}
			</div>
		{/if}

		{#if isStaff}
			<div class="w-44">
				{#key f.assignee}
					<Dropdown
						id="filter-assignee"
						size="sm"
						label="Assignee"
						value={f.assignee}
						options={assigneeOptions}
						onchange={(v) => setParams({ assignee: v })}
					>
						{#snippet item(opt)}
							{#if opt.value === 'me' && data.user}
								<Avatar name={data.user.name} src={data.user.avatarUrl} size={20} tone="soft" />
							{:else if opt.value === 'unassigned'}
								<span class="size-5 shrink-0 rounded-full border border-dashed border-ink-500"></span>
							{:else}
								<Icon name="users" class="size-4 shrink-0 text-ink-500" />
							{/if}
							<span class={['truncate text-[13px]', opt.value ? 'text-ink-100' : 'text-ink-400']}>{opt.label}</span>
						{/snippet}
					</Dropdown>
				{/key}
			</div>
		{/if}

		{#if filtered}
			<button type="button" onclick={clearFilters} class="h-9 rounded-lg px-2.5 text-[13px] text-ink-400 transition hover:bg-ink-850 hover:text-ink-100">
				Clear filters
			</button>
		{/if}

		<div class="w-56 sm:ml-auto">
			{#key f.sort}
				<Dropdown
					id="sort"
					size="sm"
					label="Sort"
					value={f.sort}
					options={sortOptions}
					onchange={(v) => setParams({ sort: v === ticketSorts[0].value ? '' : v })}
				>
					{#snippet item(opt)}
						<Icon name="sort" class="size-4 shrink-0 text-ink-500" />
						<span class="truncate text-[13px] text-ink-100">{opt.label}</span>
					{/snippet}
				</Dropdown>
			{/key}
		</div>
	</div>

	<!-- List -->
	<div class="mt-4 overflow-hidden rounded-xl border border-ink-800 bg-ink-900/60">
		{#if items.length === 0}
			<div class="flex flex-col items-center px-6 py-16 text-center">
				<span class="flex size-11 items-center justify-center rounded-xl bg-primary-400/10 text-primary-400">
					<Icon name={filtered ? 'search' : 'inbox'} class="size-5" />
				</span>
				{#if filtered}
					<p class="mt-4 text-sm font-medium text-ink-200">No tickets match your filters</p>
					<button type="button" onclick={clearFilters} class="mt-2 text-sm text-primary-300 hover:text-primary-200">Clear filters</button>
				{:else}
					<p class="mt-4 text-sm font-medium text-ink-200">
						{f.view === 'all' ? 'No tickets yet' : `No ${views[f.view].label.toLowerCase()} tickets`}
					</p>
					<p class="mt-1 text-sm text-ink-500">
						{f.view === 'active' ? 'Nice — nothing needs attention right now.' : 'Tickets will show up here when they reach this status.'}
					</p>
				{/if}
			</div>
		{:else}
			<ul class="divide-y divide-ink-800 p-1.5">
				{#each items as t (t.id)}
					<li><TicketRow ticket={t} showRequester={isStaff} /></li>
				{/each}
			</ul>
		{/if}
	</div>

	{#if items.length < data.list.total}
		<div class="mt-4 flex flex-col items-center gap-2">
			<button
				type="button"
				onclick={loadMore}
				disabled={loadingMore}
				class="h-9 rounded-lg border border-ink-700 px-4 text-sm text-ink-200 transition hover:border-primary-400/40 hover:text-primary-200 disabled:opacity-60"
			>
				{loadingMore ? 'Loading…' : `Load more (${(data.list.total - items.length).toLocaleString()} left)`}
			</button>
			{#if loadMoreFailed}<p class="text-xs text-danger-300">Couldn't load more tickets. Try again.</p>{/if}
		</div>
	{/if}
</div>
