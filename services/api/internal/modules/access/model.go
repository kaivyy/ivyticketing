package access

const (
	PoolTypeReserved  = "RESERVED"
	PoolTypeCommunity = "COMMUNITY"
	PoolTypeCorporate = "CORPORATE"
	PoolTypeSponsor   = "SPONSOR"
	PoolTypeVIP       = "VIP"
	PoolTypePartner   = "PARTNER"
	PoolTypePriority  = "PRIORITY"
	PoolTypeElite     = "ELITE"

	GrantStatusActive   = "ACTIVE"
	GrantStatusConsumed = "CONSUMED"
	GrantStatusExpired  = "EXPIRED"

	// Corporate account statuses
	CorporateStatusPending   = "PENDING"
	CorporateStatusActive    = "ACTIVE"
	CorporateStatusSuspended = "SUSPENDED"

	// Pool member statuses
	MemberStatusPending    = "PENDING"
	MemberStatusActive     = "ACTIVE"
	MemberStatusRegistered = "REGISTERED"
	MemberStatusExpired    = "EXPIRED"
	MemberStatusRevoked    = "REVOKED"
)
