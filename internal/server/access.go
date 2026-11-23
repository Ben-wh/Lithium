package server

import (
	"context"

	"github.com/lithium/lithium/internal/store"
)

// access describes which tickets a user may see.
//
//   - Regular users: only tickets they requested.
//   - Admins, and agents while agent groups are off: everything.
//   - Agents while agent groups are on: tickets in their groups' categories,
//     plus ones they raised or are assigned, plus uncategorised tickets (so
//     nothing is stranded where only admins can see it).
type access struct {
	userID     int64
	all        bool
	ownOnly    bool           // regular users
	categories map[int64]bool // group-restricted agents
}

func (s *Server) accessFor(ctx context.Context, u store.User) (access, error) {
	a := access{userID: u.ID}
	switch {
	case !isStaff(u):
		a.ownOnly = true
		return a, nil
	case u.Role == "admin":
		a.all = true
		return a, nil
	}
	settings, err := s.store.Settings(ctx)
	if err != nil {
		return a, err
	}
	if !settings.AgentGroups {
		a.all = true
		return a, nil
	}
	ids, err := s.store.GroupCategoryIDs(ctx, u.ID)
	if err != nil {
		return a, err
	}
	a.categories = make(map[int64]bool, len(ids))
	for _, id := range ids {
		a.categories[id] = true
	}
	return a, nil
}

// canView reports whether the user may see a ticket with these details.
func (a access) canView(requesterID, assigneeID, categoryID int64) bool {
	switch {
	case a.all:
		return true
	case a.ownOnly:
		return requesterID == a.userID
	}
	return requesterID == a.userID || assigneeID == a.userID || categoryID == 0 || a.categories[categoryID]
}

func (a access) canViewTicket(t store.Ticket) bool {
	var assignee, category int64
	if t.Assignee != nil {
		assignee = t.Assignee.ID
	}
	if t.Category != nil {
		category = t.Category.ID
	}
	return a.canView(t.Requester.ID, assignee, category)
}

func (a access) canViewLink(l store.TicketLink) bool {
	var assignee, category int64
	if l.Assignee != nil {
		assignee = l.Assignee.ID
	}
	if l.Category != nil {
		category = l.Category.ID
	}
	return a.canView(l.RequesterID, assignee, category)
}

// scope applies these rules to a ticket list query.
func (a access) scope(f *store.TicketFilter) {
	switch {
	case a.all:
	case a.ownOnly:
		f.RequesterID = a.userID
	default:
		f.Scoped = true
		f.ScopeUserID = a.userID
		f.ScopeCategories = make([]int64, 0, len(a.categories))
		for id := range a.categories {
			f.ScopeCategories = append(f.ScopeCategories, id)
		}
	}
}

// canBeAssigned reports whether a staff member may be assigned a ticket: they
// must be able to see it without counting the assignment itself (otherwise
// assigning anyone would always qualify them).
func (s *Server) canBeAssigned(ctx context.Context, u store.User, t store.Ticket) (bool, error) {
	if !isStaff(u) {
		return false, nil
	}
	a, err := s.accessFor(ctx, u)
	if err != nil {
		return false, err
	}
	var category int64
	if t.Category != nil {
		category = t.Category.ID
	}
	return a.canView(t.Requester.ID, 0, category), nil
}

// canViewTicket checks whether a given user may see a ticket.
func (s *Server) userCanView(ctx context.Context, u store.User, t store.Ticket) (bool, error) {
	a, err := s.accessFor(ctx, u)
	if err != nil {
		return false, err
	}
	return a.canViewTicket(t), nil
}
