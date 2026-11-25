<script lang="ts">
	import { beforeNavigate, goto } from '$app/navigation';
	import { ApiError, tickets, type Priority, type UserRef } from '$lib/api';
	import Icon from '$lib/components/Icon.svelte';
	import CategorySelect from '$lib/components/CategorySelect.svelte';
	import ImageUploader from '$lib/components/ImageUploader.svelte';
	import PriorityIcon from '$lib/components/PriorityIcon.svelte';
	import RequesterPicker from '$lib/components/RequesterPicker.svelte';
	import { priorities } from '$lib/tickets';

	let { data } = $props();

	// The (app) layout only renders for signed-in users.
	const user = $derived(data.user!);
	const isStaff = $derived(user.role === 'admin' || user.role === 'agent');
	const me = $derived<UserRef>({ id: user.id, name: user.name, email: user.email, avatarUrl: user.avatarUrl });

	// Set by admins in Lithium settings → Tickets.
	const SUBJECT_MAX = $derived(data.settings.subjectMaxLength);
	const DESCRIPTION_MAX = $derived(data.settings.descriptionMaxLength);

	let subject = $state('');
	let description = $state('');
	let priority = $state<Priority>('normal');
	let categoryId = $state<number | ''>('');
	let images = $state<File[]>([]);
	// Mandatory only while there's something to choose (the server applies the same rule).
	const categoryRequired = $derived(data.settings.requireCategory && data.categories.length > 0);
	let categoryMissing = $state(false);
	$effect(() => {
		if (categoryId !== '') categoryMissing = false;
	});
	// The chosen category's template adds fields and decides whether a
	// description is needed. Answers are kept by field id, so switching
	// category and back doesn't lose them.
	const template = $derived(
		categoryId === '' ? null : (data.categories.find((c) => c.id === categoryId)?.template ?? null)
	);
	const descriptionMode = $derived(template?.descriptionMode ?? 'required');
	let answers = $state<Record<string, string | boolean>>({});
	let missingField = $state<string | null>(null);

	// A template can suggest a priority, unless the requester already picked one.
	let priorityTouched = $state(false);
	$effect(() => {
		const suggested = template?.defaultPriority || 'normal';
		if (!priorityTouched) priority = suggested;
	});

	let requester = $derived<UserRef | null>(data.presetRequester);
	let error = $state('');
	let submitting = $state(false);
	let submitted = false;

	const dirty = $derived(
		subject.trim() !== '' ||
			description.trim() !== '' ||
			images.length > 0 ||
			Object.values(answers).some((v) => v !== '' && v !== false)
	);
	const forSomeoneElse = $derived(requester !== null && requester.id !== user.id);

	// Warn before throwing away a half-written ticket.
	beforeNavigate(({ cancel, type }) => {
		if (!dirty || submitted) return;
		if (type === 'leave') cancel(); // browser shows its own prompt
		else if (!confirm('Discard this ticket? What you wrote will be lost.')) cancel();
	});

	async function submit(e?: SubmitEvent) {
		e?.preventDefault();
		if (submitting) return;
		error = '';
		if (!subject.trim()) {
			error = 'Add a subject so the team knows what this is about.';
			return;
		}
		if (categoryRequired && categoryId === '') {
			categoryMissing = true;
			error = 'Choose a category so your ticket reaches the right team.';
			return;
		}
		for (const f of template?.fields ?? []) {
			const v = answers[f.id];
			const empty = f.type === 'checkbox' ? v !== true : typeof v !== 'string' || v.trim() === '';
			if (f.required && empty) {
				missingField = f.id;
				error = f.type === 'checkbox' ? `Tick “${f.label}” to continue.` : `Fill in “${f.label}”.`;
				document.getElementById(`field-${f.id}`)?.focus();
				return;
			}
		}
		missingField = null;
		if (descriptionMode === 'required' && !description.trim()) {
			error = 'Describe the problem or request.';
			return;
		}
		submitting = true;
		try {
			const t = await tickets.create({
				subject,
				description: descriptionMode === 'hidden' ? '' : description,
				priority,
				categoryId: categoryId === '' ? undefined : categoryId,
				requesterId: forSomeoneElse ? requester!.id : undefined,
				fields: template
					? Object.fromEntries(template.fields.filter((f) => f.id in answers).map((f) => [f.id, answers[f.id]]))
					: undefined
			}, data.settings.allowImageUploads ? images : []);
			submitted = true;
			await goto(`/tickets/${t.id}?created=1`, { replaceState: true });
		} catch (err) {
			error =
				err instanceof ApiError && err.status === 400
					? err.message.charAt(0).toUpperCase() + err.message.slice(1) + '.'
					: "Couldn't raise the ticket. Please try again.";
			submitting = false;
		}
	}

	function onKeydown(e: KeyboardEvent) {
		if (e.key === 'Enter' && (e.metaKey || e.ctrlKey)) {
			e.preventDefault();
			submit();
		}
	}

	const labelClass = 'mb-1.5 block text-[13px] font-medium text-ink-200';
	const counter = (n: number, max: number) => (n > max * 0.8 ? `${n.toLocaleString()} / ${max.toLocaleString()}` : '');
</script>

<svelte:head>
	<title>Raise a ticket · Lithium</title>
</svelte:head>

<div class="mx-auto max-w-5xl px-4 py-10 sm:px-6">
	<a href="/tickets" class="inline-flex items-center gap-1.5 text-sm text-ink-400 transition hover:text-ink-100">
		<Icon name="arrowLeft" class="size-4" /> Tickets
	</a>
	<h1 class="mt-3 text-2xl font-semibold tracking-tight">Raise a ticket</h1>
	<p class="mt-1 text-sm text-ink-400">
		{isStaff
			? 'Log a request for yourself or on behalf of someone else.'
			: "Tell us what's going on and the team will pick it up."}
	</p>

	<form onsubmit={submit} novalidate class="mt-8 grid gap-6 lg:grid-cols-[1fr_300px]">
		<!-- Main -->
		<div class="space-y-5 rounded-xl border border-ink-800 bg-ink-900/60 p-5 sm:p-6">
			{#if error}
				<p role="alert" class="rounded-lg border border-danger-400/25 bg-danger-400/10 px-3 py-2.5 text-sm text-danger-200">
					{error}
				</p>
			{/if}

			<div>
				<div class="flex items-baseline justify-between">
					<label for="subject" class={labelClass}>Subject</label>
					<span class={['text-xs tabular-nums', subject.length > SUBJECT_MAX ? 'text-primary-300' : 'text-ink-500']}>{counter(subject.length, SUBJECT_MAX)}</span>
				</div>
				<!-- svelte-ignore a11y_autofocus -->
				<input
					id="subject"
					bind:value={subject}
					maxlength={SUBJECT_MAX}
					required
					autofocus
					autocomplete="off"
					onkeydown={onKeydown}
					placeholder="A short summary, e.g. “Can't connect to the VPN”"
					class="field"
				/>
			</div>

			{#if data.categories.length > 0}
				<div>
					<label for="category" class={labelClass}>
						Category
						{#if categoryRequired}
							<span class="font-normal text-primary-300">Required</span>
						{:else}
							<span class="font-normal text-ink-500">(optional)</span>
						{/if}
					</label>
					<div class="sm:max-w-sm">
						<CategorySelect
							id="category"
							bind:value={categoryId}
							options={data.categories}
							allowNone={!categoryRequired}
							invalid={categoryMissing}
						/>
					</div>
				</div>
			{/if}

			{#if template}
				{#if template.intro}
					<div class="flex gap-2.5 rounded-lg border border-primary-400/25 bg-primary-400/5 px-3.5 py-3 text-sm text-ink-200">
						<Icon name="book" class="mt-0.5 size-4 shrink-0 text-primary-300" />
						<p class="whitespace-pre-line">{template.intro}</p>
					</div>
				{/if}
				{#each template.fields as f (f.id)}
					{@const invalid = missingField === f.id}
					<div>
						{#if f.type === 'checkbox'}
							<label class="flex cursor-pointer items-start gap-2.5 text-sm text-ink-200">
								<input
									id="field-{f.id}"
									type="checkbox"
									checked={answers[f.id] === true}
									onchange={(e) => {
										answers[f.id] = e.currentTarget.checked;
										if (missingField === f.id) missingField = null;
									}}
									class="mt-0.5 size-4 shrink-0 accent-primary-400"
								/>
								<span>
									<span class="font-medium">{f.label}</span>
									{#if f.required}<span class="text-primary-300"> *</span>{/if}
									{#if f.help}<span class="mt-0.5 block text-xs text-ink-500">{f.help}</span>{/if}
								</span>
							</label>
						{:else}
							<label for="field-{f.id}" class={labelClass}>
								{f.label}
								{#if f.required}
									<span class="font-normal text-primary-300">Required</span>
								{:else}
									<span class="font-normal text-ink-500">(optional)</span>
								{/if}
							</label>
							{#if f.type === 'textarea'}
								<textarea
									id="field-{f.id}"
									value={(answers[f.id] as string) ?? ''}
									oninput={(e) => (answers[f.id] = e.currentTarget.value)}
									maxlength="5000"
									rows="4"
									aria-invalid={invalid}
									class="field h-auto resize-y py-2 leading-relaxed"
								></textarea>
							{:else if f.type === 'select'}
								<select
									id="field-{f.id}"
									value={(answers[f.id] as string) ?? ''}
									onchange={(e) => (answers[f.id] = e.currentTarget.value)}
									aria-invalid={invalid}
									class="field sm:max-w-sm"
								>
									<option value="">Choose…</option>
									{#each f.options ?? [] as o (o)}<option value={o}>{o}</option>{/each}
								</select>
							{:else}
								<input
									id="field-{f.id}"
									type={f.type === 'number' ? 'number' : f.type === 'date' ? 'date' : 'text'}
									inputmode={f.type === 'number' ? 'decimal' : undefined}
									step={f.type === 'number' ? 'any' : undefined}
									value={(answers[f.id] as string) ?? ''}
									oninput={(e) => (answers[f.id] = e.currentTarget.value)}
									maxlength={f.type === 'text' ? 500 : undefined}
									aria-invalid={invalid}
									onkeydown={onKeydown}
									class={['field', f.type !== 'text' && 'sm:max-w-sm']}
								/>
							{/if}
							{#if f.help}<p class="mt-1.5 text-xs text-ink-500">{f.help}</p>{/if}
						{/if}
					</div>
				{/each}
			{/if}

			{#if descriptionMode !== 'hidden'}
			<div>
				<div class="flex items-baseline justify-between">
					<label for="description" class={labelClass}>
						{template?.fields.length ? 'Anything else' : 'Description'}
						{#if descriptionMode === 'optional'}<span class="font-normal text-ink-500">(optional)</span>{/if}
					</label>
					<span class={['text-xs tabular-nums', description.length > DESCRIPTION_MAX ? 'text-primary-300' : 'text-ink-500']}>{counter(description.length, DESCRIPTION_MAX)}</span>
				</div>
				<textarea
					id="description"
					bind:value={description}
					maxlength={DESCRIPTION_MAX}
					required={descriptionMode === 'required'}
					rows={template?.fields.length ? 5 : 10}
					onkeydown={onKeydown}
					placeholder={template?.fields.length
							? 'Anything else the team should know.'
							: 'What happened, what did you expect, and any steps to reproduce it.\nInclude error messages if you have them.'}
					class={['field h-auto resize-y py-2.5 leading-relaxed', template?.fields.length ? 'min-h-28' : 'min-h-56']}
				></textarea>
			</div>
			{/if}

			{#if data.settings.allowImageUploads}
				<ImageUploader bind:files={images} disabled={submitting} />
			{/if}

			<div class="flex flex-col-reverse items-stretch justify-between gap-3 border-t border-ink-800 pt-5 sm:flex-row sm:items-center">
				<p class="hidden text-xs text-ink-500 sm:block">
					<kbd class="rounded border border-ink-700 px-1 font-sans">⌘</kbd>
					<kbd class="rounded border border-ink-700 px-1 font-sans">Enter</kbd> to submit
				</p>
				<div class="flex gap-2">
					<a href="/tickets" class="flex h-9 flex-1 items-center justify-center rounded-lg px-3.5 text-sm text-ink-400 transition hover:bg-ink-850 hover:text-ink-100 sm:flex-none">
						Cancel
					</a>
					<button
						type="submit"
						disabled={submitting}
						class="inline-flex h-9 flex-1 items-center justify-center gap-1.5 rounded-lg bg-primary-400 px-4 text-sm font-semibold text-primary-950 transition hover:bg-primary-300 disabled:cursor-not-allowed disabled:opacity-70 sm:flex-none"
					>
						{#if submitting}
							<svg class="size-4 animate-spin" viewBox="0 0 24 24" fill="none" aria-hidden="true">
								<circle cx="12" cy="12" r="9" stroke="currentColor" stroke-opacity="0.25" stroke-width="3" />
								<path d="M21 12a9 9 0 0 0-9-9" stroke="currentColor" stroke-width="3" stroke-linecap="round" />
							</svg>
							{images.length ? 'Uploading…' : 'Raising…'}
						{:else}
							Raise ticket
						{/if}
					</button>
				</div>
			</div>
		</div>

		<!-- Details -->
		<aside class="space-y-6 rounded-xl border border-ink-800 bg-ink-900/60 p-5 lg:self-start">
			<div>
				<p class={labelClass}>Requester</p>
				{#if isStaff}
					<RequesterPicker bind:value={() => requester ?? me, (v) => (requester = v)} {me} />
					{#if forSomeoneElse}
						<p class="mt-1.5 text-xs text-ink-500">Raised on {requester!.name.split(' ')[0]}'s behalf. They'll be able to see it.</p>
					{/if}
				{:else}
					<p class="text-sm text-ink-100">{user.name} <span class="text-ink-500">(you)</span></p>
				{/if}
			</div>

			<fieldset>
				<legend class={labelClass}>Priority</legend>
				<div class="space-y-1">
					{#each priorities as p (p.value)}
						{@const selected = priority === p.value}
						<label
							class={[
								'flex cursor-pointer items-center gap-3 rounded-lg border px-3 py-2 transition',
								'has-focus-visible:ring-3 has-focus-visible:ring-primary-400/20',
								selected ? 'border-primary-400/50 bg-primary-400/10' : 'border-transparent hover:bg-ink-850'
							]}
						>
							<input type="radio" name="priority" value={p.value} bind:group={priority} onchange={() => (priorityTouched = true)} class="sr-only" />
							<PriorityIcon priority={p.value} class={['size-4 shrink-0', selected ? 'text-ink-100' : 'text-ink-400'].join(' ')} />
							<span class="min-w-0">
								<span class={['block text-sm', selected ? 'font-medium text-ink-100' : 'text-ink-200']}>{p.label}</span>
								<span class="block text-xs text-ink-500">{p.hint}</span>
							</span>
						</label>
					{/each}
				</div>
			</fieldset>
		</aside>
	</form>
</div>
