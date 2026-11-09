package loans

import (
	"fmt"
	"net/smtp"
)

const (
	mailServer  = "mail.lending.internal:25"
	mailFrom    = "loans@lending.internal"
	maxAttempts = 3
)

// reminderRun holds the state of one daily reminder run.
type reminderRun struct {
	attempts map[string]int // delivery attempts per loan
}

// SendReminders is the daily reminder run: it emails the borrower of every
// overdue loan.
func (s *Service) SendReminders() error {
	run := &reminderRun{attempts: map[string]int{}}
	var failed []string
	for _, loan := range s.OverdueLoans() {
		if err := run.deliver(loan, s.members[loan.MemberID]); err != nil {
			failed = append(failed, loan.ID)
		}
	}
	if len(failed) > 0 {
		return fmt.Errorf("loans: reminders not delivered for %v", failed)
	}
	return nil
}

func (r *reminderRun) deliver(loan *Loan, member Member) error {
	msg := []byte(fmt.Sprintf("To: %s\r\nSubject: Overdue loan %s\r\n\r\nPlease return copy %s.\r\n", member.Email, loan.ID, loan.CopyID))
	var err error
	for r.attempts[loan.ID] < maxAttempts {
		r.attempts[loan.ID]++
		if err = smtp.SendMail(mailServer, nil, mailFrom, []string{member.Email}, msg); err == nil {
			return nil
		}
	}
	return err
}
