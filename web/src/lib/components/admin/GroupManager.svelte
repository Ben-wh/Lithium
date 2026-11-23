<script lang="ts">
	import { onMount } from 'svelte';
	import { page } from '$app/state';
	import {
		admin,
		adminCategories,
		adminGroups,
		ApiError,
		type AdminUser,
		type Category,
		type Group
	} from '$lib/api';
	import Avatar from '$lib/components/Avatar.svelte';
	import CategoryTag from '$lib/components/CategoryTag.svelte';
	import Icon from '$lib/components/Icon.svelte';
	import Modal from '$lib/components/Modal.svelte';

	// Create groups of agents and choose which ticket categories each covers.
	const enabled = $derived(page.data.settings?.agentGroups ?? false);

	let groups = $state<Group[]>([]);
	let agents = $state<AdminUser[]>([]);
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
			const [g, users, cats] = await Promise.all([adminGroups.list(), admin.listUsers(''), adminCategories.list()]);
			groups = g;
			agents = users.filter((u) => u.role === 'agent');
			categories = cats.filter((c) => !c.archived);
			loadError = '';
		} catch (err) {
			loadError = errorText(err);
		} finally {
			loading = false;
		}
	}
	onMount(load);

	// --- Editor ---
	let open = $state(false);
	let editing = $state<Group | null>(null);
	let name = $state('');
	let description = $state('');
	let memberIds = $state<number[]>([]);
	let categoryIds = $state<number[]>([]);
	let formError = $state('');
	let saving = $state(false);

	function openEditor(g: Group | null) {
		editing = g;
		name = g?.name ?? '';
		description = g?.description ?? '';
		memberIds = g?.members.map((m) => m.id) ?? [];
		categoryIds = g?.categories.map((c) => c.id) ?? [];
		formError = '';
		open = true;
	}

	const toggle = (list: number[], id: number) => (list.includes(id) ? list.filter((x) => x !== id) : [...list, id]);

	async function save(e: SubmitEvent) {
		e.preventDefault();
		if (!name.trim()) {
			formError = 'Give the group a name.';
			return;
		}
		saving = true;
		formError = '';
		try {
			const input = { name, description, memberIds, categoryIds };
			if (editing) await adminGroups.update(editing.id, input);
			else await adminGroups.create(input);
			notice = { kind: 'success', text: `${editing ? 'Saved' : 'Created'} “${name.trim()}”.` };
			open = false;
			await load();
		} catch (err) {
			formError = errorText(err);
		} finally {
			saving = false;
		}
	}

	async function remove(g: Group) {
		if (!confirm(`Delete the “${g.name}” group? Its agents will lose access to its categories (unless another group covers them).`)) return;
		try {
			await adminGroups.remove(g.id);
			notice = { kind: 'success', text: `Deleted “${g.name}”.` };
		} catch (err) {
			notice = { kind: 'error', text: errorText(err) };
		}
		await load();
	}

	// Archived categories still covered by a group are kept in the editor so saving doesn't drop them.
	const editorCategories = $derived.by(() => {
		const extra = (editing?.categories ?? []).filter((c) => !categories.some((x) => x.id === c.id));
		return [...categories, ...extra.map((c) => ({ ...c, description: '', archived: true, ticketCount: 0, createdAt: '' }))];
	});

	const iconBtn =
		'flex size-8 items-center justify-center rounded-md text-ink-500 transition hover:bg-ink-850 hover:text-ink-100';
	const labelClass = 'mb-1.5 block text-[13px] font-medium text-ink-200';
	const checkbox =
		'size-4 shrink-0 cursor-pointer rounded border-ink-700 accent-[var(--color-primary-400)]';
</script>

<div class="mt-6">
	<div class="flex flex-wrap items-start justify-between gap-4">
		<p class="max-w-2xl text-sm text-ink-400">
			Group agents by team and choose which ticket categories each group handles. When groups are on, agents only
			see tickets in their groups' categories, plus uncategorised tickets and ones assigned to them. Admins always see
			everything.
		</p>
		<button
			type="button"
			onclick={() => openEditor(null)}
			class="inline-flex h-9 items-center gap-1.5 rounded-lg bg-primary-400 px-3.5 text-sm font-semibold text-primary-950 transition hover:bg-primary-300"
		>
			<Icon name="plus" class="size-4" /> New group
		</button>
	</div>

	{#if !enabled}
		<div class="mt-4 flex flex-wrap items-center gap-2.5 rounded-lg border border-ink-700 bg-ink-900 px-3.5 py-2.5 text-sm text-ink-200">
			<Icon name="users" class="size-4 shrink-0 text-ink-500" />
			<span class="flex-1">Groups are turned off, so every agent can see all tickets. You can set groups up now and turn them on when ready.</span>
			<a href="?tab=settings&section=users" class="font-medium text-primary-300 hover:text-primary-200">Turn on in settings</a>
		</div>
	{/if}

	{#if notice}
		<div
			role="status"
			class={[
				'mt-4 flex items-center gap-2.5 rounded-lg border px-3.5 py-2.5 text-sm',
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
		<p class="mt-6 text-center text-sm text-primary-200">{loadError}</p>
	{:else if loading}
		<p class="mt-6 text-center text-sm text-ink-500">Loading groups…</p>
	{:else if groups.length === 0}
		<div class="mt-4 flex flex-col items-center rounded-xl border border-dashed border-ink-800 px-6 py-14 text-center">
			<span class="flex size-11 items-center justify-center rounded-xl bg-primary-400/10 text-primary-400">
				<Icon name="users" class="size-5" />
			</span>
			<p class="mt-4 text-sm font-medium text-ink-200">No groups yet</p>
			<p class="mt-1 max-w-sm text-sm text-ink-500">Create a group like “Hardware team” and give it the Hardware category.</p>
		</div>
	{:else}
		<ul class="mt-4 grid gap-3 md:grid-cols-2">
			{#each groups as g (g.id)}
				<li class="rounded-xl border border-ink-800 bg-ink-900/60 p-4">
					<div class="flex items-start gap-3">
						<div class="min-w-0 flex-1">
							<p class="truncate text-sm font-semibold text-ink-100">{g.name}</p>
							{#if g.description}<p class="mt-0.5 text-xs text-ink-500">{g.description}</p>{/if}
						</div>
						<button type="button" onclick={() => openEditor(g)} class={iconBtn} aria-label="Edit {g.name}" title="Edit">
							<Icon name="pencil" class="size-4" />
						</button>
						<button type="button" onclick={() => remove(g)} class={iconBtn} aria-label="Delete {g.name}" title="Delete">
							<Icon name="trash" class="size-4" />
						</button>
					</div>

					<p class="mt-3 text-[11px] font-medium tracking-wide text-ink-500 uppercase">Categories</p>
					<div class="mt-1.5 flex flex-wrap gap-1.5">
						{#each g.categories as c (c.id)}
							<CategoryTag category={c} />
						{:else}
							<span class="text-xs text-ink-500">None yet, so these agents only see uncategorised tickets.</span>
						{/each}
					</div>

					<p class="mt-3 text-[11px] font-medium tracking-wide text-ink-500 uppercase">
						Agents{#if g.members.length}<span class="ml-1 normal-case">· {g.members.length}</span>{/if}
					</p>
					<div class="mt-1.5 flex flex-wrap items-center gap-2">
						{#each g.members as m (m.id)}
							<span class="flex items-center gap-1.5 rounded-full border border-ink-800 py-0.5 pr-2.5 pl-0.5 text-xs text-ink-200">
								<Avatar name={m.name} src={m.avatarUrl} size={20} />
								{m.name}
							</span>
						{:else}
							<span class="text-xs text-ink-500">No agents yet.</span>
						{/each}
					</div>
				</li>
			{/each}
		</ul>
	{/if}
</div>

<Modal bind:open title={editing ? 'Edit group' : 'New group'} description="Agents in this group can see tickets in these categories.">
	<form onsubmit={save} novalidate class="space-y-5">
		{#if formError}
			<p role="alert" class="rounded-lg border border-primary-400/25 bg-primary-400/10 px-3 py-2 text-sm text-primary-200">{formError}</p>
		{/if}
		<div>
			<label for="group-name" class={labelClass}>Name</label>
			<input id="group-name" bind:value={name} maxlength="60" placeholder="e.g. Hardware team" class="field" />
		</div>
		<div>
			<label for="group-description" class={labelClass}>Description <span class="font-normal text-ink-500">(optional)</span></label>
			<input id="group-description" bind:value={description} maxlength="200" placeholder="What this team looks after" class="field" />
		</div>

		<fieldset>
			<legend class={labelClass}>Categories</legend>
			{#if editorCategories.length === 0}
				<p class="text-sm text-ink-500">No categories yet. Add some in Lithium settings → Tickets.</p>
			{:else}
				<div class="max-h-44 space-y-0.5 overflow-y-auto rounded-lg border border-ink-800 p-1.5">
					{#each editorCategories as c (c.id)}
						<label class="flex cursor-pointer items-center gap-2.5 rounded-md px-2 py-1.5 hover:bg-ink-850">
							<input type="checkbox" checked={categoryIds.includes(c.id)} onchange={() => (categoryIds = toggle(categoryIds, c.id))} class={checkbox} />
							<CategoryTag category={c} />
							{#if c.archived}<span class="text-xs text-ink-500">Archived</span>{/if}
						</label>
					{/each}
				</div>
			{/if}
		</fieldset>

		<fieldset>
			<legend class={labelClass}>Agents</legend>
			{#if agents.length === 0}
				<p class="text-sm text-ink-500">No agents yet. Give someone the Agent role in the Users tab.</p>
			{:else}
				<div class="max-h-52 space-y-0.5 overflow-y-auto rounded-lg border border-ink-800 p-1.5">
					{#each agents as a (a.id)}
						<label class="flex cursor-pointer items-center gap-2.5 rounded-md px-2 py-1.5 hover:bg-ink-850">
							<input type="checkbox" checked={memberIds.includes(a.id)} onchange={() => (memberIds = toggle(memberIds, a.id))} class={checkbox} />
							<Avatar name={a.name} src={a.avatarUrl} size={24} />
							<span class="min-w-0">
								<span class="block truncate text-sm text-ink-100">{a.name}</span>
								<span class="block truncate text-xs text-ink-500">{a.email}</span>
							</span>
						</label>
					{/each}
				</div>
				<p class="mt-1.5 text-xs text-ink-500">Only agents are listed: admins always see every ticket.</p>
			{/if}
		</fieldset>

		<div class="flex justify-end gap-2 pt-1">
			<button type="button" onclick={() => (open = false)} class="h-9 rounded-lg px-3.5 text-sm text-ink-400 transition hover:bg-ink-850 hover:text-ink-100">
				Cancel
			</button>
			<button
				type="submit"
				disabled={saving}
				class="inline-flex h-9 items-center rounded-lg bg-primary-400 px-3.5 text-sm font-semibold text-primary-950 transition hover:bg-primary-300 disabled:opacity-70"
			>
				{saving ? 'Saving…' : editing ? 'Save group' : 'Create group'}
			</button>
		</div>
	</form>
</Modal>
