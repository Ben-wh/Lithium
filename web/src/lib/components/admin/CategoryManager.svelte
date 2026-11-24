<script lang="ts">
	import { onMount } from 'svelte';
	import { adminCategories, ApiError, type Category } from '$lib/api';
	import Icon from '$lib/components/Icon.svelte';
	import CategoryTag from '$lib/components/CategoryTag.svelte';
	import Modal from '$lib/components/Modal.svelte';
	import { categoryColors, nextColor, type CategoryColor } from '$lib/categories';

	const NAME_MAX = 60;
	const DESCRIPTION_MAX = 200;

	let categories = $state<Category[]>([]);
	let loading = $state(true);
	let loadError = $state('');
	let notice = $state<{ kind: 'success' | 'error'; text: string } | null>(null);

	const errorText = (err: unknown) =>
		err instanceof ApiError && err.status < 500
			? err.message.charAt(0).toUpperCase() + err.message.slice(1) + '.'
			: 'Something went wrong. Please try again.';

	async function load() {
		try {
			categories = await adminCategories.list();
			loadError = '';
		} catch (err) {
			loadError = errorText(err);
		} finally {
			loading = false;
		}
	}
	onMount(load);

	// --- Add / edit ---
	let modalOpen = $state(false);
	let editing = $state<Category | null>(null);
	let name = $state('');
	let description = $state('');
	let color = $state<CategoryColor>('gray');
	let formError = $state('');
	let saving = $state(false);

	function openAdd() {
		editing = null;
		name = description = formError = '';
		color = nextColor(categories.filter((c) => !c.archived).map((c) => c.color));
		modalOpen = true;
	}

	function openEdit(c: Category) {
		editing = c;
		name = c.name;
		description = c.description;
		color = c.color;
		formError = '';
		modalOpen = true;
	}

	async function save(e: SubmitEvent) {
		e.preventDefault();
		if (!name.trim()) {
			formError = 'Give the category a name.';
			return;
		}
		saving = true;
		formError = '';
		try {
			if (editing) {
				await adminCategories.update(editing.id, { name, description, color });
				notice = { kind: 'success', text: `Saved “${name.trim()}”.` };
			} else {
				await adminCategories.create({ name, description, color });
				notice = { kind: 'success', text: `Added “${name.trim()}”. Users can pick it when raising a ticket.` };
			}
			modalOpen = false;
			await load();
		} catch (err) {
			formError = errorText(err);
		} finally {
			saving = false;
		}
	}

	// --- Archive / restore / delete ---
	async function setArchived(c: Category, archived: boolean) {
		try {
			await adminCategories.update(c.id, { archived });
			notice = {
				kind: 'success',
				text: archived
					? `Archived “${c.name}”. It's hidden from new tickets but stays on existing ones.`
					: `Restored “${c.name}”.`
			};
		} catch (err) {
			notice = { kind: 'error', text: errorText(err) };
		}
		await load();
	}

	async function remove(c: Category) {
		if (!confirm(`Delete the “${c.name}” category? This can't be undone.`)) return;
		try {
			await adminCategories.remove(c.id);
			notice = { kind: 'success', text: `Deleted “${c.name}”.` };
		} catch (err) {
			notice = { kind: 'error', text: errorText(err) };
		}
		await load();
	}

	const iconBtn =
		'flex size-8 items-center justify-center rounded-md text-ink-500 transition hover:bg-ink-850 hover:text-ink-100 focus-visible:ring-2 focus-visible:ring-primary-400/40 focus-visible:outline-none disabled:pointer-events-none disabled:opacity-30';
	const labelClass = 'mb-1.5 block text-[13px] font-medium text-ink-200';
</script>

<div class="mt-5 rounded-xl border border-ink-800 bg-ink-900/60">
	<div class="flex flex-wrap items-start justify-between gap-3 border-b border-ink-800 px-5 py-4">
		<div>
			<h3 class="text-sm font-semibold">Categories</h3>
			<p class="mt-0.5 text-sm text-ink-400">Users choose one when they raise a ticket, so it reaches the right place.</p>
		</div>
		<button
			type="button"
			onclick={openAdd}
			class="inline-flex h-8 items-center gap-1.5 rounded-lg bg-primary-400 px-3 text-[13px] font-semibold text-primary-950 transition hover:bg-primary-300"
		>
			<Icon name="plus" class="size-4" /> Add category
		</button>
	</div>

	{#if notice}
		<div
			role="status"
			class={[
				'mx-5 mt-4 flex items-center gap-2.5 rounded-lg border px-3 py-2 text-sm',
				notice.kind === 'success' ? 'border-ink-700 bg-ink-900 text-ink-200' : 'border-primary-400/25 bg-primary-400/10 text-primary-200'
			]}
		>
			<Icon name={notice.kind === 'success' ? 'check' : 'close'} class="size-4 shrink-0 text-primary-300" />
			<span class="flex-1">{notice.text}</span>
			<button type="button" onclick={() => (notice = null)} aria-label="Dismiss" class="text-ink-500 hover:text-ink-100">
				<Icon name="close" class="size-4" />
			</button>
		</div>
	{/if}

	{#if loadError}
		<p class="px-5 py-10 text-center text-sm text-danger-200">{loadError}</p>
	{:else if loading}
		<p class="px-5 py-10 text-center text-sm text-ink-500">Loading categories…</p>
	{:else if categories.length === 0}
		<div class="flex flex-col items-center px-6 py-12 text-center">
			<span class="flex size-11 items-center justify-center rounded-xl bg-primary-400/10 text-primary-400">
				<Icon name="tag" class="size-5" />
			</span>
			<p class="mt-4 text-sm font-medium text-ink-200">No categories yet</p>
			<p class="mt-1 max-w-sm text-sm text-ink-500">
				Add categories like “Hardware”, “Access request” or “Billing”. Until then, tickets are raised without one.
			</p>
		</div>
	{:else}
		<ul class="divide-y divide-ink-800 p-1.5">
			{#each categories as c (c.id)}
				<li class={['flex items-center gap-3 rounded-lg px-3.5 py-3', c.archived && 'opacity-60']}>
					<div class="min-w-0 flex-1">
						<p class="flex items-center gap-2 text-sm font-medium text-ink-100">
							<CategoryTag category={c} class="max-w-60 text-xs" />
							{#if c.archived}
								<span class="shrink-0 rounded-full border border-ink-700 px-1.5 py-px text-[11px] font-normal text-ink-400">Archived</span>
							{/if}
						</p>
						{#if c.description}
							<p class="truncate text-xs text-ink-500">{c.description}</p>
						{/if}
					</div>
					<span class="hidden shrink-0 text-xs text-ink-500 tabular-nums sm:block">
						{c.ticketCount.toLocaleString()} {c.ticketCount === 1 ? 'ticket' : 'tickets'}
					</span>
					<div class="flex shrink-0 items-center">
						<button type="button" onclick={() => openEdit(c)} class={iconBtn} aria-label="Edit {c.name}" title="Edit">
							<Icon name="pencil" class="size-4" />
						</button>
						<button
							type="button"
							onclick={() => setArchived(c, !c.archived)}
							class={iconBtn}
							aria-label="{c.archived ? 'Restore' : 'Archive'} {c.name}"
							title={c.archived ? 'Restore' : 'Archive (hide from new tickets)'}
						>
							<Icon name={c.archived ? 'refresh' : 'archive'} class="size-4" />
						</button>
						<button
							type="button"
							onclick={() => remove(c)}
							disabled={c.ticketCount > 0}
							class={iconBtn}
							aria-label="Delete {c.name}"
							title={c.ticketCount > 0 ? 'Used by tickets — archive it instead' : 'Delete'}
						>
							<Icon name="trash" class="size-4" />
						</button>
					</div>
				</li>
			{/each}
		</ul>
	{/if}
</div>

<Modal
	bind:open={modalOpen}
	title={editing ? 'Edit category' : 'Add category'}
	description={editing && editing.ticketCount > 0
		? `Changes show on the ${editing.ticketCount.toLocaleString()} ${editing.ticketCount === 1 ? 'ticket' : 'tickets'} already using it.`
		: 'Shown to users when they raise a ticket.'}
>
	<form onsubmit={save} novalidate class="space-y-4">
		{#if formError}
			<p role="alert" class="rounded-lg border border-danger-400/25 bg-danger-400/10 px-3 py-2 text-sm text-danger-200">{formError}</p>
		{/if}
		<div>
			<label for="category-name" class={labelClass}>Name</label>
			<input id="category-name" bind:value={name} maxlength={NAME_MAX} required placeholder="e.g. Hardware" class="field" />
		</div>
		<div>
			<label for="category-description" class={labelClass}>
				Description <span class="font-normal text-ink-500">(optional)</span>
			</label>
			<textarea
				id="category-description"
				bind:value={description}
				maxlength={DESCRIPTION_MAX}
				rows="2"
				placeholder="Helps users pick the right one, e.g. “Laptops, monitors, phones”"
				class="field h-auto resize-none py-2"
			></textarea>
		</div>
		<fieldset>
			<legend class={labelClass}>Colour</legend>
			<div class="flex flex-wrap gap-2">
				{#each categoryColors as c (c.value)}
					<label
						title={c.label}
						class={[
							`tag-${c.value} relative flex size-8 cursor-pointer items-center justify-center rounded-full transition`,
							'has-focus-visible:ring-2 has-focus-visible:ring-primary-400/50',
							color === c.value ? 'ring-2 ring-ink-100 ring-offset-2 ring-offset-ink-900' : 'hover:scale-110'
						]}
					>
						<input type="radio" name="category-color" value={c.value} bind:group={color} class="sr-only" aria-label={c.label} />
						<span class="tag-dot size-6 rounded-full"></span>
					</label>
				{/each}
			</div>
			<div class="mt-3 flex items-center gap-2 text-xs text-ink-500">
				Preview <CategoryTag category={{ name: name.trim() || 'Category', color }} />
			</div>
		</fieldset>
		<div class="flex justify-end gap-2 pt-2">
			<button type="button" onclick={() => (modalOpen = false)} class="h-9 rounded-lg px-3.5 text-sm text-ink-400 transition hover:bg-ink-850 hover:text-ink-100">
				Cancel
			</button>
			<button
				type="submit"
				disabled={saving}
				class="inline-flex h-9 items-center rounded-lg bg-primary-400 px-3.5 text-sm font-semibold text-primary-950 transition hover:bg-primary-300 disabled:opacity-70"
			>
				{saving ? 'Saving…' : editing ? 'Save' : 'Add category'}
			</button>
		</div>
	</form>
</Modal>
