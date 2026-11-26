<script lang="ts">
	import { onMount } from 'svelte';
	import { updates, type UpdateStatus } from '$lib/api';
	import { dateTimeFmt, timeAgo } from '$lib/format';
	import Icon from '$lib/components/Icon.svelte';

	let st = $state<UpdateStatus | null>(null);
	let checking = $state(false);
	let loadFailed = $state(false);
	let copied = $state(false);

	const COMMAND = './install.sh update';

	async function check(refresh: boolean) {
		checking = true;
		loadFailed = false;
		try {
			st = await updates.check(refresh);
		} catch {
			loadFailed = true;
		} finally {
			checking = false;
		}
	}

	onMount(() => check(false));

	async function copy() {
		try {
			await navigator.clipboard.writeText(COMMAND);
			copied = true;
			setTimeout(() => (copied = false), 2000);
		} catch {
			/* clipboard blocked; the command is on screen */
		}
	}

	const short = (sha: string) => sha.slice(0, 7);

	const btnSecondary =
		'inline-flex h-9 items-center gap-1.5 rounded-lg border border-ink-700 px-3.5 text-sm font-medium text-ink-200 transition hover:border-ink-500 hover:text-ink-100 disabled:cursor-not-allowed disabled:opacity-40';
</script>

{#if loadFailed && !st}
	<p class="mt-5 text-sm text-danger-200">Couldn't check for updates.</p>
	<button type="button" onclick={() => check(true)} class={['mt-3', btnSecondary]}>Try again</button>
{:else if !st}
	<p class="mt-5 text-sm text-ink-500">Checking GitHub for updates…</p>
{:else}
	<div class="mt-5 rounded-xl border border-ink-800 bg-ink-900/60">
		<!-- Status -->
		<div class="flex flex-wrap items-start justify-between gap-4 border-b border-ink-800 px-5 py-4">
			<div class="flex min-w-0 items-start gap-3">
				{#if st.status === 'current'}
					<Icon name="check" class="mt-0.5 size-5 shrink-0 text-primary-400" />
				{:else if st.status === 'behind'}
					<Icon name="download" class="mt-0.5 size-5 shrink-0 text-primary-400" />
				{:else}
					<Icon name="shieldAlert" class="mt-0.5 size-5 shrink-0 text-ink-400" />
				{/if}
				<div class="min-w-0 text-sm">
					<h3 class="text-base font-semibold">
						{#if st.status === 'current'}
							Lithium is up to date
						{:else if st.status === 'behind'}
							{st.behind} {st.behind === 1 ? 'update' : 'updates'} available
						{:else if st.status === 'unknown'}
							Running version not recorded
						{:else if st.status === 'unrecognised'}
							Running a version GitHub doesn't have
						{:else}
							Couldn't check for updates
						{/if}
					</h3>
					<p class="mt-1 text-ink-400">
						{#if st.status === 'error'}
							{st.error ? st.error.charAt(0).toUpperCase() + st.error.slice(1) + '.' : ''}
							Make sure the server can reach github.com.
						{:else if st.status === 'unknown'}
							This build wasn't made with <code class="font-mono text-[12px]">install.sh</code>, so
							Lithium can't tell which version it is. Updating once with the command below fixes that.
						{:else if st.status === 'unrecognised'}
							This build has changes that aren't on the
							<span class="font-mono text-[12px]">{st.branch}</span> branch, so it can't be compared.
						{:else}
							Running
							<a href="{st.repoUrl}/commit/{st.current}" target="_blank" rel="noopener noreferrer" class="font-mono text-[12px] text-ink-200 hover:text-primary-300">{short(st.current)}</a>
							{#if st.status === 'behind' && st.latest}
								· latest is
								<a href={st.latest.url} target="_blank" rel="noopener noreferrer" class="font-mono text-[12px] text-ink-200 hover:text-primary-300">{short(st.latest.sha)}</a>
								from {timeAgo(st.latest.date)}
							{/if}
						{/if}
					</p>
				</div>
			</div>
			<button type="button" onclick={() => check(true)} disabled={checking} class={btnSecondary}>
				<Icon name="refresh" class={['size-4', checking && 'animate-spin'].filter(Boolean).join(' ')} />
				{checking ? 'Checking…' : 'Check for updates'}
			</button>
		</div>

		<!-- What's new -->
		{#if st.status === 'behind' && st.commits.length}
			<div class="border-b border-ink-800 px-5 py-4">
				<h4 class="text-[13px] font-medium text-ink-200">What's new</h4>
				<ul class="mt-2 space-y-1.5 text-sm">
					{#each st.commits as c (c.sha)}
						<li class="flex items-baseline gap-3">
							<a href={c.url} target="_blank" rel="noopener noreferrer" class="shrink-0 font-mono text-[12px] text-ink-500 hover:text-primary-300">{short(c.sha)}</a>
							<span class="min-w-0 flex-1 truncate text-ink-200" title={c.message}>{c.message}</span>
							<span class="shrink-0 text-xs text-ink-500" title={dateTimeFmt.format(new Date(c.date))}>{timeAgo(c.date)}</span>
						</li>
					{/each}
				</ul>
				{#if st.behind > st.commits.length}
					<a href="{st.repoUrl}/compare/{st.current}...{st.branch}" target="_blank" rel="noopener noreferrer" class="mt-2 inline-flex items-center gap-1 text-[13px] text-ink-400 hover:text-primary-300">
						and {st.behind - st.commits.length} more on GitHub <Icon name="external" class="size-3.5" />
					</a>
				{/if}
			</div>
		{/if}

		<!-- How to update -->
		<div class="space-y-3 px-5 py-4 text-sm">
			<p class="text-ink-300">
				{st.status === 'current' ? 'When an update comes out, install it' : 'To update, run this'} in the Lithium folder on
				the server. It pulls the latest version from GitHub, rebuilds and restarts. Your data is kept, and Lithium is
				unavailable for about a minute while it restarts.
			</p>
			<div class="flex items-center gap-2 rounded-lg border border-ink-800 bg-ink-950/60 py-1.5 pr-1.5 pl-3">
				<code class="min-w-0 flex-1 truncate font-mono text-[13px] text-ink-100">{COMMAND}</code>
				<button type="button" onclick={copy} class="inline-flex h-7 items-center gap-1.5 rounded-md px-2 text-xs text-ink-400 transition hover:bg-ink-850 hover:text-ink-100">
					<Icon name={copied ? 'check' : 'copy'} class="size-3.5" />
					{copied ? 'Copied' : 'Copy'}
				</button>
			</div>
			<p class="text-xs text-ink-500">
				Lithium can't update itself from here: for safety, the app has no access to the server or to Docker.
				Checked {timeAgo(st.checkedAt)} against
				<a href={st.repoUrl} target="_blank" rel="noopener noreferrer" class="hover:text-primary-300">{st.repoUrl.replace('https://', '')}</a>.
			</p>
		</div>
	</div>
{/if}
