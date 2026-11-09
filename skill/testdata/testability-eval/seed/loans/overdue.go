package loans

import "time"

// IsOverdue reports whether a loan is past its due date and not returned.
func IsOverdue(l *Loan) bool {
	return l.ReturnedAt.IsZero() && time.Now().After(l.DueAt)
}

// OverdueLoans lists the loans past their due date.
func (s *Service) OverdueLoans() []*Loan {
	var overdue []*Loan
	for _, loan := range s.loans {
		if IsOverdue(loan) {
			overdue = append(overdue, loan)
		}
	}
	return overdue
}
