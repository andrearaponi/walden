package loans

import (
	"errors"
	"testing"
	"time"
)

func TestBorrowRejectsUnknownMember(t *testing.T) {
	s := NewService(Features{}, []Member{{ID: "m1", Name: "Ada", Email: "ada@example.com"}})
	_, err := s.Borrow("nobody", "c1", time.Date(2026, 3, 1, 10, 0, 0, 0, time.UTC))
	if !errors.Is(err, ErrUnknownMember) {
		t.Fatalf("err = %v, want ErrUnknownMember", err)
	}
	if got := s.ActiveLoans("nobody"); len(got) != 0 {
		t.Fatalf("a loan was recorded: %v", got)
	}
}
