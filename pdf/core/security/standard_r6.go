package security

import (
	"bytes"
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"crypto/sha256"
	"crypto/sha512"
	"encoding/binary"
	"errors"
	"hash"
	"io"
	"math"
)

var _ StdHandler = stdHandlerR6{}

func newAESCipher(b []byte) cipher.Block {
	c, err := aes.NewCipher(b)
	if err != nil {
		panic(err)
	}
	return c
}

func NewHandlerR6() StdHandler {
	return stdHandlerR6{}
}

type stdHandlerR6 struct{}

func (sh stdHandlerR6) alg2a(d *StdEncryptDict, pass []byte) ([]byte, Permissions, error) {

	if err := checkAtLeast("alg2a", "O", 48, d.O); err != nil {
		return nil, 0, err
	}
	if err := checkAtLeast("alg2a", "U", 48, d.U); err != nil {
		return nil, 0, err
	}

	if len(pass) > 127 {
		pass = pass[:127]
	}

	h, err := sh.alg12(d, pass)
	if err != nil {
		return nil, 0, err
	}
	var (
		data []byte
		ekey []byte
		ukey []byte
	)
	var perm Permissions
	if len(h) != 0 {

		perm = PermOwner

		str := make([]byte, len(pass)+8+48)
		i := copy(str, pass)
		i += copy(str[i:], d.O[40:48])
		i += copy(str[i:], d.U[0:48])

		data = str
		ekey = d.OE
		ukey = d.U[0:48]
	} else {

		h, err = sh.alg11(d, pass)
		if err == nil && len(h) == 0 {

			h, err = sh.alg11(d, []byte(""))
		}
		if err != nil {
			return nil, 0, err
		} else if len(h) == 0 {

			return nil, 0, nil
		}
		perm = d.P

		str := make([]byte, len(pass)+8)
		i := copy(str, pass)
		i += copy(str[i:], d.U[40:48])

		data = str
		ekey = d.UE
		ukey = nil
	}
	if err := checkAtLeast("alg2a", "Key", 32, ekey); err != nil {
		return nil, 0, err
	}
	ekey = ekey[:32]

	ikey := sh.alg2b(d.R, data, pass, ukey)

	ac, err := aes.NewCipher(ikey[:32])
	if err != nil {
		return nil, 0, err
	}

	iv := make([]byte, aes.BlockSize)
	cbc := cipher.NewCBCDecrypter(ac, iv)
	fkey := make([]byte, 32)
	cbc.CryptBlocks(fkey, ekey)

	if d.R == 5 {
		return fkey, perm, nil
	}

	err = sh.alg13(d, fkey)
	if err != nil {
		return nil, 0, err
	}
	return fkey, perm, nil
}

func alg2bR5(data []byte) []byte {
	h := sha256.New()
	h.Write(data)
	return h.Sum(nil)
}

func repeat(buf []byte, n int) {
	bp := n
	for bp < len(buf) {
		copy(buf[bp:], buf[:bp])
		bp *= 2
	}
}

func alg2b(data, pwd, userKey []byte) []byte {
	var (
		s256, s384, s512 hash.Hash
	)
	s256 = sha256.New()
	hbuf := make([]byte, 64)

	h := s256
	h.Write(data)
	K := h.Sum(hbuf[:0])

	buf := make([]byte, 64*(127+64+48))

	round := func(rnd int) (E []byte) {

		n := len(pwd) + len(K) + len(userKey)
		part := buf[:n]
		i := copy(part, pwd)
		i += copy(part[i:], K[:])
		i += copy(part[i:], userKey)
		if i != n {
			panic("wrong size")
		}
		K1 := buf[:n*64]
		repeat(K1, n)

		ac := newAESCipher(K[0:16])
		cbc := cipher.NewCBCEncrypter(ac, K[16:32])
		cbc.CryptBlocks(K1, K1)
		E = K1

		b := 0
		for i := 0; i < 16; i++ {
			b += int(E[i] % 3)
		}
		var h hash.Hash
		switch b % 3 {
		case 0:
			h = s256
		case 1:
			if s384 == nil {
				s384 = sha512.New384()
			}
			h = s384
		case 2:
			if s512 == nil {
				s512 = sha512.New()
			}
			h = s512
		}

		h.Reset()
		h.Write(E)
		K = h.Sum(hbuf[:0])

		return E
	}

	for i := 0; ; {
		E := round(i)
		b := uint8(E[len(E)-1])

		i++
		if i >= 64 && b <= uint8(i-32) {
			break
		}
	}
	return K[:32]
}

func (sh stdHandlerR6) alg2b(R int, data, pwd, userKey []byte) []byte {
	if R == 5 {
		return alg2bR5(data)
	}
	return alg2b(data, pwd, userKey)
}

func (sh stdHandlerR6) alg8(d *StdEncryptDict, ekey []byte, upass []byte) error {
	if err := checkAtLeast("alg8", "Key", 32, ekey); err != nil {
		return err
	}

	var rbuf [16]byte
	if _, err := io.ReadFull(rand.Reader, rbuf[:]); err != nil {
		return err
	}
	valSalt := rbuf[0:8]
	keySalt := rbuf[8:16]

	str := make([]byte, len(upass)+len(valSalt))
	i := copy(str, upass)
	i += copy(str[i:], valSalt)

	h := sh.alg2b(d.R, str, upass, nil)

	U := make([]byte, len(h)+len(valSalt)+len(keySalt))
	i = copy(U, h[:32])
	i += copy(U[i:], valSalt)
	i += copy(U[i:], keySalt)

	d.U = U

	i = len(upass)
	i += copy(str[i:], keySalt)

	h = sh.alg2b(d.R, str, upass, nil)

	ac := newAESCipher(h[:32])

	iv := make([]byte, aes.BlockSize)
	cbc := cipher.NewCBCEncrypter(ac, iv)
	UE := make([]byte, 32)
	cbc.CryptBlocks(UE, ekey[:32])
	d.UE = UE

	return nil
}

func (sh stdHandlerR6) alg9(d *StdEncryptDict, ekey []byte, opass []byte) error {
	if err := checkAtLeast("alg9", "Key", 32, ekey); err != nil {
		return err
	}
	if err := checkAtLeast("alg9", "U", 48, d.U); err != nil {
		return err
	}

	var rbuf [16]byte
	if _, err := io.ReadFull(rand.Reader, rbuf[:]); err != nil {
		return err
	}
	valSalt := rbuf[0:8]
	keySalt := rbuf[8:16]
	userKey := d.U[:48]

	str := make([]byte, len(opass)+len(valSalt)+len(userKey))
	i := copy(str, opass)
	i += copy(str[i:], valSalt)
	i += copy(str[i:], userKey)

	h := sh.alg2b(d.R, str, opass, userKey)

	O := make([]byte, len(h)+len(valSalt)+len(keySalt))
	i = copy(O, h[:32])
	i += copy(O[i:], valSalt)
	i += copy(O[i:], keySalt)

	d.O = O

	i = len(opass)
	i += copy(str[i:], keySalt)

	h = sh.alg2b(d.R, str, opass, userKey)

	ac := newAESCipher(h[:32])

	iv := make([]byte, aes.BlockSize)
	cbc := cipher.NewCBCEncrypter(ac, iv)
	OE := make([]byte, 32)
	cbc.CryptBlocks(OE, ekey[:32])
	d.OE = OE

	return nil
}

func (sh stdHandlerR6) alg10(d *StdEncryptDict, ekey []byte) error {
	if err := checkAtLeast("alg10", "Key", 32, ekey); err != nil {
		return err
	}

	perms := uint64(uint32(d.P)) | (math.MaxUint32 << 32)

	Perms := make([]byte, 16)
	binary.LittleEndian.PutUint64(Perms[:8], perms)

	if d.EncryptMetadata {
		Perms[8] = 'T'
	} else {
		Perms[8] = 'F'
	}

	copy(Perms[9:12], "adb")

	if _, err := io.ReadFull(rand.Reader, Perms[12:16]); err != nil {
		return err
	}

	ac := newAESCipher(ekey[:32])

	ecb := newECBEncrypter(ac)
	ecb.CryptBlocks(Perms, Perms)

	d.Perms = Perms[:16]
	return nil
}

func (sh stdHandlerR6) alg11(d *StdEncryptDict, upass []byte) ([]byte, error) {
	if err := checkAtLeast("alg11", "U", 48, d.U); err != nil {
		return nil, err
	}
	str := make([]byte, len(upass)+8)
	i := copy(str, upass)
	i += copy(str[i:], d.U[32:40])

	h := sh.alg2b(d.R, str, upass, nil)
	h = h[:32]
	if !bytes.Equal(h, d.U[:32]) {
		return nil, nil
	}
	return h, nil
}

func (sh stdHandlerR6) alg12(d *StdEncryptDict, opass []byte) ([]byte, error) {
	if err := checkAtLeast("alg12", "U", 48, d.U); err != nil {
		return nil, err
	}
	if err := checkAtLeast("alg12", "O", 48, d.O); err != nil {
		return nil, err
	}
	str := make([]byte, len(opass)+8+48)
	i := copy(str, opass)
	i += copy(str[i:], d.O[32:40])
	i += copy(str[i:], d.U[0:48])

	h := sh.alg2b(d.R, str, opass, d.U[0:48])
	h = h[:32]
	if !bytes.Equal(h, d.O[:32]) {
		return nil, nil
	}
	return h, nil
}

func (sh stdHandlerR6) alg13(d *StdEncryptDict, fkey []byte) error {
	if err := checkAtLeast("alg13", "Key", 32, fkey); err != nil {
		return err
	}
	if err := checkAtLeast("alg13", "Perms", 16, d.Perms); err != nil {
		return err
	}
	perms := make([]byte, 16)
	copy(perms, d.Perms[:16])

	ac, err := aes.NewCipher(fkey[:32])
	if err != nil {
		return err
	}

	ecb := newECBDecrypter(ac)
	ecb.CryptBlocks(perms, perms)

	if !bytes.Equal(perms[9:12], []byte("adb")) {
		return errors.New("decoded permissions are invalid")
	}
	p := Permissions(binary.LittleEndian.Uint32(perms[0:4]))
	if p != d.P {
		return errors.New("permissions validation failed")
	}
	encMeta := true
	if perms[8] == 'T' {
		encMeta = true
	} else if perms[8] == 'F' {
		encMeta = false
	} else {
		return errors.New("decoded metadata encryption flag is invalid")
	}
	if encMeta != d.EncryptMetadata {
		return errors.New("metadata encryption validation failed")
	}
	return nil
}

func (sh stdHandlerR6) GenerateParams(d *StdEncryptDict, opass, upass []byte) ([]byte, error) {
	ekey := make([]byte, 32)
	if _, err := io.ReadFull(rand.Reader, ekey); err != nil {
		return nil, err
	}

	d.U = nil
	d.O = nil
	d.UE = nil
	d.OE = nil
	d.Perms = nil

	if len(upass) > 127 {
		upass = upass[:127]
	}
	if len(opass) > 127 {
		opass = opass[:127]
	}

	if err := sh.alg8(d, ekey, upass); err != nil {
		return nil, err
	}

	if err := sh.alg9(d, ekey, opass); err != nil {
		return nil, err
	}
	if d.R == 5 {
		return ekey, nil
	}

	if err := sh.alg10(d, ekey); err != nil {
		return nil, err
	}
	return ekey, nil
}

func (sh stdHandlerR6) Authenticate(d *StdEncryptDict, pass []byte) ([]byte, Permissions, error) {
	return sh.alg2a(d, pass)
}
