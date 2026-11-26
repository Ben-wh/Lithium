<script lang="ts">
	import { onMount } from 'svelte';
	import {
		adminTemplates,
		ApiError,
		type DescriptionMode,
		type FieldType,
		type Priority,
		type TemplateField,
		type TicketTemplate
	} from '$lib/api';
	import { priorities } from '$lib/tickets';
	import Icon from '$lib/components/Icon.svelte';
	import Modal from '$lib/components/Modal.svelte';
	import { loadTemplates, templateStore } from './templates.svelte';

	const MAX_FIELDS = 25;

	const fieldTypes: { value: FieldType; label: string }[] = [
		{ value: 'text', label: 'Short text' },
		{ value: 'textarea', label: 'Long text' },
		{ value: 'number', label: 'Number' },
		{ value: 'select', label: 'Dropdown' },
		{ value: 'checkbox', label: 'Checkbox' },
		{ value: 'date', label: 'Date' }
	];
	const typeLabel = (t: FieldType) => fieldTypes.find((f) => f.value === t)?.label ?? t;

	const descriptionModes: { value: DescriptionMode; label: string; hint: string }[] = [
		{ value: 'required', label: 'Required', hint: 'People must describe the problem as well.' },
		{ value: 'optional', label: 'Optional', hint: 'Shown as “Anything else”, can be left blank.' },
		{ value: 'hidden', label: 'Hidden', hint: 'Only your fields are asked for.' }
	];

	onMount(loadTemplates);

	let notice = $state<{ kind: 'success' | 'error'; text: string } | null>(null);
	const errorText = (err: unknown) =>
		err instanceof ApiError && err.status < 500
			? err.message.charAt(0).toUpperCase() + err.message.slice(1) + '.'
			: 'Something went wrong. Please try again.';

	// --- Editor ---
	// Fields being edited carry a local key for the list, and options as text
	// (one per line) so typing feels natural.
	type DraftField = TemplateField & { key: number; optionsText: string };
	let nextKey = 0;
	const toDraft = (f: TemplateField): DraftField => ({ ...f, key: nextKey++, optionsText: (f.options ?? []).join('\n') });

	let editorOpen = $state(false);
	let editing = $state<TicketTemplate | null>(null);
	let name = $state('');
	let intro = $state('');
	let descriptionMode = $state<DescriptionMode>('required');
	let defaultPriority = $state<Priority | ''>('');
	let fields = $state<DraftField[]>([]);
	let formError = $state('');
	let saving = $state(false);

	function openNew() {
		editing = null;
		name = intro = formError = '';
		descriptionMode = 'required';
		defaultPriority = '';
		fields = [toDraft({ id: '', label: '', type: 'text', required: false, help: '' })];
		editorOpen = true;
	}

	function openEdit(t: TicketTemplate) {
		editing = t;
		name = t.name;
		intro = t.intro;
		descriptionMode = t.descriptionMode;
		defaultPriority = t.defaultPriority;
		fields = t.fields.map(toDraft);
		formError = '';
		editorOpen = true;
	}

	function addField() {
		fields.push(toDraft({ id: '', label: '', type: 'text', required: false, help: '' }));
		// Focus the new label once it's drawn.
		const key = fields[fields.length - 1].key;
		queueMicrotask(() => document.getElementById(`tf-label-${key}`)?.focus());
	}

	function move(i: number, by: -1 | 1) {
		const j = i + by;
		if (j < 0 || j >= fields.length) return;
		[fields[i], fields[j]] = [fields[j], fields[i]];
	}

	async function save(e: SubmitEvent) {
		e.preventDefault();
		if (!name.trim()) {
			formError = 'Give the template a name.';
			return;
		}
		const unlabelled = fields.findIndex((f) => !f.label.trim());
		if (unlabelled >= 0) {
			formError = `Field ${unlabelled + 1} needs a label.`;
			document.getElementById(`tf-label-${fields[unlabelled].key}`)?.focus();
			return;
		}
		saving = true;
		formError = '';
		const draft = {
			name,
			intro,
			descriptionMode,
			defaultPriority,
			fields: fields.map(({ key: _k, optionsText, ...f }) => ({
				...f,
				options: f.type === 'select' ? optionsText.split('\n').map((o) => o.trim()).filter(Boolean) : undefined
			}))
		};
		try {
			if (editing) {
				await adminTemplates.update(editing.id, draft);
				notice = { kind: 'success', text: `Saved “${name.trim()}”. New tickets use it straight away.` };
			} else {
				await adminTemplates.create(draft);
				notice = { kind: 'success', text: `Added “${name.trim()}”. Link it to a category above to use it.` };
			}
			editorOpen = false;
			await loadTemplates();
		} catch (err) {
			formError = errorText(err);
		} finally {
			saving = false;
		}
	}

	// --- Delete ---
	let deleting = $state<TicketTemplate | null>(null);
	async function confirmDelete() {
		const t = deleting;
		if (!t) return;
		deleting = null;
		try {
			await adminTemplates.remove(t.id);
			notice = { kind: 'success', text: `Deleted “${t.name}”. Existing tickets keep what was filled in.` };
			await loadTemplates();
		} catch (err) {
			notice = { kind: 'error', text: errorText(err) };
		}
	}

	const labelClass = 'mb-1.5 block text-[13px] font-medium text-ink-200';
	const iconBtn =
		'flex size-8 items-center justify-center rounded-md text-ink-500 transition hover:bg-ink-850 hover:text-ink-100 disabled:cursor-not-allowed disabled:opacity-30 disabled:hover:bg-transparent';
</script>

<div class="mt-6 rounded-xl border border-ink-800 bg-ink-900/60">
	<div class="flex items-center justify-between gap-3 border-b border-ink-800 px-5 py-4">
		<div>
			<h3 class="text-sm font-semibold">Ticket templates</h3>
			<p class="mt-0.5 text-sm text-ink-400">Extra questions for tickets in a category. Link a template to categories above.</p>
		</div>
		<button
			type="button"
			onclick={openNew}
			class="flex h-8 shrink-0 items-center gap-1.5 rounded-lg bg-primary-400 px-3 text-[13px] font-semibold text-primary-950 transition hover:bg-primary-300"
		>
			<Icon name="plus" class="size-4" /> New template
		</button>
	</div>

	{#if notice}
		<p
			role="status"
			class={[
				'mx-5 mt-4 rounded-lg border px-3 py-2 text-sm',
				notice.kind === 'success'
					? 'border-primary-400/25 bg-primary-400/10 text-primary-200'
					: 'border-danger-400/25 bg-danger-400/10 text-danger-200'
			]}
		>
			{notice.text}
		</p>
	{/if}

	{#if !templateStore.loaded}
		<p class="px-5 py-10 text-center text-sm text-ink-500">Loading templates…</p>
	{:else if templateStore.error}
		<p class="px-5 py-10 text-center text-sm text-danger-200">{templateStore.error}</p>
	{:else if templateStore.list.length === 0}
		<div class="flex flex-col items-center px-6 py-10 text-center">
			<span class="flex size-11 items-center justify-center rounded-xl bg-primary-400/10 text-primary-400">
				<Icon name="listUl" class="size-5" />
			</span>
			<p class="mt-4 text-sm font-medium text-ink-200">No templates yet</p>
			<p class="mt-1 max-w-sm text-sm text-ink-500">
				For example, a “New starter” template could ask for a start date, team and the equipment they need.
			</p>
		</div>
	{:else}
		<ul class="divide-y divide-ink-800 p-1.5">
			{#each templateStore.list as t (t.id)}
				<li class="flex items-center gap-3 rounded-lg px-3.5 py-3">
					<div class="min-w-0 flex-1">
						<p class="truncate text-sm font-medium text-ink-100">{t.name}</p>
						<p class="truncate text-xs text-ink-500">
							{t.fields.length}
							{t.fields.length === 1 ? 'field' : 'fields'}{#if t.fields.length}: {t.fields.map((f) => f.label).join(', ')}{/if}
						</p>
					</div>
					<span class={['hidden shrink-0 text-xs tabular-nums sm:block', t.categoryCount ? 'text-ink-400' : 'text-ink-600']}>
						{t.categoryCount ? `${t.categoryCount} ${t.categoryCount === 1 ? 'category' : 'categories'}` : 'Not linked'}
					</span>
					<div class="flex shrink-0 items-center">
						<button type="button" onclick={() => openEdit(t)} class={iconBtn} aria-label="Edit {t.name}" title="Edit">
							<Icon name="pencil" class="size-4" />
						</button>
						<button type="button" onclick={() => (deleting = t)} class={iconBtn} aria-label="Delete {t.name}" title="Delete">
							<Icon name="trash" class="size-4" />
						</button>
					</div>
				</li>
			{/each}
		</ul>
	{/if}
</div>

<!-- Editor -->
<Modal
	bind:open={editorOpen}
	size="lg"
	closeOnBackdrop={false}
	title={editing ? `Edit “${editing.name}”` : 'New ticket template'}
	description={editing
		? 'Changes apply to new tickets. Tickets already raised keep what was filled in.'
		: 'Choose the questions people answer when raising a ticket in linked categories.'}
>
	<form onsubmit={save} novalidate class="space-y-5">
		{#if formError}
			<p role="alert" class="rounded-lg border border-danger-400/25 bg-danger-400/10 px-3 py-2 text-sm text-danger-200">{formError}</p>
		{/if}

		<div class="grid gap-4 sm:grid-cols-[1fr_11rem]">
			<div>
				<label for="tpl-name" class={labelClass}>Name</label>
				<input id="tpl-name" bind:value={name} maxlength="60" placeholder="e.g. New starter" class="field" />
			</div>
			<div>
				<label for="tpl-priority" class={labelClass}>Starting priority</label>
				<select id="tpl-priority" bind:value={defaultPriority} class="field">
					<option value="">Normal (usual)</option>
					{#each priorities as p (p.value)}
						{#if p.value !== 'normal'}<option value={p.value}>{p.label}</option>{/if}
					{/each}
				</select>
			</div>
		</div>

		<div>
			<label for="tpl-intro" class={labelClass}>
				Instructions <span class="font-normal text-ink-500">(optional, shown above the form)</span>
			</label>
			<textarea
				id="tpl-intro"
				bind:value={intro}
				maxlength="1000"
				rows="2"
				placeholder="e.g. Raise this at least two weeks before their first day."
				class="field h-auto resize-y py-2"
			></textarea>
		</div>

		<fieldset>
			<legend class={labelClass}>Description box</legend>
			<div class="grid gap-2 sm:grid-cols-3">
				{#each descriptionModes as m (m.value)}
					<label
						class={[
							'cursor-pointer rounded-lg border px-3 py-2 transition has-focus-visible:ring-2 has-focus-visible:ring-primary-400/30',
							descriptionMode === m.value ? 'border-primary-400/50 bg-primary-400/10' : 'border-ink-800 hover:border-ink-600'
						]}
					>
						<input type="radio" name="tpl-desc" value={m.value} bind:group={descriptionMode} class="sr-only" />
						<span class="block text-sm font-medium text-ink-100">{m.label}</span>
						<span class="block text-xs text-ink-500">{m.hint}</span>
					</label>
				{/each}
			</div>
		</fieldset>

		<!-- Fields -->
		<div>
			<p class={labelClass}>Fields</p>
			{#if fields.length === 0}
				<p class="rounded-lg border border-dashed border-ink-700 px-3 py-4 text-center text-sm text-ink-500">
					No extra fields. People will just see the usual form{descriptionMode === 'hidden' ? ', so add a field or show the description box' : ''}.
				</p>
			{/if}
			<ol class="space-y-3">
				{#each fields as f, i (f.key)}
					<li class="rounded-lg border border-ink-800 bg-ink-950/40 p-3">
						<div class="flex items-start gap-2">
							<span class="mt-2 w-5 shrink-0 text-xs text-ink-500 tabular-nums">{i + 1}.</span>
							<div class="grid min-w-0 flex-1 gap-2 sm:grid-cols-[1fr_9rem]">
								<input
									id="tf-label-{f.key}"
									bind:value={f.label}
									maxlength="80"
									placeholder="Question, e.g. Start date"
									aria-label="Field {i + 1} label"
									class="field h-9 text-[13px]"
								/>
								<select bind:value={f.type} aria-label="Field {i + 1} type" class="field h-9 text-[13px]">
									{#each fieldTypes as t (t.value)}<option value={t.value}>{t.label}</option>{/each}
								</select>
								<input
									bind:value={f.help}
									maxlength="200"
									placeholder="Help text (optional)"
									aria-label="Field {i + 1} help text"
									class="field h-9 text-[13px] sm:col-span-2"
								/>
								{#if f.type === 'select'}
									<textarea
										bind:value={f.optionsText}
										rows="3"
										placeholder={'Choices, one per line\nLaptop\nMonitor'}
										aria-label="Field {i + 1} choices"
										class="field h-auto resize-y py-2 text-[13px] sm:col-span-2"
									></textarea>
								{/if}
								<label class="flex items-center gap-2 text-[13px] text-ink-300">
									<input type="checkbox" bind:checked={f.required} class="size-4 accent-primary-400" />
									{f.type === 'checkbox' ? 'Must be ticked' : 'Required'}
								</label>
							</div>
							<div class="flex shrink-0 flex-col">
								<button type="button" onclick={() => move(i, -1)} disabled={i === 0} class={iconBtn} aria-label="Move field {i + 1} up" title="Move up">
									<Icon name="chevron" class="size-4 rotate-180" />
								</button>
								<button type="button" onclick={() => move(i, 1)} disabled={i === fields.length - 1} class={iconBtn} aria-label="Move field {i + 1} down" title="Move down">
									<Icon name="chevron" class="size-4" />
								</button>
								<button type="button" onclick={() => fields.splice(i, 1)} class={iconBtn} aria-label="Remove field {i + 1}" title="Remove">
									<Icon name="trash" class="size-4" />
								</button>
							</div>
						</div>
					</li>
				{/each}
			</ol>
			<button
				type="button"
				onclick={addField}
				disabled={fields.length >= MAX_FIELDS}
				class="mt-3 flex h-8 items-center gap-1.5 rounded-lg border border-dashed border-ink-700 px-3 text-[13px] text-ink-300 transition hover:border-ink-500 hover:text-ink-100 disabled:opacity-40"
			>
				<Icon name="plus" class="size-4" /> Add field
			</button>
			{#if fields.some((f) => f.label.trim())}
				<p class="mt-2 text-xs text-ink-500">
					People will see: {fields.filter((f) => f.label.trim()).map((f) => `${f.label.trim()} (${typeLabel(f.type).toLowerCase()}${f.required ? ', required' : ''})`).join(' · ')}
				</p>
			{/if}
		</div>

		<div class="flex justify-end gap-2 border-t border-ink-800 pt-4">
			<button type="button" onclick={() => (editorOpen = false)} class="h-9 rounded-lg px-3.5 text-sm text-ink-400 transition hover:bg-ink-850 hover:text-ink-100">
				Cancel
			</button>
			<button
				type="submit"
				disabled={saving}
				class="inline-flex h-9 items-center rounded-lg bg-primary-400 px-3.5 text-sm font-semibold text-primary-950 transition hover:bg-primary-300 disabled:opacity-70"
			>
				{saving ? 'Saving…' : editing ? 'Save template' : 'Create template'}
			</button>
		</div>
	</form>
</Modal>

<!-- Delete -->
<Modal
	bind:open={() => deleting !== null, (v) => !v && (deleting = null)}
	title="Delete “{deleting?.name}”?"
	description={deleting?.categoryCount
		? `It's linked to ${deleting.categoryCount} ${deleting.categoryCount === 1 ? 'category' : 'categories'}, which will go back to the usual form.`
		: 'It isn’t linked to any categories.'}
>
	<p class="text-sm text-ink-300">Tickets already raised with it keep what was filled in.</p>
	<div class="mt-5 flex justify-end gap-2">
		<button type="button" onclick={() => (deleting = null)} class="h-9 rounded-lg px-3.5 text-sm text-ink-400 transition hover:bg-ink-850 hover:text-ink-100">
			Cancel
		</button>
		<button type="button" onclick={confirmDelete} class="inline-flex h-9 items-center rounded-lg bg-danger-600 px-3.5 text-sm font-semibold text-white transition hover:bg-danger-500">
			Delete template
		</button>
	</div>
</Modal>
