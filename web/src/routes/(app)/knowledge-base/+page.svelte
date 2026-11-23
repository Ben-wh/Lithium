<script lang="ts">
	import { goto } from '$app/navigation';
	import { page } from '$app/state';
	import { ApiError, kb, type KBCollection } from '$lib/api';
	import Avatar from '$lib/components/Avatar.svelte';
	import Icon from '$lib/components/Icon.svelte';
	import Modal from '$lib/components/Modal.svelte';
	import { timeAgo } from '$lib/format';

	let { data } = $props();
	const isStaff = $derived(data.user?.role === 'admin' || data.user?.role === 'agent');
	const f = $derived(data.filters);
	const current = $derived(typeof f.collection === 'number' ? data.collections.find((c) => c.id === f.collection) : undefined);
	const totalVisible = $derived(data.collections.reduce((n, c) => n + c.articleCount, 0));

	const heading = $derived(
		f.status === 'draft' ? 'Drafts' : current ? current.name : f.collection === 'none' ? 'Uncategorised' : 'All articles'
	);

	function href(params: Record<string, string>) {
		const url = new URL(page.url);
		for (const k of ['collection', 'status']) url.searchParams.delete(k);
		for (const [k, v] of Object.entries(params)) if (v) url.searchParams.set(k, v);
		return url.pathname + url.search;
	}

	// Debounced search, kept in the URL.
	let search = $state(page.url.searchParams.get('q') ?? '');
	$effect(() => {
		const q = search.trim();
		if (q === f.q) return;
		const t = setTimeout(() => {
			const url = new URL(page.url);
			if (q) url.searchParams.set('q', q);
			else url.searchParams.delete('q');
			goto(url, { replaceState: true, keepFocus: true, noScroll: true });
		}, 250);
		return () => clearTimeout(t);
	});

	// --- New article: starts a draft (in the current collection) and opens the editor ---
	let creating = $state(false);
	async function newArticle() {
		creating = true;
		try {
			const a = await kb.create(typeof f.collection === 'number' ? f.collection : 0);
			await goto(`/knowledge-base/${a.id}/edit`);
		} finally {
			creating = false;
		}
	}

	// --- Collections (staff) ---
	let colOpen = $state(false);
	let editingCol = $state<KBCollection | null>(null);
	let colName = $state('');
	let colDesc = $state('');
	let colError = $state('');
	let colSaving = $state(false);

	function openCollection(c: KBCollection | null) {
		editingCol = c;
		colName = c?.name ?? '';
		colDesc = c?.description ?? '';
		colError = '';
		colOpen = true;
	}

	async function saveCollection(e: SubmitEvent) {
		e.preventDefault();
		colSaving = true;
		colError = '';
		try {
			const input = { name: colName, description: colDesc };
			const c = editingCol ? await kb.updateCollection(editingCol.id, input) : await kb.createCollection(input);
			colOpen = false;
			await goto(href({ collection: String(c.id) }), { invalidateAll: true });
		} catch (err) {
			colError = err instanceof ApiError && err.status < 500 ? err.message.charAt(0).toUpperCase() + err.message.slice(1) + '.' : 'Something went wrong.';
		} finally {
			colSaving = false;
		}
	}

	async function deleteCollection(c: KBCollection) {
		if (!confirm(`Delete the “${c.name}” collection? Its articles won't be deleted; they'll move to Uncategorised.`)) return;
		await kb.deleteCollection(c.id);
		colOpen = false;
		await goto(href({}), { invalidateAll: true });
	}

	const navItem = (active: boolean) => [
		'flex w-full items-center gap-2.5 rounded-lg px-3 py-2 text-left text-sm transition',
		active ? 'bg-ink-850 font-medium text-ink-100' : 'text-ink-400 hover:bg-ink-900 hover:text-ink-100'
	];
</script>

<svelte:head>
	<title>Knowledge base · Lithium</title>
</svelte:head>

<div class="mx-auto max-w-7xl px-4 py-10 sm:px-6">
	<div class="flex flex-wrap items-end justify-between gap-4">
		<div>
			<h1 class="text-2xl font-semibold tracking-tight">Knowledge base</h1>
			<p class="mt-1 text-sm text-ink-400">
				{isStaff ? 'Guides and documentation written by the team.' : 'Guides and answers to common questions.'}
			</p>
		</div>
		{#if isStaff}
			<button
				type="button"
				onclick={newArticle}
				disabled={creating}
				class="inline-flex h-9 items-center gap-1.5 rounded-lg bg-primary-400 px-3.5 text-sm font-semibold text-primary-950 transition hover:bg-primary-300 disabled:opacity-70"
			>
				<Icon name="plus" class="size-4" /> New article
			</button>
		{/if}
	</div>

	<div class="mt-8 grid gap-6 md:grid-cols-[220px_1fr] md:gap-8">
		<!-- Collections -->
		<nav aria-label="Collections" class="md:self-start">
			<div class="relative mb-3">
				<Icon name="search" class="pointer-events-none absolute top-1/2 left-3 size-4 -translate-y-1/2 text-ink-500" />
				<input type="search" bind:value={search} placeholder="Search articles" aria-label="Search articles" class="field h-9 pl-9 text-[13px]" />
			</div>
			<ul class="space-y-0.5">
				<li>
					<a href={href({})} class={navItem(f.collection === '' && f.status === '')}>
						<Icon name="book" class="size-4" /> All articles
						{#if !isStaff}<span class="ml-auto text-xs text-ink-500">{totalVisible}</span>{/if}
					</a>
				</li>
				{#if isStaff}
					<li>
						<a href={href({ status: 'draft' })} class={navItem(f.status === 'draft')}>
							<Icon name="pencil" class="size-4" /> Drafts
						</a>
					</li>
				{/if}
			</ul>
			<div class="mt-5 mb-1.5 flex items-center justify-between px-3">
				<p class="text-[11px] font-medium tracking-wide text-ink-500 uppercase">Collections</p>
				{#if isStaff}
					<button type="button" onclick={() => openCollection(null)} class="text-ink-500 hover:text-primary-300" aria-label="New collection" title="New collection">
						<Icon name="plus" class="size-4" />
					</button>
				{/if}
			</div>
			<ul class="space-y-0.5">
				{#each data.collections as c (c.id)}
					<li class="group relative">
						<a href={href({ collection: String(c.id) })} class={navItem(f.collection === c.id && f.status === '')}>
							<Icon name="tag" class="size-4" />
							<span class="truncate">{c.name}</span>
							<span class="ml-auto text-xs text-ink-500 group-hover:opacity-0">{c.articleCount}</span>
						</a>
						{#if isStaff}
							<button
								type="button"
								onclick={() => openCollection(c)}
								aria-label="Edit {c.name}"
								class="absolute top-1/2 right-2 flex size-6 -translate-y-1/2 items-center justify-center rounded text-ink-500 opacity-0 transition group-hover:opacity-100 hover:text-ink-100 focus-visible:opacity-100"
							>
								<Icon name="pencil" class="size-3.5" />
							</button>
						{/if}
					</li>
				{:else}
					<li class="px-3 py-1 text-xs text-ink-500">{isStaff ? 'No collections yet.' : 'None yet.'}</li>
				{/each}
				{#if isStaff}
					<li>
						<a href={href({ collection: 'none' })} class={navItem(f.collection === 'none')}>
							<Icon name="inbox" class="size-4" /> Uncategorised
						</a>
					</li>
				{/if}
			</ul>
		</nav>

		<!-- Articles -->
		<section aria-labelledby="kb-heading" class="min-w-0">
			<h2 id="kb-heading" class="text-lg font-semibold tracking-tight">{heading}</h2>
			{#if current?.description}<p class="mt-0.5 text-sm text-ink-400">{current.description}</p>{/if}
			{#if f.q}<p class="mt-0.5 text-sm text-ink-400">Matching “{f.q}”</p>{/if}

			{#if data.articles.length === 0}
				<div class="mt-4 flex flex-col items-center rounded-xl border border-dashed border-ink-800 px-6 py-16 text-center">
					<span class="flex size-11 items-center justify-center rounded-xl bg-primary-400/10 text-primary-400">
						<Icon name="book" class="size-5" />
					</span>
					<p class="mt-4 text-sm font-medium text-ink-200">{f.q ? 'No articles match your search' : 'No articles here yet'}</p>
					<p class="mt-1 text-sm text-ink-500">
						{isStaff ? 'Write the first one with “New article”.' : 'Check back soon.'}
					</p>
				</div>
			{:else}
				<ul class="mt-4 grid gap-3 lg:grid-cols-2">
					{#each data.articles as a (a.id)}
						<li>
							<a
								href="/knowledge-base/{a.id}"
								class="group flex h-full flex-col rounded-xl border border-ink-800 bg-ink-900/60 p-4 transition hover:border-ink-700 hover:bg-ink-900 focus-visible:ring-2 focus-visible:ring-primary-400/50 focus-visible:outline-none"
							>
								<div class="flex flex-wrap items-center gap-1.5">
									{#if a.collection}<span class="text-xs text-ink-500">{a.collection.name}</span>{/if}
									{#if a.status === 'draft'}
										<span class="rounded-full border border-dashed border-ink-500/70 px-1.5 py-px text-[10px] font-medium text-ink-200">Draft</span>
									{/if}
									{#if isStaff && a.visibility === 'staff'}
										<span class="inline-flex items-center gap-1 rounded-full border border-ink-700 px-1.5 py-px text-[10px] font-medium text-ink-400">
											<Icon name="lock" class="size-2.5" /> Staff only
										</span>
									{/if}
								</div>
								<h3 class="mt-1.5 text-[15px] font-semibold text-ink-100 group-hover:text-primary-200">{a.title}</h3>
								{#if a.excerpt}<p class="mt-1 line-clamp-2 text-sm text-ink-400">{a.excerpt}</p>{/if}
								<div class="mt-auto flex items-center gap-2 pt-3 text-xs text-ink-500">
									{#if a.updatedBy ?? a.author}
										{@const who = (a.updatedBy ?? a.author)!}
										<Avatar name={who.name} src={who.avatarUrl} size={18} />
										<span>{who.name}</span>
										<span>·</span>
									{/if}
									<span>Updated {timeAgo(a.updatedAt)}</span>
								</div>
							</a>
						</li>
					{/each}
				</ul>
			{/if}
		</section>
	</div>
</div>

<Modal bind:open={colOpen} title={editingCol ? 'Edit collection' : 'New collection'} description="Collections group related articles together.">
	<form onsubmit={saveCollection} class="space-y-4">
		{#if colError}
			<p role="alert" class="rounded-lg border border-primary-400/25 bg-primary-400/10 px-3 py-2 text-sm text-primary-200">{colError}</p>
		{/if}
		<div>
			<label for="col-name" class="mb-1.5 block text-[13px] font-medium text-ink-200">Name</label>
			<input id="col-name" bind:value={colName} maxlength="60" required placeholder="e.g. Getting started" class="field" />
		</div>
		<div>
			<label for="col-desc" class="mb-1.5 block text-[13px] font-medium text-ink-200">Description <span class="font-normal text-ink-500">(optional)</span></label>
			<input id="col-desc" bind:value={colDesc} maxlength="200" placeholder="What's in this collection" class="field" />
		</div>
		<div class="flex items-center justify-between gap-2 pt-1">
			{#if editingCol}
				<button type="button" onclick={() => editingCol && deleteCollection(editingCol)} class="text-sm text-ink-400 hover:text-primary-300">Delete collection</button>
			{:else}
				<span></span>
			{/if}
			<div class="flex gap-2">
				<button type="button" onclick={() => (colOpen = false)} class="h-9 rounded-lg px-3.5 text-sm text-ink-400 transition hover:bg-ink-850 hover:text-ink-100">Cancel</button>
				<button type="submit" disabled={colSaving} class="inline-flex h-9 items-center rounded-lg bg-primary-400 px-3.5 text-sm font-semibold text-primary-950 transition hover:bg-primary-300 disabled:opacity-70">
					{colSaving ? 'Saving…' : editingCol ? 'Save' : 'Create'}
				</button>
			</div>
		</div>
	</form>
</Modal>
