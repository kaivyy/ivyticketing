package sms

// MaskPhoneNumber masks the middle digits of a phone number for privacy in logs.
// e.g., "+6281234567890" -> "+62812****7890", "08123456789" -> "0812****6789".
func MaskPhoneNumber(phone string) string {
	if len(phone) <= 6 {
		return "****"
	}
	prefixLen := 4
	suffixLen := 4
	if len(phone) < 10 {
		prefixLen = 2
		suffixLen = 2
	}
	prefix := phone[:prefixLen]
	suffix := phone[len(phone)-suffixLen:]
	return prefix + "****" + suffix
}
