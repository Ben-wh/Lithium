<script lang="ts">
	import { onMount } from 'svelte';
	import { invalidateAll } from '$app/navigation';
	import { page } from '$app/state';
	import CodeInput from '$lib/components/CodeInput.svelte';
	import { ApiError, lockdown, LOCKDOWN_PHRASE, type Lockdown } from '$lib/api';
	import { dateTimeFmt } from '$lib/format';
	import Icon from '$lib/components/Icon.svelte';
	import Modal from '$lib/components/Modal.svelte';

	let state_ = $state<Lockdown | null>(null);
	let loadFailed = $state(false);

	onMount(async () => {
		try {
			state_ = await lockdown.get();
		} catch {
			loadFailed = true;
		}
	});

	// --- Starting: three separate confirmations before anything happens ---
	// 1. tick that you understand each consequence
	// 2. type LOCKDOWN (and optionally say why)
	// 3. enter your password
	let step = $state<0 | 1 | 2 | 3>(0);
	const consequences = [
		'Everyone except me is signed out straight away, including other admins and my own other devices.',
		'Nobody else can sign in until I end the lockdown. They’ll see that Lithium is in lockdown.',
		'Only I can end it here. Otherwise it takes someone with access to the server.'
	];
	let understood = $state(consequences.map(() => false));
	let phrase = $state('');
	let reason = $state('');
	let password = $state('');
	// With two-factor on, the last step also asks for a code.
	const needsCode = $derived(page.data.user?.twoFactorEnabled ?? false);
	let code = $state('');
	let useRecovery = $state(false);
	let starting = $state(false);
	let startError = $state('');

	function begin() {
		understood = consequences.map(() => false);
		phrase = '';
		reason = '';
		password = '';
		code = '';
		useRecovery = false;
		startError = '';
		step = 1;
	}

	// The dialog is open while there's a step. Closing it (Esc, ×, clicking
	// outside) abandons the whole thing.
	const dialogOpen = {
		get: () => step > 0,
		set: (open: boolean) => {
			if (!open && !starting) step = 0;
		}
	};

	async function start(e: SubmitEvent) {
		e.preventDefault();
		starting = true;
		startError = '';
		try {
			state_ = await lockdown.start(password, reason.trim(), needsCode ? code : '');
			step = 0;
			password = '';
			code = '';
			await invalidateAll(); // shows the red frame
		} catch (err) {
			const msg = err instanceof ApiError ? err.message : 'something went wrong, please try again';
			startError = msg.charAt(0).toUpperCase() + msg.slice(1) + '.';
			code = ''; // codes can't be retried; enter the next one
		} finally {
			starting = false;
		}
	}

	// --- Ending ---
	let confirmEnd = $state(false);
	let ending = $state(false);
	let endError = $state('');

	async function end() {
		ending = true;
		endError = '';
		try {
			state_ = await lockdown.end();
			confirmEnd = false;
			await invalidateAll();
		} catch {
			endError = "Couldn't end the lockdown. Try again.";
		} finally {
			ending = false;
		}
	}

	const btnSecondary =
		'inline-flex h-9 items-center rounded-lg border border-ink-700 px-3.5 text-sm font-medium text-ink-200 transition hover:border-ink-500 hover:text-ink-100';
	const btnDanger =
		'inline-flex h-9 items-center gap-1.5 rounded-lg bg-danger-600 px-3.5 text-sm font-semibold text-white transition hover:bg-danger-500 disabled:cursor-not-allowed disabled:opacity-40';
</script>

{#if loadFailed}
	<p class="mt-5 text-sm text-danger-200">Couldn't load the lockdown status.</p>
{:else if !state_}
	<p class="mt-5 text-sm text-ink-500">Loading…</p>
{:else if state_.active}
	<div class="mt-5 rounded-xl border border-danger-400/40 bg-danger-400/10 p-5">
		<div class="flex items-start gap-3">
			<Icon name="lock" class="mt-0.5 size-5 shrink-0 text-danger-300" />
			<div class="min-w-0 text-sm">
				<p class="text-base font-semibold text-danger-200">Lockdown mode is on</p>
				<p class="mt-1 text-ink-300">
					Started by <span class="font-medium text-ink-100">{state_.byName}</span> on
					{dateTimeFmt.format(new Date(state_.at))}. Nobody else can sign in.
				</p>
				{#if state_.reason}
					<p class="mt-2 text-ink-300"><span class="text-ink-500">Reason:</span> {state_.reason}</p>
				{/if}
			</div>
		</div>
		<div class="mt-5 flex flex-wrap items-center gap-3">
			<button type="button" onclick={() => (confirmEnd = true)} class="inline-flex h-9 items-center gap-1.5 rounded-lg bg-primary-400 px-3.5 text-sm font-semibold text-primary-950 transition hover:bg-primary-300">
				End lockdown
			</button>
			<a href="/activity?category=auth" class="text-[13px] text-ink-400 hover:text-primary-300">See sign-in attempts →</a>
		</div>
	</div>
{:else}
	<div class="mt-5 rounded-xl border border-ink-800 bg-ink-900/60">
		<div class="border-b border-ink-800 px-5 py-4">
			<h3 class="text-sm font-semibold">Lockdown mode</h3>
			<p class="mt-0.5 text-sm text-ink-400">For emergencies, such as a suspected break-in or a leaked password.</p>
		</div>
		<div class="space-y-4 px-5 py-4 text-sm">
			<ul class="space-y-2 text-ink-300">
				<li class="flex gap-2"><Icon name="logout" class="mt-0.5 size-4 shrink-0 text-ink-500" /> Signs out everyone except you, straight away.</li>
				<li class="flex gap-2"><Icon name="lock" class="mt-0.5 size-4 shrink-0 text-ink-500" /> Stops anyone else signing in until you end it.</li>
				<li class="flex gap-2"><Icon name="shieldAlert" class="mt-0.5 size-4 shrink-0 text-ink-500" /> Puts a red LOCKDOWN MODE frame around every page, including the sign-in page.</li>
				<li class="flex gap-2"><Icon name="activity" class="mt-0.5 size-4 shrink-0 text-ink-500" /> Keeps a record of everything, including blocked sign-in attempts, in Activity.</li>
			</ul>
			<p class="text-xs text-ink-500">
				If you can't sign in to end it, someone with access to the server can run
				<code class="font-mono text-[11px]">lithium end-lockdown</code>
				(with Docker: <code class="font-mono text-[11px]">docker compose exec lithium /lithium end-lockdown</code>).
			</p>
			<button type="button" onclick={begin} class={btnDanger}>
				<Icon name="lock" class="size-4" />
				Start lockdown…
			</button>
		</div>
	</div>
{/if}

<!-- The three confirmation steps share one dialog -->
<Modal
	bind:open={dialogOpen.get, dialogOpen.set}
	title={step === 1 ? 'Start lockdown mode?' : step === 2 ? 'Confirm lockdown (2 of 3)' : 'Confirm lockdown (3 of 3)'}
	description={step === 1
		? 'This is for emergencies. Confirm each of these to continue (1 of 3).'
		: step === 2
			? `Type ${LOCKDOWN_PHRASE} to show you mean it.`
			: needsCode
				? 'Enter your password and a two-factor code. Lockdown starts as soon as you confirm.'
				: 'Enter your password. Lockdown starts as soon as you confirm.'}
>
	{#if step === 1}
		<div class="space-y-2.5">
			{#each consequences as text, i (i)}
				<label class="flex cursor-pointer items-start gap-3 rounded-lg border border-ink-800 px-3 py-2.5 text-sm text-ink-200 transition hover:border-ink-600">
					<input type="checkbox" bind:checked={understood[i]} class="mt-0.5 size-4 shrink-0 accent-[#c53030]" />
					<span>{text}</span>
				</label>
			{/each}
		</div>
		<div class="mt-5 flex justify-end gap-2">
			<button type="button" onclick={() => (step = 0)} class={btnSecondary}>Cancel</button>
			<button type="button" disabled={!understood.every(Boolean)} onclick={() => (step = 2)} class={btnDanger}>
				Continue
			</button>
		</div>
	{:else if step === 2}
		<form
			onsubmit={(e) => {
				e.preventDefault();
				if (phrase === LOCKDOWN_PHRASE) step = 3;
			}}
		>
			<label for="lockdown-phrase" class="mb-1.5 block text-[13px] font-medium text-ink-200">
				Type <span class="font-mono font-bold text-danger-200">{LOCKDOWN_PHRASE}</span>
			</label>
			<!-- svelte-ignore a11y_autofocus -->
			<input
				id="lockdown-phrase"
				bind:value={phrase}
				autocomplete="off"
				spellcheck="false"
				autofocus
				class="field h-9 w-full font-mono text-[13px] tracking-widest"
			/>
			<label for="lockdown-reason" class="mt-4 mb-1.5 block text-[13px] font-medium text-ink-200">
				Reason <span class="font-normal text-ink-500">(optional, shown to admins and in Activity)</span>
			</label>
			<input id="lockdown-reason" bind:value={reason} maxlength="200" placeholder="e.g. Suspected leaked admin password" class="field h-9 w-full text-[13px]" />
			<div class="mt-5 flex justify-between gap-2">
				<button type="button" onclick={() => (step = 1)} class={btnSecondary}>Back</button>
				<div class="flex gap-2">
					<button type="button" onclick={() => (step = 0)} class={btnSecondary}>Cancel</button>
					<button type="submit" disabled={phrase !== LOCKDOWN_PHRASE} class={btnDanger}>Continue</button>
				</div>
			</div>
		</form>
	{:else if step === 3}
		<form onsubmit={start}>
			<div class="mb-4 flex items-start gap-3 rounded-lg border border-danger-400/25 bg-danger-400/10 px-3 py-2.5 text-sm text-danger-200">
				<Icon name="shieldAlert" class="mt-0.5 size-4 shrink-0" />
				<p>Everyone else will be signed out the moment you press the button below.</p>
			</div>
			<label for="lockdown-password" class="mb-1.5 block text-[13px] font-medium text-ink-200">Your password</label>
			<!-- svelte-ignore a11y_autofocus -->
			<input
				id="lockdown-password"
				type="password"
				bind:value={password}
				autocomplete="current-password"
				autofocus
				class="field h-9 w-full text-[13px]"
			/>
			{#if needsCode}
				<div class="mt-4">
					<div class="mb-1.5 flex items-baseline justify-between gap-3">
						<span class="text-[13px] font-medium text-ink-200">
							{useRecovery ? 'Recovery code' : 'Code from your authenticator app'}
						</span>
						<button
							type="button"
							onclick={() => {
								useRecovery = !useRecovery;
								code = '';
							}}
							class="text-xs text-ink-400 hover:text-primary-300"
						>
							{useRecovery ? 'Use your app instead' : 'Use a recovery code'}
						</button>
					</div>
					{#key useRecovery}
						<CodeInput bind:value={code} allowRecovery={useRecovery} disabled={starting} />
					{/key}
				</div>
			{/if}
			{#if startError}
				<p role="alert" class="mt-3 text-sm text-danger-300">{startError}</p>
			{/if}
			<div class="mt-5 flex justify-between gap-2">
				<button type="button" onclick={() => (step = 2)} disabled={starting} class={btnSecondary}>Back</button>
				<div class="flex gap-2">
					<button type="button" onclick={() => (step = 0)} disabled={starting} class={btnSecondary}>Cancel</button>
					<button type="submit" disabled={!password || (needsCode && !code.trim()) || starting} class={btnDanger}>
						<Icon name="lock" class="size-4" />
						{starting ? 'Locking down…' : 'Lock everyone out now'}
					</button>
				</div>
			</div>
		</form>
	{/if}
</Modal>

<Modal
	bind:open={confirmEnd}
	title="End lockdown?"
	description="Everyone will be able to sign in again. People who were signed out will need to sign in again."
>
	{#if endError}
		<p role="alert" class="mb-3 text-sm text-danger-300">{endError}</p>
	{/if}
	<div class="mt-2 flex justify-end gap-2">
		<button type="button" onclick={() => (confirmEnd = false)} class={btnSecondary}>Keep it on</button>
		<button
			type="button"
			onclick={end}
			disabled={ending}
			class="inline-flex h-9 items-center rounded-lg bg-primary-400 px-3.5 text-sm font-semibold text-primary-950 transition hover:bg-primary-300 disabled:opacity-40"
		>
			{ending ? 'Ending…' : 'End lockdown'}
		</button>
	</div>
</Modal>
