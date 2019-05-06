package core

import (
	"crypto/md5"
	"crypto/rand"
	"errors"
	"fmt"
	"time"

	"github.com/codefinio/doc/common"
	"github.com/codefinio/doc/pdf/core/security"
	crypto "github.com/codefinio/doc/pdf/core/security/crypt"
)

type EncryptInfo struct {
	Version

	Encrypt *PdfObjectDictionary

	ID0, ID1 string
}

func PdfCryptNewEncrypt(cf crypto.Filter, userPass, ownerPass []byte, perm security.Permissions) (*PdfCrypt, *EncryptInfo, error) {
	crypter := &PdfCrypt{
		encryptedObjects: make(map[PdfObject]bool),
		cryptFilters:     make(cryptFilters),
		encryptStd: security.StdEncryptDict{
			P:               perm,
			EncryptMetadata: true,
		},
	}
	var vers Version
	if cf != nil {
		v := cf.PDFVersion()
		vers.Major, vers.Minor = v[0], v[1]

		V, R := cf.HandlerVersion()
		crypter.encrypt.V = V
		crypter.encryptStd.R = R

		crypter.encrypt.Length = cf.KeyLength() * 8
	}
	const (
		defaultFilter = stdCryptFilter
	)
	crypter.cryptFilters[defaultFilter] = cf
	if crypter.encrypt.V >= 4 {
		crypter.streamFilter = defaultFilter
		crypter.stringFilter = defaultFilter
	}
	ed := crypter.newEncryptDict()

	hashcode := md5.Sum([]byte(time.Now().Format(time.RFC850)))
	id0 := string(hashcode[:])
	b := make([]byte, 100)
	rand.Read(b)
	hashcode = md5.Sum(b)
	id1 := string(hashcode[:])
	common.Log.Trace("Random b: % x", b)

	common.Log.Trace("Gen Id 0: % x", id0)

	crypter.id0 = string(id0)

	err := crypter.generateParams(userPass, ownerPass)
	if err != nil {
		return nil, nil, err
	}

	encodeEncryptStd(&crypter.encryptStd, ed)
	if crypter.encrypt.V >= 4 {
		if err := crypter.saveCryptFilters(ed); err != nil {
			return nil, nil, err
		}
	}

	return crypter, &EncryptInfo{
		Version: vers,
		Encrypt: ed,
		ID0:     id0, ID1: id1,
	}, nil
}

type PdfCrypt struct {
	encrypt    encryptDict
	encryptStd security.StdEncryptDict

	id0              string
	encryptionKey    []byte
	decryptedObjects map[PdfObject]bool
	encryptedObjects map[PdfObject]bool
	authenticated    bool

	cryptFilters cryptFilters
	streamFilter string
	stringFilter string

	parser *PdfParser

	decryptedObjNum map[int]struct{}
}

func encodeEncryptStd(d *security.StdEncryptDict, ed *PdfObjectDictionary) {
	ed.Set("R", MakeInteger(int64(d.R)))
	ed.Set("P", MakeInteger(int64(d.P)))

	ed.Set("O", MakeStringFromBytes(d.O))
	ed.Set("U", MakeStringFromBytes(d.U))
	if d.R >= 5 {
		ed.Set("OE", MakeStringFromBytes(d.OE))
		ed.Set("UE", MakeStringFromBytes(d.UE))
		ed.Set("EncryptMetadata", MakeBool(d.EncryptMetadata))
		if d.R > 5 {
			ed.Set("Perms", MakeStringFromBytes(d.Perms))
		}
	}
}

func decodeEncryptStd(d *security.StdEncryptDict, ed *PdfObjectDictionary) error {

	R, ok := ed.Get("R").(*PdfObjectInteger)
	if !ok {
		return errors.New("encrypt dictionary missing R")
	}

	if *R < 2 || *R > 6 {
		return fmt.Errorf("invalid R (%d)", *R)
	}
	d.R = int(*R)

	O, ok := ed.GetString("O")
	if !ok {
		return errors.New("encrypt dictionary missing O")
	}
	if d.R == 5 || d.R == 6 {

		if len(O) < 48 {
			return fmt.Errorf("Length(O) < 48 (%d)", len(O))
		}
	} else if len(O) != 32 {
		return fmt.Errorf("Length(O) != 32 (%d)", len(O))
	}
	d.O = []byte(O)

	U, ok := ed.GetString("U")
	if !ok {
		return errors.New("encrypt dictionary missing U")
	}
	if d.R == 5 || d.R == 6 {

		if len(U) < 48 {
			return fmt.Errorf("Length(U) < 48 (%d)", len(U))
		}
	} else if len(U) != 32 {

		common.Log.Debug("Warning: Length(U) != 32 (%d)", len(U))

	}
	d.U = []byte(U)

	if d.R >= 5 {
		OE, ok := ed.GetString("OE")
		if !ok {
			return errors.New("encrypt dictionary missing OE")
		} else if len(OE) != 32 {
			return fmt.Errorf("Length(OE) != 32 (%d)", len(OE))
		}
		d.OE = []byte(OE)

		UE, ok := ed.GetString("UE")
		if !ok {
			return errors.New("encrypt dictionary missing UE")
		} else if len(UE) != 32 {
			return fmt.Errorf("Length(UE) != 32 (%d)", len(UE))
		}
		d.UE = []byte(UE)
	}

	P, ok := ed.Get("P").(*PdfObjectInteger)
	if !ok {
		return errors.New("encrypt dictionary missing permissions attr")
	}
	d.P = security.Permissions(*P)

	if d.R == 6 {
		Perms, ok := ed.GetString("Perms")
		if !ok {
			return errors.New("encrypt dictionary missing Perms")
		} else if len(Perms) != 16 {
			return fmt.Errorf("Length(Perms) != 16 (%d)", len(Perms))
		}
		d.Perms = []byte(Perms)
	}

	if em, ok := ed.Get("EncryptMetadata").(*PdfObjectBool); ok {
		d.EncryptMetadata = bool(*em)
	} else {
		d.EncryptMetadata = true
	}
	return nil
}

func decodeCryptFilter(cf *crypto.FilterDict, d *PdfObjectDictionary) error {

	if typename, ok := d.Get("Type").(*PdfObjectName); ok {
		if string(*typename) != "CryptFilter" {
			return fmt.Errorf("CF dict type != CryptFilter (%s)", typename)
		}
	}

	name, ok := d.Get("CFM").(*PdfObjectName)
	if !ok {
		return fmt.Errorf("unsupported crypt filter (None)")
	}
	cf.CFM = string(*name)

	if event, ok := d.Get("AuthEvent").(*PdfObjectName); ok {
		cf.AuthEvent = security.AuthEvent(*event)
	} else {
		cf.AuthEvent = security.EventDocOpen
	}

	if length, ok := d.Get("Length").(*PdfObjectInteger); ok {
		cf.Length = int(*length)
	}
	return nil
}

func (crypt *PdfCrypt) newEncryptDict() *PdfObjectDictionary {

	ed := MakeDict()
	ed.Set("Filter", MakeName("Standard"))
	ed.Set("V", MakeInteger(int64(crypt.encrypt.V)))
	ed.Set("Length", MakeInteger(int64(crypt.encrypt.Length)))
	return ed
}

func (crypt *PdfCrypt) String() string {
	if crypt == nil {
		return ""
	}

	str := crypt.encrypt.Filter + " - "

	if crypt.encrypt.V == 0 {
		str += "Undocumented algorithm"
	} else if crypt.encrypt.V == 1 {

		str += "RC4: 40 bits"
	} else if crypt.encrypt.V == 2 {
		str += fmt.Sprintf("RC4: %d bits", crypt.encrypt.Length)
	} else if crypt.encrypt.V == 3 {
		str += "Unpublished algorithm"
	} else if crypt.encrypt.V >= 4 {

		str += fmt.Sprintf("Stream filter: %s - String filter: %s", crypt.streamFilter, crypt.stringFilter)
		str += "; Crypt filters:"
		for name, cf := range crypt.cryptFilters {
			str += fmt.Sprintf(" - %s: %s (%d)", name, cf.Name(), cf.KeyLength())
		}
	}
	perms := crypt.GetAccessPermissions()
	str += fmt.Sprintf(" - %#v", perms)

	return str
}

type encryptDict struct {
	Filter    string
	V         int
	SubFilter string
	Length    int

	StmF string
	StrF string
	EFF  string

	CF map[string]crypto.FilterDict
}

const stdCryptFilter = "StdCF"

func newCryptFiltersV2(length int) cryptFilters {
	return cryptFilters{
		stdCryptFilter: crypto.NewFilterV2(length),
	}
}

type cryptFilters map[string]crypto.Filter

func (crypt *PdfCrypt) loadCryptFilters(ed *PdfObjectDictionary) error {
	crypt.cryptFilters = cryptFilters{}

	obj := ed.Get("CF")
	obj = TraceToDirectObject(obj)
	if ref, isRef := obj.(*PdfObjectReference); isRef {
		o, err := crypt.parser.LookupByReference(*ref)
		if err != nil {
			common.Log.Debug("Error looking up CF reference")
			return err
		}
		obj = TraceToDirectObject(o)
	}

	cf, ok := obj.(*PdfObjectDictionary)
	if !ok {
		common.Log.Debug("Invalid CF, type: %T", obj)
		return errors.New("invalid CF")
	}

	for _, name := range cf.Keys() {
		v := cf.Get(name)

		if ref, isRef := v.(*PdfObjectReference); isRef {
			o, err := crypt.parser.LookupByReference(*ref)
			if err != nil {
				common.Log.Debug("Error lookup up dictionary reference")
				return err
			}
			v = TraceToDirectObject(o)
		}

		dict, ok := v.(*PdfObjectDictionary)
		if !ok {
			return fmt.Errorf("invalid dict in CF (name %s) - not a dictionary but %T", name, v)
		}

		if name == "Identity" {
			common.Log.Debug("ERROR - Cannot overwrite the identity filter - Trying next")
			continue
		}

		var cfd crypto.FilterDict
		if err := decodeCryptFilter(&cfd, dict); err != nil {
			return err
		}
		cf, err := crypto.NewFilter(cfd)
		if err != nil {
			return err
		}
		crypt.cryptFilters[string(name)] = cf
	}

	crypt.cryptFilters["Identity"] = crypto.NewIdentity()

	crypt.stringFilter = "Identity"
	if strf, ok := ed.Get("StrF").(*PdfObjectName); ok {
		if _, exists := crypt.cryptFilters[string(*strf)]; !exists {
			return fmt.Errorf("crypt filter for StrF not specified in CF dictionary (%s)", *strf)
		}
		crypt.stringFilter = string(*strf)
	}

	crypt.streamFilter = "Identity"
	if stmf, ok := ed.Get("StmF").(*PdfObjectName); ok {
		if _, exists := crypt.cryptFilters[string(*stmf)]; !exists {
			return fmt.Errorf("crypt filter for StmF not specified in CF dictionary (%s)", *stmf)
		}
		crypt.streamFilter = string(*stmf)
	}

	return nil
}

func encodeCryptFilter(cf crypto.Filter, event security.AuthEvent) *PdfObjectDictionary {
	if event == "" {
		event = security.EventDocOpen
	}
	v := MakeDict()
	v.Set("Type", MakeName("CryptFilter"))
	v.Set("AuthEvent", MakeName(string(event)))
	v.Set("CFM", MakeName(cf.Name()))
	v.Set("Length", MakeInteger(int64(cf.KeyLength())))
	return v
}

func (crypt *PdfCrypt) saveCryptFilters(ed *PdfObjectDictionary) error {
	if crypt.encrypt.V < 4 {
		return errors.New("can only be used with V>=4")
	}
	cf := MakeDict()
	ed.Set("CF", cf)

	for name, filter := range crypt.cryptFilters {
		if name == "Identity" {
			continue
		}
		v := encodeCryptFilter(filter, "")
		cf.Set(PdfObjectName(name), v)
	}
	ed.Set("StrF", MakeName(crypt.stringFilter))
	ed.Set("StmF", MakeName(crypt.streamFilter))
	return nil
}

func PdfCryptNewDecrypt(parser *PdfParser, ed, trailer *PdfObjectDictionary) (*PdfCrypt, error) {
	crypter := &PdfCrypt{
		authenticated:    false,
		decryptedObjects: make(map[PdfObject]bool),
		encryptedObjects: make(map[PdfObject]bool),
		decryptedObjNum:  make(map[int]struct{}),
		parser:           parser,
	}

	filter, ok := ed.Get("Filter").(*PdfObjectName)
	if !ok {
		common.Log.Debug("ERROR Crypt dictionary missing required Filter field!")
		return crypter, errors.New("required crypt field Filter missing")
	}
	if *filter != "Standard" {
		common.Log.Debug("ERROR Unsupported filter (%s)", *filter)
		return crypter, errors.New("unsupported Filter")
	}
	crypter.encrypt.Filter = string(*filter)

	if subfilter, ok := ed.Get("SubFilter").(*PdfObjectString); ok {
		crypter.encrypt.SubFilter = subfilter.Str()
		common.Log.Debug("Using subfilter %s", subfilter)
	}

	if L, ok := ed.Get("Length").(*PdfObjectInteger); ok {
		if (*L % 8) != 0 {
			common.Log.Debug("ERROR Invalid encryption length")
			return crypter, errors.New("invalid encryption length")
		}
		crypter.encrypt.Length = int(*L)
	} else {
		crypter.encrypt.Length = 40
	}

	crypter.encrypt.V = 0
	if v, ok := ed.Get("V").(*PdfObjectInteger); ok {
		V := int(*v)
		crypter.encrypt.V = V
		if V >= 1 && V <= 2 {

			crypter.cryptFilters = newCryptFiltersV2(crypter.encrypt.Length)
		} else if V >= 4 && V <= 5 {
			if err := crypter.loadCryptFilters(ed); err != nil {
				return crypter, err
			}
		} else {
			common.Log.Debug("ERROR Unsupported encryption algo V = %d", V)
			return crypter, errors.New("unsupported algorithm")
		}
	}

	if err := decodeEncryptStd(&crypter.encryptStd, ed); err != nil {
		return crypter, err
	}

	id0 := ""
	if idArray, ok := trailer.Get("ID").(*PdfObjectArray); ok && idArray.Len() >= 1 {
		id0obj, ok := GetString(idArray.Get(0))
		if !ok {
			return crypter, errors.New("invalid trailer ID")
		}
		id0 = id0obj.Str()
	} else {
		common.Log.Debug("Trailer ID array missing or invalid!")
	}
	crypter.id0 = id0

	return crypter, nil
}

func (crypt *PdfCrypt) GetAccessPermissions() security.Permissions {
	return crypt.encryptStd.P
}

func (crypt *PdfCrypt) securityHandler() security.StdHandler {
	if crypt.encryptStd.R >= 5 {
		return security.NewHandlerR6()
	}
	return security.NewHandlerR4(crypt.id0, crypt.encrypt.Length)
}

func (crypt *PdfCrypt) authenticate(password []byte) (bool, error) {
	crypt.authenticated = false
	h := crypt.securityHandler()
	fkey, perm, err := h.Authenticate(&crypt.encryptStd, password)
	if err != nil {
		return false, err
	} else if perm == 0 || len(fkey) == 0 {
		return false, nil
	}
	crypt.authenticated = true
	crypt.encryptionKey = fkey
	return true, nil
}

func (crypt *PdfCrypt) checkAccessRights(password []byte) (bool, security.Permissions, error) {
	h := crypt.securityHandler()

	fkey, perm, err := h.Authenticate(&crypt.encryptStd, password)
	if err != nil {
		return false, 0, err
	} else if perm == 0 || len(fkey) == 0 {
		return false, 0, nil
	}
	return true, perm, nil
}

func (crypt *PdfCrypt) makeKey(filter string, objNum, genNum uint32, ekey []byte) ([]byte, error) {
	f, ok := crypt.cryptFilters[filter]
	if !ok {
		return nil, fmt.Errorf("unknown crypt filter (%s)", filter)
	}
	return f.MakeKey(objNum, genNum, ekey)
}

var encryptDictKeys = []PdfObjectName{
	"V", "R", "O", "U", "P",
}

func (crypt *PdfCrypt) isDecrypted(obj PdfObject) bool {
	_, ok := crypt.decryptedObjects[obj]
	if ok {
		common.Log.Trace("Already decrypted")
		return true
	}
	switch obj := obj.(type) {
	case *PdfObjectStream:
		if crypt.encryptStd.R != 5 {
			if name, ok := obj.Get("Type").(*PdfObjectName); ok && *name == "XRef" {
				return true
			}
		}
	case *PdfIndirectObject:
		if _, ok = crypt.decryptedObjNum[int(obj.ObjectNumber)]; ok {
			return true
		}
		switch obj := obj.PdfObject.(type) {
		case *PdfObjectDictionary:

			ok := true
			for _, key := range encryptDictKeys {
				if obj.Get(key) == nil {
					ok = false
					break
				}
			}
			if ok {
				return true
			}
		}
	}

	common.Log.Trace("Not decrypted yet")
	return false
}

func (crypt *PdfCrypt) decryptBytes(buf []byte, filter string, okey []byte) ([]byte, error) {
	common.Log.Trace("Decrypt bytes")
	f, ok := crypt.cryptFilters[filter]
	if !ok {
		return nil, fmt.Errorf("unknown crypt filter (%s)", filter)
	}
	return f.DecryptBytes(buf, okey)
}

func (crypt *PdfCrypt) Decrypt(obj PdfObject, parentObjNum, parentGenNum int64) error {
	if crypt.isDecrypted(obj) {
		return nil
	}

	switch obj := obj.(type) {
	case *PdfIndirectObject:
		crypt.decryptedObjects[obj] = true

		common.Log.Trace("Decrypting indirect %d %d obj!", obj.ObjectNumber, obj.GenerationNumber)

		objNum := obj.ObjectNumber
		genNum := obj.GenerationNumber

		err := crypt.Decrypt(obj.PdfObject, objNum, genNum)
		if err != nil {
			return err
		}
		return nil
	case *PdfObjectStream:

		crypt.decryptedObjects[obj] = true
		dict := obj.PdfObjectDictionary

		if crypt.encryptStd.R != 5 {
			if s, ok := dict.Get("Type").(*PdfObjectName); ok && *s == "XRef" {
				return nil
			}
		}

		objNum := obj.ObjectNumber
		genNum := obj.GenerationNumber
		common.Log.Trace("Decrypting stream %d %d !", objNum, genNum)

		streamFilter := stdCryptFilter
		if crypt.encrypt.V >= 4 {
			streamFilter = crypt.streamFilter
			common.Log.Trace("this.streamFilter = %s", crypt.streamFilter)

			if filters, ok := dict.Get("Filter").(*PdfObjectArray); ok {

				if firstFilter, ok := GetName(filters.Get(0)); ok {
					if *firstFilter == "Crypt" {

						streamFilter = "Identity"

						if decodeParams, ok := dict.Get("DecodeParms").(*PdfObjectDictionary); ok {
							if filterName, ok := decodeParams.Get("Name").(*PdfObjectName); ok {
								if _, ok := crypt.cryptFilters[string(*filterName)]; ok {
									common.Log.Trace("Using stream filter %s", *filterName)
									streamFilter = string(*filterName)
								}
							}
						}
					}
				}
			}

			common.Log.Trace("with %s filter", streamFilter)
			if streamFilter == "Identity" {

				return nil
			}
		}

		err := crypt.Decrypt(dict, objNum, genNum)
		if err != nil {
			return err
		}

		okey, err := crypt.makeKey(streamFilter, uint32(objNum), uint32(genNum), crypt.encryptionKey)
		if err != nil {
			return err
		}

		obj.Stream, err = crypt.decryptBytes(obj.Stream, streamFilter, okey)
		if err != nil {
			return err
		}

		dict.Set("Length", MakeInteger(int64(len(obj.Stream))))

		return nil
	case *PdfObjectString:
		common.Log.Trace("Decrypting string!")

		stringFilter := stdCryptFilter
		if crypt.encrypt.V >= 4 {

			common.Log.Trace("with %s filter", crypt.stringFilter)
			if crypt.stringFilter == "Identity" {

				return nil
			}
			stringFilter = crypt.stringFilter
		}

		key, err := crypt.makeKey(stringFilter, uint32(parentObjNum), uint32(parentGenNum), crypt.encryptionKey)
		if err != nil {
			return err
		}

		str := obj.Str()
		decrypted := make([]byte, len(str))
		for i := 0; i < len(str); i++ {
			decrypted[i] = str[i]
		}
		common.Log.Trace("Decrypt string: %s : % x", decrypted, decrypted)
		decrypted, err = crypt.decryptBytes(decrypted, stringFilter, key)
		if err != nil {
			return err
		}
		obj.val = string(decrypted)

		return nil
	case *PdfObjectArray:
		for _, o := range obj.Elements() {
			err := crypt.Decrypt(o, parentObjNum, parentGenNum)
			if err != nil {
				return err
			}
		}
		return nil
	case *PdfObjectDictionary:
		isSig := false
		if t := obj.Get("Type"); t != nil {
			typeStr, ok := t.(*PdfObjectName)
			if ok && *typeStr == "Sig" {
				isSig = true
			}
		}
		for _, keyidx := range obj.Keys() {
			o := obj.Get(keyidx)

			if isSig && string(keyidx) == "Contents" {

				continue
			}

			if string(keyidx) != "Parent" && string(keyidx) != "Prev" && string(keyidx) != "Last" {
				err := crypt.Decrypt(o, parentObjNum, parentGenNum)
				if err != nil {
					return err
				}
			}
		}
		return nil
	}

	return nil
}

func (crypt *PdfCrypt) isEncrypted(obj PdfObject) bool {
	_, ok := crypt.encryptedObjects[obj]
	if ok {
		common.Log.Trace("Already encrypted")
		return true
	}

	common.Log.Trace("Not encrypted yet")
	return false
}

func (crypt *PdfCrypt) encryptBytes(buf []byte, filter string, okey []byte) ([]byte, error) {
	common.Log.Trace("Encrypt bytes")
	f, ok := crypt.cryptFilters[filter]
	if !ok {
		return nil, fmt.Errorf("unknown crypt filter (%s)", filter)
	}
	return f.EncryptBytes(buf, okey)
}

func (crypt *PdfCrypt) Encrypt(obj PdfObject, parentObjNum, parentGenNum int64) error {
	if crypt.isEncrypted(obj) {
		return nil
	}
	switch obj := obj.(type) {
	case *PdfIndirectObject:
		crypt.encryptedObjects[obj] = true

		common.Log.Trace("Encrypting indirect %d %d obj!", obj.ObjectNumber, obj.GenerationNumber)

		objNum := obj.ObjectNumber
		genNum := obj.GenerationNumber

		err := crypt.Encrypt(obj.PdfObject, objNum, genNum)
		if err != nil {
			return err
		}
		return nil
	case *PdfObjectStream:
		crypt.encryptedObjects[obj] = true
		dict := obj.PdfObjectDictionary

		if s, ok := dict.Get("Type").(*PdfObjectName); ok && *s == "XRef" {
			return nil
		}

		objNum := obj.ObjectNumber
		genNum := obj.GenerationNumber
		common.Log.Trace("Encrypting stream %d %d !", objNum, genNum)

		streamFilter := stdCryptFilter
		if crypt.encrypt.V >= 4 {

			streamFilter = crypt.streamFilter
			common.Log.Trace("this.streamFilter = %s", crypt.streamFilter)

			if filters, ok := dict.Get("Filter").(*PdfObjectArray); ok {

				if firstFilter, ok := GetName(filters.Get(0)); ok {
					if *firstFilter == "Crypt" {

						streamFilter = "Identity"

						if decodeParams, ok := dict.Get("DecodeParms").(*PdfObjectDictionary); ok {
							if filterName, ok := decodeParams.Get("Name").(*PdfObjectName); ok {
								if _, ok := crypt.cryptFilters[string(*filterName)]; ok {
									common.Log.Trace("Using stream filter %s", *filterName)
									streamFilter = string(*filterName)
								}
							}
						}
					}
				}
			}

			common.Log.Trace("with %s filter", streamFilter)
			if streamFilter == "Identity" {

				return nil
			}
		}

		err := crypt.Encrypt(obj.PdfObjectDictionary, objNum, genNum)
		if err != nil {
			return err
		}

		okey, err := crypt.makeKey(streamFilter, uint32(objNum), uint32(genNum), crypt.encryptionKey)
		if err != nil {
			return err
		}

		obj.Stream, err = crypt.encryptBytes(obj.Stream, streamFilter, okey)
		if err != nil {
			return err
		}

		dict.Set("Length", MakeInteger(int64(len(obj.Stream))))

		return nil
	case *PdfObjectString:
		common.Log.Trace("Encrypting string!")

		stringFilter := stdCryptFilter
		if crypt.encrypt.V >= 4 {
			common.Log.Trace("with %s filter", crypt.stringFilter)
			if crypt.stringFilter == "Identity" {

				return nil
			}
			stringFilter = crypt.stringFilter
		}

		key, err := crypt.makeKey(stringFilter, uint32(parentObjNum), uint32(parentGenNum), crypt.encryptionKey)
		if err != nil {
			return err
		}

		str := obj.Str()
		encrypted := make([]byte, len(str))
		for i := 0; i < len(str); i++ {
			encrypted[i] = str[i]
		}
		common.Log.Trace("Encrypt string: %s : % x", encrypted, encrypted)
		encrypted, err = crypt.encryptBytes(encrypted, stringFilter, key)
		if err != nil {
			return err
		}
		obj.val = string(encrypted)

		return nil
	case *PdfObjectArray:
		for _, o := range obj.Elements() {
			err := crypt.Encrypt(o, parentObjNum, parentGenNum)
			if err != nil {
				return err
			}
		}
		return nil
	case *PdfObjectDictionary:
		isSig := false
		if t := obj.Get("Type"); t != nil {
			typeStr, ok := t.(*PdfObjectName)
			if ok && *typeStr == "Sig" {
				isSig = true
			}
		}

		for _, keyidx := range obj.Keys() {
			o := obj.Get(keyidx)

			if isSig && string(keyidx) == "Contents" {

				continue
			}
			if string(keyidx) != "Parent" && string(keyidx) != "Prev" && string(keyidx) != "Last" {
				err := crypt.Encrypt(o, parentObjNum, parentGenNum)
				if err != nil {
					return err
				}
			}
		}
		return nil
	}

	return nil
}

func (crypt *PdfCrypt) generateParams(upass, opass []byte) error {
	h := crypt.securityHandler()
	ekey, err := h.GenerateParams(&crypt.encryptStd, opass, upass)
	if err != nil {
		return err
	}
	crypt.encryptionKey = ekey
	return nil
}
