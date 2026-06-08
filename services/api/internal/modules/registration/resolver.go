package registration

type ModeInput struct {
	EventModeSet     bool
	EventMode        Mode
	CategoryOverride bool
	CategoryModeSet  bool
	CategoryMode     Mode
}

func ResolveMode(in ModeInput) Mode {
	if in.CategoryOverride && in.CategoryModeSet && in.CategoryMode != "" {
		return in.CategoryMode
	}
	if in.EventModeSet && in.EventMode != "" {
		return in.EventMode
	}
	return ModeNormal
}
