This guide is for admins: the people who run Lithium for everyone else. It covers accounts, workspace settings, security and the audit log. Only admins can see it.

Admins can do everything agents can, and always see every ticket. The **How to use Lithium** guide for agents covers working the queue.

## Where things are

Admins have three extra places:

- **Admin** (the button at the top right): accounts, password resets and agent groups.
- **Settings**: configuration for the whole workspace. Changes here affect everyone.
- **Activity**: a record of everything people have changed.

## People and roles

Every account has one role:

| Role | What they can do |
| --- | --- |
| **Admin** | Everything, including Admin, Settings and Activity |
| **Agent** | Work on tickets, see the user book and reports, write articles. No admin access |
| **User** | Raise and follow their own tickets, and read articles shared with everyone |

Under **Admin → Users** you can:

- **Create user**: enter their name, email, role and a temporary password. They choose their own password when they first sign in.
- **Change a role**. Moving someone with active tickets to **User** unassigns those tickets, which go back to the queue.
- **Reset password**: set a new temporary password.
- **Reset two-factor**: removes their authenticator app and recovery codes and signs them out everywhere. Make sure it's really them asking.
- **Delete account**: signs them out and removes the account. Tickets they raised are permanently deleted, their active assigned tickets are unassigned, and replies on other people's tickets stay as "Deleted user". You'll be asked to type their email to confirm.

### Password resets

When someone chooses **Forgot password** on the sign-in page, the request appears under **Admin → Password resets**. Check it's really them, choose **Set new password**, and share it with them securely. Or dismiss the request.

### Agent groups

Under **Admin → Groups**, put agents into teams and choose which categories each group handles. While groups are on, agents only see tickets in their groups' categories, plus uncategorised tickets and ones they raised or are assigned. Admins always see everything.

You can set groups up first, then turn them on under **Settings → Users → Agent groups** when you're ready. Staff can also @mention a group in a reply to notify every member.

## Tickets settings

Under **Settings → Tickets**:

- **Categories**: add categories like "Hardware" or "Access request", each with a description and colour. Users pick one when they raise a ticket. **Archive** a category to hide it from new tickets; it can only be deleted while no tickets use it.
- **Ticket templates**: ask for extra details on tickets in a category. A template can have instructions, a starting priority, and fields (short text, long text, number, dropdown, checkbox or date). Choose whether the description box is required, optional or hidden. Then link the template to categories.
- **Mandatory category**: users must choose a category.
- **Allow image uploads**: up to 5 images per ticket.
- **Allow reporters to reopen their tickets**: people can reopen their own tickets once they're resolved, closed or raised in error. Agents and admins can always reopen tickets.
- **Incident tree**: lets staff see a diagram of every ticket linked under an urgent ticket.
- **Ticket ID prefix**: the letters before each ticket number, such as **TID**. Searching by an old prefix still works.
- **Length limits**: the longest subject and description a new ticket can have.

## Appearance

Under **Settings → Appearance**:

- **Company branding**: upload a logo and set your company name. Both show in the navbar, on the sign-in pages and in emails.
- **Accent colour** and **Background** for everyone.
- **Sensitive mode**: shows a red banner with your message at the top of every page, and leaves ticket subjects out of notification emails.

## Users settings

Under **Settings → Users**:

- **Mandatory two-factor authentication** (highly recommended): everyone must set up an authenticator app, including you.
- **Email notifications**: turn notification emails on or off for everyone.
- **Disable custom profile pictures**: only the built-in avatars can be used.
- **Allow users to @mention**: lets users mention people in replies. Staff can always mention.
- **Agent groups**: see above.

## Email

Lithium notifies people in the app. To email them as well, enter your mail provider's SMTP details under **Settings → Email**: server, port, encryption, username, password and from address. Set **Lithium address** to where people open Lithium, so emails can link to the ticket. Use **Send test email** to check it works.

People choose which emails they get from their profile. Emails never include reply text.

> **Tip:** if the server sets email with `LITHIUM_SMTP_*` environment variables, this page is read-only. Change the variables and restart Lithium instead.

## Lockdown mode

For emergencies, such as a suspected break-in or a leaked password. Under **Settings → Lockdown**, **Start lockdown…**:

- signs out everyone except you, straight away;
- stops anyone else signing in until you end it;
- puts a red frame around every page;
- records everything, including blocked sign-in attempts, in Activity.

You'll need to type `LOCKDOWN`, give an optional reason, and enter your password (and a two-factor code if you use one). Choose **End lockdown** when it's over. If you can't sign in to end it, someone with access to the server can run `lithium end-lockdown`.

## Backups

Under **Settings → Backups**, **Back up now** saves a copy of everything in Lithium: tickets, users, settings, the knowledge base and every uploaded file. Turn on **Back up automatically** to make one every day or week at a set time, keeping the newest few.

Backups are kept on the Lithium server, so **Download** them now and then and keep copies somewhere safe. They contain everything, including email settings, so treat them like a password. Restoring one is done on the server; the instructions are on the Backups page.

## Activity

**Activity** lists everything people have changed in Lithium over the last year, including sign-ins and refused attempts. Search it, filter by date or type, or choose **Failed only** to see blocked actions. Open an entry for the account, IP address and browser.

## Admin-only articles

When you write a knowledge base article, **Who can read it** has an extra option for admins: **Admins only**. Use it for runbooks, recovery steps and anything agents shouldn't see. Agents can't see, edit or delete these articles, and they can't make an article admin-only.

## Getting started checklist

- [ ] Upload your logo and set your company name
- [ ] Add categories, and templates for any that need extra details
- [ ] Set up email
- [ ] Turn on mandatory two-factor authentication
- [ ] Create accounts for your agents
- [ ] Set up agent groups, if your team is split by area
- [ ] Write knowledge base articles for common questions
