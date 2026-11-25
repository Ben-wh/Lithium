<script lang="ts">
	import { untrack } from 'svelte';
	import { beforeNavigate, goto } from '$app/navigation';
	import { ApiError, kb, type KBArticle, type KBStatus, type KBVisibility } from '$lib/api';
	import Dropdown, { type DropdownOption } from '$lib/components/Dropdown.svelte';
	import Icon from '$lib/components/Icon.svelte';
	import MarkdownEditor from '$lib/components/kb/MarkdownEditor.svelte';
	import { timeAgo } from '$lib/format';

	let { data } = $props();

	// The saved version, and the working copy being edited. Reset whenever a
	// different article is opened in this editor.
	const shownTitle = (a: KBArticle) => (a.title === 'Untitled article' && !a.body ? '' : a.title);
	let saved = $state<KBArticle>(untrack(() => data.article));
	let title = $state('');
	let body = $state('');
	let collectionId = $state<number | ''>('');
	let visibility = $state<KBVisibility>('staff');

	$effect.pre(() => {
		const a = data.article;
		untrack(() => {
			saved = a;
			title = shownTitle(a);
			body = a.body ?? '';
			collectionId = a.collection?.id ?? '';
			visibility = a.visibility;
		});
	});

	let saving = $state(false);
	let error = $state('');
	let leaving = false;

	const dirty = $derived(
		title.trim() !== shownTitle(saved) ||
			body !== (saved.body ?? '') ||
			(collectionId || null) !== (saved.collection?.id ?? null) ||
			visibility !== saved.visibility
	);

	beforeNavigate(({ cancel, type }) => {
		if (!dirty || leaving) return;
		if (type === 'leave') cancel();
		else if (!confirm('Leave without saving? Your changes will be lost.')) cancel();
	});

	async function save(status: KBStatus = saved.status) {
		if (saving) return;
		error = '';
		if (!title.trim()) {
			error = 'Give the article a title.';
			return;
		}
		saving = true;
		try {
			saved = await kb.save(saved.id, {
				title,
				body,
				collectionId: collectionId === '' ? 0 : collectionId,
				visibility,
				status
			});
			if (status === 'published' && saved.status === 'published') {
				leaving = true;
				await goto(`/knowledge-base/${saved.id}`);
			}
		} catch (err) {
			error = err instanceof ApiError && err.status < 500 ? err.message.charAt(0).toUpperCase() + err.message.slice(1) + '.' : "Couldn't save. Please try again.";
		} finally {
			saving = false;
		}
	}

	async function remove() {
		if (!confirm(`Delete “${saved.title}”? This also deletes its images and can't be undone.`)) return;
		await kb.remove(saved.id);
		leaving = true;
		await goto('/knowledge-base', { invalidateAll: true });
	}

	const collectionOptions = $derived<DropdownOption<number | ''>[]>([
		{ value: '', label: 'No collection' },
		...data.collections.map((c) => ({ value: c.id, label: c.name }))
	]);
	// Only admins can make an article admin-only (agents can't open one).
	const isAdmin = $derived(data.user?.role === 'admin');
	const visibilityOptions = $derived<DropdownOption<KBVisibility>[]>([
		...(isAdmin ? [{ value: 'admins' as const, label: 'Admins only' }] : []),
		{ value: 'staff', label: 'Staff only' },
		{ value: 'everyone', label: 'Everyone' }
	]);
	const visibilityHints: Record<KBVisibility, string> = {
		admins: 'Only admins can read it',
		staff: 'Only admins and agents can read it',
		everyone: 'Users can read it too, once published'
	};
	const visibilityIcons = { admins: 'shield', staff: 'lock', everyone: 'users' } as const;
</script>

<svelte:head>
	<title>Editing {title || 'article'} · Knowledge base · Lithium</title>
</svelte:head>

<div class="mx-auto max-w-7xl px-4 py-8 sm:px-6">
	<!-- Top bar -->
	<div class="flex flex-wrap items-center justify-between gap-3">
		<a href="/knowledge-base/{saved.id}" class="inline-flex items-center gap-1.5 text-sm text-ink-400 transition hover:text-ink-100">
			<Icon name="arrowLeft" class="size-4" /> {saved.status === 'draft' ? 'Preview article' : 'Back to article'}
		</a>
		<div class="flex flex-wrap items-center gap-2">
			<span class="text-xs text-ink-500" aria-live="polite">
				{#if saving}Saving…{:else if dirty}Unsaved changes{:else}Saved {timeAgo(saved.updatedAt)}{/if}
			</span>
			{#if saved.status === 'draft'}
				<button type="button" onclick={() => save('draft')} disabled={saving} class="h-9 rounded-lg border border-ink-700 px-3.5 text-sm text-ink-200 transition hover:border-ink-500 disabled:opacity-60">
					Save draft
				</button>
				<button type="button" onclick={() => save('published')} disabled={saving} class="inline-flex h-9 items-center gap-1.5 rounded-lg bg-primary-400 px-3.5 text-sm font-semibold text-primary-950 transition hover:bg-primary-300 disabled:opacity-60">
					<Icon name="check" class="size-4" /> Publish
				</button>
			{:else}
				<button type="button" onclick={() => save('draft')} disabled={saving} class="h-9 rounded-lg px-3 text-sm text-ink-400 transition hover:bg-ink-850 hover:text-ink-100 disabled:opacity-60">
					Unpublish
				</button>
				<button type="button" onclick={() => save('published')} disabled={saving || !dirty} class="inline-flex h-9 items-center gap-1.5 rounded-lg bg-primary-400 px-3.5 text-sm font-semibold text-primary-950 transition hover:bg-primary-300 disabled:opacity-50">
					Save changes
				</button>
			{/if}
		</div>
	</div>

	{#if error}
		<p role="alert" class="mt-4 rounded-lg border border-danger-400/25 bg-danger-400/10 px-3 py-2 text-sm text-danger-200">{error}</p>
	{/if}

	<!-- Title and details -->
	<div class="mt-6">
		<label for="kb-title" class="sr-only">Title</label>
		<!-- svelte-ignore a11y_autofocus -->
		<input
			id="kb-title"
			bind:value={title}
			maxlength="150"
			autofocus={!title}
			placeholder="Article title"
			class="w-full bg-transparent text-3xl font-semibold tracking-tight text-ink-100 outline-none placeholder:text-ink-500"
		/>
		<div class="mt-4 flex flex-wrap items-end gap-3">
			<div class="w-56">
				<label for="kb-collection" class="mb-1 block text-xs text-ink-500">Collection</label>
				<Dropdown id="kb-collection" size="sm" bind:value={collectionId} options={collectionOptions}>
					{#snippet item(opt)}
						<Icon name={opt.value === '' ? 'inbox' : 'tag'} class="size-4 shrink-0 text-ink-500" />
						<span class={['truncate text-[13px]', opt.value === '' ? 'text-ink-400' : 'text-ink-100']}>{opt.label}</span>
					{/snippet}
				</Dropdown>
			</div>
			<div class="w-56">
				<label for="kb-visibility" class="mb-1 block text-xs text-ink-500">Who can read it</label>
				<Dropdown id="kb-visibility" size="sm" menu="wide" bind:value={visibility} options={visibilityOptions}>
					{#snippet item(opt)}
						<Icon name={visibilityIcons[opt.value]} class="size-4 shrink-0 text-ink-500" />
						<span class="min-w-0">
							<span class="block text-[13px] text-ink-100">{opt.label}</span>
							<span class="block text-xs text-ink-500">{visibilityHints[opt.value]}</span>
						</span>
					{/snippet}
					{#snippet selected(opt)}
						<Icon name={visibilityIcons[opt.value]} class="size-4 shrink-0 text-ink-500" />
						<span class="text-ink-100">{opt.label}</span>
					{/snippet}
				</Dropdown>
			</div>
			<span class={['mb-2 rounded-full border px-2 py-0.5 text-xs font-medium', saved.status === 'published' ? 'border-ink-700 text-ink-200' : 'border-dashed border-ink-500/70 text-ink-200']}>
				{saved.status === 'published' ? 'Published' : 'Draft'}
			</span>
		</div>
	</div>

	<div class="mt-5">
		<MarkdownEditor bind:value={body} articleId={saved.id} onsave={() => save()} />
	</div>

	<div class="mt-6 flex justify-end">
		<button type="button" onclick={remove} class="inline-flex items-center gap-1.5 text-sm text-ink-500 transition hover:text-primary-300">
			<Icon name="trash" class="size-4" /> Delete article
		</button>
	</div>
</div>
