package registration

type EventSettingsRequest struct {
	DefaultMode     string `json:"defaultMode"`
	QueueEnabled    bool   `json:"queueEnabled"`
	BallotEnabled   bool   `json:"ballotEnabled"`
	PriorityEnabled bool   `json:"priorityEnabled"`
	WaitlistEnabled bool   `json:"waitlistEnabled"`
}

type CategorySettingsRequest struct {
	CategoryID       string  `json:"categoryId"`
	RegistrationMode *string `json:"registrationMode"`
	OverrideEnabled  bool    `json:"overrideEnabled"`
}

type SettingsResponse struct {
	EventID         string                     `json:"eventId"`
	DefaultMode     string                     `json:"defaultMode"`
	QueueEnabled    bool                       `json:"queueEnabled"`
	BallotEnabled   bool                       `json:"ballotEnabled"`
	PriorityEnabled bool                       `json:"priorityEnabled"`
	WaitlistEnabled bool                       `json:"waitlistEnabled"`
	Categories      []CategorySettingsResponse `json:"categories"`
}

type CategorySettingsResponse struct {
	CategoryID       string  `json:"categoryId"`
	RegistrationMode *string `json:"registrationMode"`
	OverrideEnabled  bool    `json:"overrideEnabled"`
}
