package crypt

import (
	"fmt"

	"github.com/finalversus/doc/pdf/core/security"
)

var (
	filterMethods = make(map[string]filterFunc)
)

type filterFunc func(d FilterDict) (Filter, error)

type Filter interface {
	Name() string

	KeyLength() int

	PDFVersion() [2]int

	HandlerVersion() (V, R int)

	MakeKey(objNum, genNum uint32, fkey []byte) ([]byte, error)

	EncryptBytes(p []byte, okey []byte) ([]byte, error)

	DecryptBytes(p []byte, okey []byte) ([]byte, error)
}

func NewFilter(d FilterDict) (Filter, error) {
	fnc, err := getFilter(d.CFM)
	if err != nil {
		return nil, err
	}
	cf, err := fnc(d)
	if err != nil {
		return nil, err
	}
	return cf, nil
}

func NewIdentity() Filter {
	return filterIdentity{}
}

type FilterDict struct {
	CFM       string
	AuthEvent security.AuthEvent
	Length    int
}

func registerFilter(name string, fnc filterFunc) {
	if _, ok := filterMethods[name]; ok {
		panic("already registered")
	}
	filterMethods[name] = fnc
}

func getFilter(name string) (filterFunc, error) {
	f := filterMethods[string(name)]
	if f == nil {
		return nil, fmt.Errorf("unsupported crypt filter: %q", name)
	}
	return f, nil
}

type filterIdentity struct{}

func (filterIdentity) PDFVersion() [2]int {
	return [2]int{}
}

func (filterIdentity) HandlerVersion() (V, R int) {
	return
}

func (filterIdentity) Name() string {
	return "Identity"
}

func (filterIdentity) KeyLength() int {
	return 0
}

func (filterIdentity) MakeKey(objNum, genNum uint32, fkey []byte) ([]byte, error) {
	return fkey, nil
}

func (filterIdentity) EncryptBytes(p []byte, okey []byte) ([]byte, error) {
	return p, nil
}

func (filterIdentity) DecryptBytes(p []byte, okey []byte) ([]byte, error) {
	return p, nil
}
