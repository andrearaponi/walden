package loans

import (
	"fmt"
	"strings"
	"time"
)

// MonthlyStatement renders a member's statement for one month: the loans
// borrowed in that month and the total of their fees.
func (s *Service) MonthlyStatement(memberID string, year int, month time.Month) string {
	var b strings.Builder
	member := s.members[memberID]
	fmt.Fprintf(&b, "Statement for %s, %s %d\n", member.Name, month, year)
	var total int64
	for _, loan := range s.loans {
		if loan.MemberID != memberID || loan.BorrowedAt.Year() != year || loan.BorrowedAt.Month() != month {
			continue
		}
		fmt.Fprintf(&b, "%s  %s  %s\n", loan.BorrowedAt.Format("2006-01-02"), loan.CopyID, formatCents(loan.FeeCents))
		total += loan.FeeCents
	}
	fmt.Fprintf(&b, "Total fees: %s\n", formatCents(total))
	return b.String()
}

func formatCents(cents int64) string {
	return fmt.Sprintf("EUR %d.%02d", cents/100, cents%100)
}
