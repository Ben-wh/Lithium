<script lang="ts">
	import { goto } from '$app/navigation';
	import { page } from '$app/state';
	import { activity, type ActivityCategory, type ActivityEntry } from '$lib/api';
	import Icon, { type IconName } from '$lib/components/Icon.svelte';
	import { dateTimeFmt } from '$lib/format';

	let { data } = $props();

	const categories: { id: ActivityCategory; label: string; icon: IconName }[] = [
		{ id: 'auth', label: 'Sign-in', icon: 'lock' },
		{ id: 'tickets', label: 'Tickets', icon: 'ticket' },
		{ id: 'users', label: 'Users', icon: 'users' },
		{ id: 'settings', label: 'Settings', icon: 'settings' },
		{ id: 'knowledge', label: 'Knowledge base', icon: 'book' },
		{ id: 'profile', label: 'Profiles', icon: 'user' },
		{ id: 'reports', label: 'Reports', icon: 'chart' },
		{ id: 'system', label: 'System', icon: 'shield' }
	];
	const categoryInfo = (id: string) => categories.find((c) => c.id === id) ?? categories[categories.length - 1];

	// Rows shown; reset when the filters change, extended by "Load more".
	let rows = $derived<ActivityEntry[]>(data.entries);
	let hasMore = $derived(data.hasMore);
	let loadingMore = $state(false);
	let loadError = $state(false);

	async function loadMore() {
		loadingMore = true;
		loadError = false;
		try {
			const next = await activity.list({ ...data.query, before: rows[rows.length - 1]?.id });
			rows = [...rows, ...next.entries];
			hasMore = next.hasMore;
		} catch {
			loadError = true;
		} finally {
			loadingMore = false;
		}
	}

	// Filters live in the URL, so a filtered view can be shared or bookmarked.
	function setParam(key: string, value: string) {
		const url = new URL(page.url);
		if (value) url.searchParams.set(key, value);
		else url.searchParams.delete(key);
		goto(url, { replaceState: true, keepFocus: true, noScroll: true });
	}

	let search = $state(page.url.searchParams.get('q') ?? '');
	$effect(() => {
		const q = search.trim();
		if (q === data.query.q) return;
		const t = setTimeout(() => setParam('q', q), 250);
		return () => clearTimeout(t);
	});

	const filtered = $derived(!!(data.query.q || data.query.category || data.query.outcome || data.from || data.to));

	function clearFilters() {
		search = '';
		goto(page.url.pathname, { replaceState: true, noScroll: true });
	}

	// --- Grouping by day ---
	const dayFmt = new Intl.DateTimeFormat(undefined, { weekday: 'long', day: 'numeric', month: 'long', year: 'numeric' });
	const timeFmt = new Intl.DateTimeFormat(undefined, { hour: '2-digit', minute: '2-digit' });

	function dayLabel(d: Date) {
		const today = new Date();
		const yesterday = new Date(today);
		yesterday.setDate(today.getDate() - 1);
		if (d.toDateString() === today.toDateString()) return 'Today';
		if (d.toDateString() === yesterday.toDateString()) return 'Yesterday';
		return dayFmt.format(d);
	}

	const days = $derived.by(() => {
		const out: { label: string; entries: ActivityEntry[] }[] = [];
		for (const e of rows) {
			const label = dayLabel(new Date(e.at));
			if (out.at(-1)?.label !== label) out.push({ label, entries: [] });
			out.at(-1)!.entries.push(e);
		}
		return out;
	});

	function who(e: ActivityEntry) {
		if (e.actorName) return e.actorName;
		if (e.actorEmail) return e.actorEmail;
		return 'Someone';
	}

	let expanded = $state<number | null>(null);
</script>

<svelte:head>
	<title>Activity · Lithium</title>
</svelte:head>

<div class="mx-auto max-w-7xl px-4 py-10 sm:px-6">
	<h1 class="text-2xl font-semibold tracking-tight">Activity</h1>
	<p class="mt-1 text-sm text-ink-400">
		Everything people have changed in Lithium, including sign-ins and refused attempts, over the last year. Only admins
		can see this page.
	</p>

	<!-- Filters -->
	<div class="mt-6 flex flex-wrap items-end gap-3">
		<div class="relative w-full sm:w-80">
			<Icon name="search" class="pointer-events-none absolute top-1/2 left-3 size-4 -translate-y-1/2 text-ink-500" />
			<input
				type="search"
				bind:value={search}
				placeholder="Search people, actions, tickets, IPs…"
				aria-label="Search activity"
				class="field h-9 pl-9 text-[13px]"
			/>
		</div>
		<label class="flex flex-col gap-1 text-xs text-ink-500">
			From
			<input
				type="date"
				value={data.from}
				max={data.to || undefined}
				onchange={(e) => setParam('from', e.currentTarget.value)}
				class="field h-9 text-[13px]"
			/>
		</label>
		<label class="flex flex-col gap-1 text-xs text-ink-500">
			To
			<input
				type="date"
				value={data.to}
				min={data.from || undefined}
				onchange={(e) => setParam('to', e.currentTarget.value)}
				class="field h-9 text-[13px]"
			/>
		</label>
		<button
			type="button"
			aria-pressed={data.query.outcome === 'failure'}
			onclick={() => setParam('outcome', data.query.outcome === 'failure' ? '' : 'failure')}
			class={[
				'flex h-9 items-center gap-1.5 rounded-lg border px-3 text-[13px] transition',
				data.query.outcome === 'failure'
					? 'border-danger-400/50 bg-danger-400/10 text-danger-200'
					: 'border-ink-700 text-ink-300 hover:border-ink-500 hover:text-ink-100'
			]}
		>
			<Icon name="shieldAlert" class="size-4" />
			Failed only
		</button>
		{#if filtered}
			<button type="button" onclick={clearFilters} class="h-9 px-1 text-[13px] text-ink-400 hover:text-primary-300">
				Clear filters
			</button>
		{/if}
	</div>

	<div class="no-scrollbar -mx-4 mt-4 flex gap-1.5 overflow-x-auto px-4 sm:mx-0 sm:flex-wrap sm:px-0" role="group" aria-label="Filter by type">
		{#each [{ id: '', label: 'Everything', icon: 'activity' as IconName }, ...categories] as c (c.id)}
			{@const active = (data.query.category ?? '') === c.id}
			<button
				type="button"
				aria-pressed={active}
				onclick={() => setParam('category', c.id)}
				class={[
					'flex h-8 shrink-0 items-center gap-1.5 rounded-full border px-3 text-[13px] transition',
					active
						? 'border-primary-400/50 bg-primary-400/10 text-primary-200'
						: 'border-ink-800 text-ink-400 hover:border-ink-600 hover:text-ink-100'
				]}
			>
				<Icon name={c.icon} class="size-3.5" />
				{c.label}
			</button>
		{/each}
	</div>

	<!-- Log -->
	<div class="mt-5 rounded-xl border border-ink-800 bg-ink-900/60">
		{#if rows.length === 0}
			<div class="flex flex-col items-center px-6 py-16 text-center">
				<Icon name="activity" class="size-6 text-ink-600" />
				<p class="mt-3 text-sm font-medium text-ink-200">{filtered ? 'Nothing matches' : 'No activity yet'}</p>
				<p class="mt-1 text-sm text-ink-500">
					{filtered ? 'Try fewer words or a wider date range.' : 'Changes people make will show up here.'}
				</p>
			</div>
		{:else}
			{#each days as day (day.label)}
				<h2 class="sticky top-0 z-[1] border-b border-ink-800 bg-ink-900/95 px-5 py-2 text-xs font-medium text-ink-400 backdrop-blur-sm first:rounded-t-xl">
					{day.label}
				</h2>
				<ul>
					{#each day.entries as e (e.id)}
						{@const cat = categoryInfo(e.category)}
						{@const failed = e.outcome === 'failure'}
						{@const open = expanded === e.id}
						<li class="border-b border-ink-800/70 last:border-b-0">
							<button
								type="button"
								aria-expanded={open}
								onclick={() => (expanded = open ? null : e.id)}
								class="flex w-full items-start gap-3 px-5 py-3 text-left transition hover:bg-ink-850/50"
							>
								<time datetime={e.at} title={dateTimeFmt.format(new Date(e.at))} class="w-12 shrink-0 pt-0.5 text-xs tabular-nums text-ink-500">
									{timeFmt.format(new Date(e.at))}
								</time>
								<span
									class={[
										'mt-px flex size-6 shrink-0 items-center justify-center rounded-full',
										failed ? 'bg-danger-400/15 text-danger-300' : 'bg-ink-800 text-ink-400'
									]}
									title={cat.label}
								>
									<Icon name={failed ? 'shieldAlert' : cat.icon} class="size-3.5" />
								</span>
								<span class="min-w-0 flex-1">
									<span class="block text-sm text-ink-200">
										<span class="font-medium text-ink-100">{who(e)}</span>
										<span class="text-ink-400">·</span>
										{e.summary}{#if e.target}<span class="text-ink-400">:</span>
											<span class="font-medium text-ink-100">{e.target}</span>{/if}
										{#if failed}
											<span class="ml-1 rounded bg-danger-400/15 px-1.5 py-0.5 text-[11px] font-semibold text-danger-200">Failed</span>
										{/if}
									</span>
									{#if e.detail}
										<span class="mt-0.5 block truncate text-xs text-ink-500" class:whitespace-normal={open}>{e.detail}</span>
									{/if}
								</span>
								<span class="hidden shrink-0 pt-0.5 font-mono text-[11px] text-ink-500 sm:block">{e.ip}</span>
							</button>
							{#if open}
								<dl class="grid gap-x-6 gap-y-1.5 px-5 pb-4 pl-[6.5rem] text-xs sm:grid-cols-[max-content_1fr]">
									<dt class="text-ink-500">When</dt>
									<dd class="text-ink-300">{dateTimeFmt.format(new Date(e.at))}</dd>
									{#if e.actorEmail}
										<dt class="text-ink-500">Account</dt>
										<dd class="text-ink-300">
											{e.actorEmail}{#if e.actorId === null}<span class="text-ink-500"> (no matching account)</span>{/if}
										</dd>
									{/if}
									<dt class="text-ink-500">Type</dt>
									<dd class="text-ink-300">{cat.label} <span class="font-mono text-ink-500">({e.action})</span></dd>
									{#if e.ip}
										<dt class="text-ink-500">IP address</dt>
										<dd class="font-mono text-ink-300">{e.ip}</dd>
									{/if}
									{#if e.userAgent}
										<dt class="text-ink-500">Browser</dt>
										<dd class="break-all text-ink-300">{e.userAgent}</dd>
									{/if}
								</dl>
							{/if}
						</li>
					{/each}
				</ul>
			{/each}
		{/if}
	</div>

	{#if hasMore}
		<div class="mt-4 flex flex-col items-center gap-2">
			<button
				type="button"
				onclick={loadMore}
				disabled={loadingMore}
				class="h-9 rounded-lg border border-ink-700 px-4 text-[13px] text-ink-200 transition hover:border-ink-500 hover:text-ink-100 disabled:opacity-60"
			>
				{loadingMore ? 'Loading…' : 'Load more'}
			</button>
			{#if loadError}
				<p class="text-xs text-danger-300">Couldn't load more. Try again.</p>
			{/if}
		</div>
	{/if}
</div>
