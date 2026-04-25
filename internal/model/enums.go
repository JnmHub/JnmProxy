package model

type SubscriptionStatus string

const (
	SubscriptionStatusSuccess SubscriptionStatus = "success"
	SubscriptionStatusFailed  SubscriptionStatus = "failed"
	SubscriptionStatusNever   SubscriptionStatus = "never"
)

type AdapterStatus string

const (
	AdapterStatusSupported   AdapterStatus = "supported"
	AdapterStatusUnsupported AdapterStatus = "unsupported"
	AdapterStatusError       AdapterStatus = "error"
)

type AliveStatus string

const (
	AliveStatusUnknown AliveStatus = "unknown"
	AliveStatusAlive   AliveStatus = "alive"
	AliveStatusDead    AliveStatus = "dead"
)

type CredentialBindMode string

const (
	CredentialBindModeAll   CredentialBindMode = "all"
	CredentialBindModeGroup CredentialBindMode = "group"
	CredentialBindModeNode  CredentialBindMode = "node"
)

type SelectionPolicy string

const (
	SelectionPolicyRandom SelectionPolicy = "random"
	SelectionPolicyFixed  SelectionPolicy = "fixed"
)

type SingBoxStatus string

const (
	SingBoxStatusSupported   SingBoxStatus = "supported"
	SingBoxStatusUnsupported SingBoxStatus = "unsupported"
	SingBoxStatusError       SingBoxStatus = "error"
)
