<script lang="ts">
	import { goto } from '$app/navigation';
	import { page } from '$app/state';
	import { people, type Person } from '$lib/api';
	import Avatar from '$lib/components/Avatar.svelte';
	import Icon from '$lib/components/Icon.svelte';
	import { timeAgo } from '$lib/format';

	let { data } = $props();

	// Rows shown; reset on each new search, extended by "Load more".
	let rows = $derived<Person[]>(data.people);
	let loadingMore = $state(false);

	async function loadMore() {
		loadingMore = true;
		try {
			const next = await people.list(data.q, rows.length);
			rows = [...rows, ...next.people];
		} finally {
			loadingMore = false;
		}
	}

	// Debounced search, kept in the URL.
	let search = $state(page.url.searchParams.get('q') ?? '');
	$effect(() => {
		const q = search.trim();
		if (q === data.q) return;
		const t = setTimeout(() => {
			const url = new URL(page.url);
			if (q) url.searchParams.set('q', q);
			else url.searchParams.delete('q');
			goto(url, { replaceState: true, keepFocus: true, noScroll: true });
		}, 250);
		return () => clearTimeout(t);
	});

	const roleLabels = { admin: 'Admin', agent: 'Agent', user: 'User' } as const;
</script>

<svelte:head>
	<title>User book · Lithium</title>
</svelte:head>

<div class="mx-auto max-w-7xl px-4 py-10 sm:px-6">
	<h1 class="text-2xl font-semibold tracking-tight">User book</h1>
	<p class="mt-1 text-sm text-ink-400">
		Everyone who raises tickets, with their history. {data.total.toLocaleString()}
		{data.total === 1 ? 'person' : 'people'}{data.q ? ` matching “${data.q}”` : ''}.
	</p>

	<div class="relative mt-6 w-full sm:w-80">
		<Icon name="search" class="pointer-events-none absolute top-1/2 left-3 size-4 -translate-y-1/2 text-ink-500" />
		<input type="search" bind:value={search} placeholder="Search by name or email" aria-label="Search people" class="field h-9 pl-9 text-[13px]" />
	</div>

	<div class="mt-4 rounded-xl border border-ink-800 bg-ink-900/60">
		{#if rows.length === 0}
			<div class="flex flex-col items-center px-6 py-16 text-center">
				<span class="flex size-11 items-center justify-center rounded-xl bg-primary-400/10 text-primary-400">
					<Icon name="users" class="size-5" />
				</span>
				<p class="mt-4 text-sm font-medium text-ink-200">{data.q ? 'No one matches your search' : 'No one here yet'}</p>
				<p class="mt-1 text-sm text-ink-500">People appear here once they have an account or raise a ticket.</p>
			</div>
		{:else}
			<div class="hidden grid-cols-[1fr_90px_90px_120px] gap-4 border-b border-ink-800 px-5 py-3 text-xs text-ink-500 sm:grid">
				<span>Person</span>
				<span class="text-right">Open</span>
				<span class="text-right">Total</span>
				<span class="text-right">Last ticket</span>
			</div>
			<ul class="divide-y divide-ink-800 p-1.5">
				{#each rows as p (p.id)}
					<li>
						<a
							href="/user-book/{p.id}"
							class="grid grid-cols-[1fr_auto] items-center gap-4 rounded-lg px-3.5 py-3 transition hover:bg-ink-850/60 focus-visible:bg-ink-850/60 focus-visible:outline-none sm:grid-cols-[1fr_90px_90px_120px]"
						>
							<span class="flex min-w-0 items-center gap-3">
								<Avatar name={p.name} src={p.avatarUrl} size={32} />
								<span class="min-w-0">
									<span class="flex items-center gap-2 text-sm font-medium text-ink-100">
										<span class="truncate">{p.name}</span>
										{#if p.role !== 'user'}
											<span class="shrink-0 rounded-full border border-primary-400/30 bg-primary-400/10 px-1.5 py-px text-[10px] font-medium text-primary-200">
												{roleLabels[p.role]}
											</span>
										{/if}
									</span>
									<span class="block truncate text-xs text-ink-500">{p.email}</span>
								</span>
							</span>
							<span class={['text-right text-sm tabular-nums', p.openTickets ? 'font-medium text-ink-100' : 'text-ink-500']}>
								<span class="sm:hidden text-xs text-ink-500">open </span>{p.openTickets}
							</span>
							<span class="hidden text-right text-sm text-ink-400 tabular-nums sm:block">{p.totalTickets}</span>
							<span class="hidden text-right text-xs text-ink-500 sm:block">
								{p.lastTicketAt ? timeAgo(p.lastTicketAt) : 'Never'}
							</span>
						</a>
					</li>
				{/each}
			</ul>
		{/if}
	</div>

	{#if rows.length < data.total}
		<div class="mt-4 flex justify-center">
			<button
				type="button"
				onclick={loadMore}
				disabled={loadingMore}
				class="h-9 rounded-lg border border-ink-700 px-4 text-sm text-ink-200 transition hover:border-primary-400/40 hover:text-primary-200 disabled:opacity-60"
			>
				{loadingMore ? 'Loading…' : `Load more (${(data.total - rows.length).toLocaleString()} left)`}
			</button>
		</div>
	{/if}
</div>
