// Package loans lends copies of books to the members of a community library.
package loans

import (
	"errors"
	"fmt"
	"time"
)

// ErrUnknownMember is returned when a request names no known member.
var ErrUnknownMember = errors.New("loans: unknown member")

// ErrUnknownLoan is returned when a request names no known loan.
var ErrUnknownLoan = errors.New("loans: unknown loan")

// ErrLoanLimit is returned when a member already holds the most active loans.
var ErrLoanLimit = errors.New("loans: loan limit reached")

// MaxActiveLoans is the most loans a member may hold at once.
const MaxActiveLoans = 5

// LoanPeriod is how long a loan lasts.
const LoanPeriod = 14 * 24 * time.Hour

// Member is a library member.
type Member struct {
	ID    string
	Name  string
	Email string
}

// Loan is one copy lent to one member.
type Loan struct {
	ID         string
	MemberID   string
	CopyID     string
	BorrowedAt time.Time
	DueAt      time.Time
	ReturnedAt time.Time
	FeeCents   int64
}

// Features holds the optional features of a deployment.
type Features struct {
	OverdueBadges bool
}

// Service keeps the members and the loans in memory.
type Service struct {
	Features Features
	members  map[string]Member
	loans    []*Loan
	nextID   int
}

// NewService returns a service for the given members.
func NewService(features Features, members []Member) *Service {
	s := &Service{Features: features, members: map[string]Member{}}
	for _, m := range members {
		s.members[m.ID] = m
	}
	return s
}

// Borrow lends a copy to a member at the given instant.
func (s *Service) Borrow(memberID, copyID string, at time.Time) (*Loan, error) {
	if _, ok := s.members[memberID]; !ok {
		return nil, ErrUnknownMember
	}
	if len(s.ActiveLoans(memberID)) >= MaxActiveLoans {
		return nil, ErrLoanLimit
	}
	s.nextID++
	loan := &Loan{
		ID:         fmt.Sprintf("L%d", s.nextID),
		MemberID:   memberID,
		CopyID:     copyID,
		BorrowedAt: at,
		DueAt:      at.Add(LoanPeriod),
	}
	s.loans = append(s.loans, loan)
	return loan, nil
}

// Return closes a loan at the given instant and charges the late fee when the
// loan comes back after its due date.
func (s *Service) Return(loanID string, at time.Time) (*Loan, error) {
	loan := s.find(loanID)
	if loan == nil || !loan.ReturnedAt.IsZero() {
		return nil, ErrUnknownLoan
	}
	loan.ReturnedAt = at
	if at.After(loan.DueAt) {
		daysLate := int(at.Sub(loan.DueAt).Hours()/24) + 1
		loan.FeeCents = lateFee(daysLate)
	}
	return loan, nil
}

// ActiveLoans lists the loans a member has not returned.
func (s *Service) ActiveLoans(memberID string) []*Loan {
	var active []*Loan
	for _, loan := range s.loans {
		if loan.MemberID == memberID && loan.ReturnedAt.IsZero() {
			active = append(active, loan)
		}
	}
	return active
}

func (s *Service) find(loanID string) *Loan {
	for _, loan := range s.loans {
		if loan.ID == loanID {
			return loan
		}
	}
	return nil
}
