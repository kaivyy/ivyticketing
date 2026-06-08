package queue

type JoinResponse struct {
	TokenID  string `json:"tokenId"`
	Status   string `json:"status"`
	Position int64  `json:"position"`
}

type StatusResponse struct {
	TokenID              string `json:"tokenId"`
	Status               string `json:"status"`
	Position             int64  `json:"position"`
	EstimatedWaitSeconds int64  `json:"estimatedWaitSeconds"`
	SystemState          string `json:"systemState"`
	AdmissionToken       string `json:"admissionToken,omitempty"`
	CheckoutExpiresAt    string `json:"checkoutExpiresAt,omitempty"`
}
