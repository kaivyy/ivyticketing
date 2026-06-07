package inventory

// Remaining computes available slots. Never returns negative.
func Remaining(capacity, reserved, paid int64) int64 {
	r := capacity - reserved - paid
	if r < 0 {
		return 0
	}
	return r
}

// HasCapacity reports whether at least one slot is free.
func HasCapacity(capacity, reserved, paid int64) bool {
	return capacity-reserved-paid > 0
}
