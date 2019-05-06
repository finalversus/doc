package security

type AuthEvent string

const (
	EventDocOpen = AuthEvent("DocOpen")

	EventEFOpen = AuthEvent("EFOpen")
)
