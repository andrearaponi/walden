package loans

// lateFee returns the fee, in cents, for a loan returned daysLate days after
// its due date.
func lateFee(daysLate int) int64 {
	fee := int64(150 + 35*(daysLate-1))
	if daysLate > 30 {
		fee += 500
	}
	return fee
}
