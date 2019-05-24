package crypt

import (
	"fmt"

	"github.com/finalversus/doc/common"
)

func init() {
	registerFilter("AESV2", newFilterAESV2)
}

func NewFilterAESV2() Filter {
	f, err := newFilterAESV2(FilterDict{})
	if err != nil {
		panic(err)
	}
	return f
}

func newFilterAESV2(d FilterDict) (Filter, error) {
	if d.Length == 128 {
		common.Log.Debug("AESV2 crypt filter length appears to be in bits rather than bytes - assuming bits (%d)", d.Length)
		d.Length /= 8
	}
	if d.Length != 0 && d.Length != 16 {
		return nil, fmt.Errorf("invalid AESV2 crypt filter length (%d)", d.Length)
	}
	return filterAESV2{}, nil
}

var _ Filter = filterAESV2{}

type filterAESV2 struct {
	filterAES
}

func (filterAESV2) PDFVersion() [2]int {
	return [2]int{1, 5}
}

func (filterAESV2) HandlerVersion() (V, R int) {
	V, R = 4, 4
	return
}

func (filterAESV2) Name() string {
	return "AESV2"
}

func (filterAESV2) KeyLength() int {
	return 128 / 8
}

func (filterAESV2) MakeKey(objNum, genNum uint32, ekey []byte) ([]byte, error) {
	return makeKeyV2(objNum, genNum, ekey, true)
}
