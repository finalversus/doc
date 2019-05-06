package security

import "fmt"

type StdHandler interface {
	GenerateParams(d *StdEncryptDict, ownerPass, userPass []byte) ([]byte, error)

	Authenticate(d *StdEncryptDict, pass []byte) ([]byte, Permissions, error)
}

type StdEncryptDict struct {
	R int

	P               Permissions
	EncryptMetadata bool

	O, U   []byte
	OE, UE []byte
	Perms  []byte
}

func checkAtLeast(fnc, field string, exp int, b []byte) error {
	if len(b) < exp {
		return errInvalidField{Func: fnc, Field: field, Exp: exp, Got: len(b)}
	}
	return nil
}

type errInvalidField struct {
	Func  string
	Field string
	Exp   int
	Got   int
}

func (e errInvalidField) Error() string {
	return fmt.Sprintf("%s: expected %s field to be %d bytes, got %d", e.Func, e.Field, e.Exp, e.Got)
}
