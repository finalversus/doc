package crypt

import (
	"crypto/md5"
	"crypto/rc4"
	"fmt"

	"github.com/codefinio/doc/common"
)

func init() {
	registerFilter("V2", newFilterV2)
}

func NewFilterV2(length int) Filter {
	f, err := newFilterV2(FilterDict{Length: length})
	if err != nil {
		panic(err)
	}
	return f
}

func newFilterV2(d FilterDict) (Filter, error) {
	if d.Length%8 != 0 {
		return nil, fmt.Errorf("crypt filter length not multiple of 8 (%d)", d.Length)
	}

	if d.Length < 5 || d.Length > 16 {
		if d.Length == 40 || d.Length == 64 || d.Length == 128 {
			common.Log.Debug("STANDARD VIOLATION: Crypt Length appears to be in bits rather than bytes - assuming bits (%d)", d.Length)
			d.Length /= 8
		} else {
			return nil, fmt.Errorf("crypt filter length not in range 40 - 128 bit (%d)", d.Length)
		}
	}
	return filterV2{length: d.Length}, nil
}

func makeKeyV2(objNum, genNum uint32, ekey []byte, isAES bool) ([]byte, error) {
	key := make([]byte, len(ekey)+5)
	for i := 0; i < len(ekey); i++ {
		key[i] = ekey[i]
	}
	for i := 0; i < 3; i++ {
		b := byte((objNum >> uint32(8*i)) & 0xff)
		key[i+len(ekey)] = b
	}
	for i := 0; i < 2; i++ {
		b := byte((genNum >> uint32(8*i)) & 0xff)
		key[i+len(ekey)+3] = b
	}
	if isAES {

		key = append(key, 0x73)
		key = append(key, 0x41)
		key = append(key, 0x6C)
		key = append(key, 0x54)
	}

	h := md5.New()
	h.Write(key)
	hashb := h.Sum(nil)

	if len(ekey)+5 < 16 {
		return hashb[0 : len(ekey)+5], nil
	}

	return hashb, nil
}

var _ Filter = filterV2{}

type filterV2 struct {
	length int
}

func (f filterV2) PDFVersion() [2]int {
	return [2]int{}
}

func (f filterV2) HandlerVersion() (V, R int) {
	V, R = 2, 3
	return
}

func (filterV2) Name() string {
	return "V2"
}

func (f filterV2) KeyLength() int {
	return f.length
}

func (f filterV2) MakeKey(objNum, genNum uint32, ekey []byte) ([]byte, error) {
	return makeKeyV2(objNum, genNum, ekey, false)
}

func (filterV2) EncryptBytes(buf []byte, okey []byte) ([]byte, error) {

	ciph, err := rc4.NewCipher(okey)
	if err != nil {
		return nil, err
	}
	common.Log.Trace("RC4 Encrypt: % x", buf)
	ciph.XORKeyStream(buf, buf)
	common.Log.Trace("to: % x", buf)
	return buf, nil
}

func (filterV2) DecryptBytes(buf []byte, okey []byte) ([]byte, error) {

	ciph, err := rc4.NewCipher(okey)
	if err != nil {
		return nil, err
	}
	common.Log.Trace("RC4 Decrypt: % x", buf)
	ciph.XORKeyStream(buf, buf)
	common.Log.Trace("to: % x", buf)
	return buf, nil
}
