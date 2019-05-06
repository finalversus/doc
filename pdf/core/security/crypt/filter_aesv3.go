package crypt

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"fmt"
	"io"

	"github.com/codefinio/doc/common"
)

func init() {
	registerFilter("AESV3", newFilterAESV3)
}

func NewFilterAESV3() Filter {
	f, err := newFilterAESV3(FilterDict{})
	if err != nil {
		panic(err)
	}
	return f
}

func newFilterAESV3(d FilterDict) (Filter, error) {
	if d.Length == 256 {
		common.Log.Debug("AESV3 crypt filter length appears to be in bits rather than bytes - assuming bits (%d)", d.Length)
		d.Length /= 8
	}
	if d.Length != 0 && d.Length != 32 {
		return nil, fmt.Errorf("invalid AESV3 crypt filter length (%d)", d.Length)
	}
	return filterAESV3{}, nil
}

type filterAES struct{}

func (filterAES) EncryptBytes(buf []byte, okey []byte) ([]byte, error) {

	ciph, err := aes.NewCipher(okey)
	if err != nil {
		return nil, err
	}

	common.Log.Trace("AES Encrypt (%d): % x", len(buf), buf)

	const block = aes.BlockSize

	pad := block - len(buf)%block
	for i := 0; i < pad; i++ {
		buf = append(buf, byte(pad))
	}
	common.Log.Trace("Padded to %d bytes", len(buf))

	ciphertext := make([]byte, block+len(buf))
	iv := ciphertext[:block]
	if _, err := io.ReadFull(rand.Reader, iv); err != nil {
		return nil, err
	}

	mode := cipher.NewCBCEncrypter(ciph, iv)
	mode.CryptBlocks(ciphertext[block:], buf)

	buf = ciphertext
	common.Log.Trace("to (%d): % x", len(buf), buf)

	return buf, nil
}

func (filterAES) DecryptBytes(buf []byte, okey []byte) ([]byte, error) {

	ciph, err := aes.NewCipher(okey)
	if err != nil {
		return nil, err
	}

	if len(buf) < 16 {
		common.Log.Debug("ERROR AES invalid buf %s", buf)
		return buf, fmt.Errorf("AES: Buf len < 16 (%d)", len(buf))
	}

	iv := buf[:16]
	buf = buf[16:]

	if len(buf)%16 != 0 {
		common.Log.Debug(" iv (%d): % x", len(iv), iv)
		common.Log.Debug("buf (%d): % x", len(buf), buf)
		return buf, fmt.Errorf("AES buf length not multiple of 16 (%d)", len(buf))
	}

	mode := cipher.NewCBCDecrypter(ciph, iv)

	common.Log.Trace("AES Decrypt (%d): % x", len(buf), buf)
	common.Log.Trace("chop AES Decrypt (%d): % x", len(buf), buf)
	mode.CryptBlocks(buf, buf)
	common.Log.Trace("to (%d): % x", len(buf), buf)

	if len(buf) == 0 {
		common.Log.Trace("Empty buf, returning empty string")
		return buf, nil
	}

	padLen := int(buf[len(buf)-1])
	if padLen > len(buf) {
		common.Log.Debug("Illegal pad length (%d > %d)", padLen, len(buf))
		return buf, fmt.Errorf("invalid pad length")
	}
	buf = buf[:len(buf)-padLen]

	return buf, nil
}

var _ Filter = filterAESV3{}

type filterAESV3 struct {
	filterAES
}

func (filterAESV3) PDFVersion() [2]int {
	return [2]int{2, 0}
}

func (filterAESV3) HandlerVersion() (V, R int) {
	V, R = 5, 6
	return
}

func (filterAESV3) Name() string {
	return "AESV3"
}

func (filterAESV3) KeyLength() int {
	return 256 / 8
}

func (filterAESV3) MakeKey(_, _ uint32, ekey []byte) ([]byte, error) {
	return ekey, nil
}
