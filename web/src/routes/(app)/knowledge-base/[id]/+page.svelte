<script lang="ts">
	import Avatar from '$lib/components/Avatar.svelte';
	import Icon from '$lib/components/Icon.svelte';
	import { dateTimeFmt, timeAgo } from '$lib/format';
	import { renderMarkdown } from '$lib/markdown';

	let { data } = $props();
	const a = $derived(data.article);
	const isStaff = $derived(data.user?.role === 'admin' || data.user?.role === 'agent');
	const html = $derived(renderMarkdown(a.body ?? ''));
	const editor = $derived(a.updatedBy ?? a.author);
</script>

<svelte:head>
	<title>{a.title} · Knowledge base · Lithium</title>
</svelte:head>

<div class="mx-auto max-w-4xl px-4 py-10 sm:px-6">
	<nav aria-label="Breadcrumb" class="flex flex-wrap items-center gap-1.5 text-sm text-ink-400">
		<a href="/knowledge-base" class="hover:text-ink-100">Knowledge base</a>
		{#if a.collection}
			<Icon name="chevron" class="size-3.5 -rotate-90 text-ink-500" />
			<a href="/knowledge-base?collection={a.collection.id}" class="hover:text-ink-100">{a.collection.name}</a>
		{/if}
	</nav>

	<header class="mt-4 flex flex-wrap items-start justify-between gap-4">
		<div class="min-w-0">
			{#if isStaff && (a.status === 'draft' || a.visibility === 'staff')}
				<div class="mb-2 flex flex-wrap gap-1.5">
					{#if a.status === 'draft'}
						<span class="rounded-full border border-dashed border-ink-500/70 px-2 py-0.5 text-xs font-medium text-ink-200">Draft, not published</span>
					{/if}
					{#if a.visibility === 'staff'}
						<span class="inline-flex items-center gap-1 rounded-full border border-ink-700 px-2 py-0.5 text-xs font-medium text-ink-400">
							<Icon name="lock" class="size-3" /> Staff only
						</span>
					{/if}
				</div>
			{/if}
			<h1 class="text-3xl font-semibold tracking-tight break-words">{a.title}</h1>
			{#if editor}
				<p class="mt-3 flex items-center gap-2 text-sm text-ink-400">
					<Avatar name={editor.name} src={editor.avatarUrl} size={22} />
					<span>
						{a.updatedBy && a.author && a.updatedBy.id !== a.author.id ? `Updated by ${editor.name}` : editor.name}
						· <time datetime={a.updatedAt} title={dateTimeFmt.format(new Date(a.updatedAt))}>{timeAgo(a.updatedAt)}</time>
					</span>
				</p>
			{/if}
		</div>
		{#if isStaff}
			<a
				href="/knowledge-base/{a.id}/edit"
				class="inline-flex h-9 shrink-0 items-center gap-1.5 rounded-lg border border-ink-700 px-3.5 text-sm text-ink-200 transition hover:border-primary-400/40 hover:text-primary-200"
			>
				<Icon name="pencil" class="size-4" /> Edit
			</a>
		{/if}
	</header>

	<article class="mt-8 rounded-2xl border border-ink-800 bg-ink-900/60 px-6 py-8 sm:px-10">
		{#if a.body?.trim()}
			<div class="kb-prose">{@html html}</div>
		{:else}
			<p class="text-sm text-ink-500">This article is empty.{#if isStaff} <a href="/knowledge-base/{a.id}/edit" class="text-primary-300 hover:text-primary-200">Start writing</a>{/if}</p>
		{/if}
	</article>
</div>
