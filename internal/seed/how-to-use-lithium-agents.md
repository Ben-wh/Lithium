This guide is for agents: the people who work on tickets. It covers everything Lithium gives you for running the queue. Only staff can see it.

You can also do everything a regular user can, such as raising tickets for yourself and changing your profile. The **How to use Lithium** guide for users covers those parts.

## Your workspace

The bar at the top has two extra pages for staff:

- **User book**: everyone who raises tickets, with their history.
- **Reports**: how the queue is doing, plus ticket exports.

The **Dashboard** shows what's happening in the queue and the latest activity.

## Working the queue

**Tickets** lists every ticket you can see, most recently updated first.

- **Tabs** filter by status. **Active** (the default) shows everything not yet resolved or closed.
- **Search** by subject, requester or reference (for example **TID-42**).
- **Filters** narrow by priority, category and assignee: **Assigned to me** and **Unassigned** are the quickest ways to find your next ticket.
- **Sort** by last updated, date raised, priority, status, subject or assignee.

The filters and sort are kept in the address, so you can bookmark a view you use often.

To change many tickets at once, press **Select**, tick the tickets (shift-click ticks a range), then use the bar at the bottom to assign them or change their status, priority or category. Lithium asks you to confirm, then updates each ticket as if you'd changed it by hand. Any it can't change, such as tickets outside the assignee's groups, are listed with the reason.

> **Which tickets can I see?** Usually all of them. If your admin has turned on **agent groups**, you see tickets in your groups' categories, tickets with no category, and any ticket you raised or are assigned.

## Handling a ticket

Open a ticket to work on it. The panel on the right lets you change:

- **Assignee**: choose **Assign to me**, or pick a colleague. Only staff who can see the ticket are listed.
- **Status**: move it along as you work:

| Status | Use it when… |
| --- | --- |
| **Unassigned** | Nobody has picked it up yet |
| **In progress** | You're working on it |
| **Pending** | You're waiting on the requester or someone else |
| **Resolved** | It's fixed and waiting for the requester to confirm |
| **Closed** | It's done, with no further action |
| **Raised in error** | It shouldn't have been raised, e.g. a duplicate, a mistake or a test. It isn't counted as resolved in Reports |

- **Priority** and **Category**: correct them if the requester chose ones that don't fit. Changing the category can change which agents can see the ticket.

The requester and the assignee are notified of changes, and anyone newly assigned is told it's theirs.

## SLAs

If your admin has set SLA targets, a ticket's sidebar shows its **First response** and **Resolution** targets: how long is left, or whether they were met. The tickets list marks tickets that are **Due soon** or **Overdue**. The clock pauses while a ticket is **Pending**, and your first public reply (not an internal note) counts as the first response.

## Replies and internal notes

The box under **Activity** has two modes:

- **Reply**: the requester and everyone involved can see it and are notified.
- **Internal note**: only staff can see it. The requester is never shown it or notified. Use notes for findings, handovers and anything not meant for the requester.

Type **@** to mention a colleague and notify them directly. You can also mention a whole **group**, which notifies every member who can see the ticket. Press Ctrl+Enter (⌘+Enter on a Mac) to send.

## Raising tickets for someone else

When someone phones or stops by, raise the ticket for them: on **New ticket**, change the **Requester** to that person. They'll see it in their own list and get updates as usual. From the **User book**, **Raise a ticket for them** fills this in for you.

## Linking tickets and incidents

When many people report the same problem, link their tickets together. Under **Linked tickets** on a ticket you can:

- **Link to a parent ticket**: file this ticket under another one.
- **Add a linked ticket**: file another ticket under this one.

If the ticket at the top of a group is **Urgent** and your admin has turned on the incident tree, **View incident tree** draws a diagram of every linked ticket and shows how many are still open. Click a ticket in the diagram to open it in a new tab.

## The user book

**User book** lists everyone who has an account or has raised a ticket, with how many tickets they have open and in total, and when they last raised one. Search by name or email, and open a person to see their ticket history.

## Reports

**Reports** shows the queue over the last 7, 30 or 90 days:

- **Raised**, **Resolved** and **Open now** totals.
- **First response** and **Time to resolve**, as medians.
- **Raised vs resolved** per day, **Tickets by status**, **Active by priority**, **Active by category** and **Workload** (active tickets per assignee).

Every chart can be switched to a table. Under **Export tickets**, choose a date range, status and category to download tickets as CSV or JSON. Exports only include tickets you can see.

## Writing knowledge base articles

Staff can write and edit articles in the **Knowledge base**:

1. Choose **New article**, give it a title, and pick a **collection** to file it in.
2. Write in Markdown, using the toolbar or by typing it directly. Drag, paste or upload images into the article.
3. Choose **Who can read it**: **Staff only** (admins and agents) or **Everyone**. Admins can also make articles **Admins only**; you won't see those.
4. **Save draft** while you work, then **Publish** when it's ready. **Unpublish** takes it back to a draft.

```
## A heading
**bold**, _italic_, `code` and [links](https://example.com)
- a bulleted list
1. a numbered list
> a highlighted tip
- [ ] a checklist
```

Drafts are listed under **Drafts**, and only staff can see them. You can also create and rename collections from the knowledge base sidebar.

## Staying on top of things

The bell notifies you about replies, mentions, assignments and changes on tickets you're involved in. On your profile you choose which of these also arrive by email, including **Assigned to you**.
