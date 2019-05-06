package security

import "math"

type Permissions uint32

const (
	PermOwner = Permissions(math.MaxUint32)

	PermPrinting = Permissions(1 << 2)

	PermModify = Permissions(1 << 3)

	PermExtractGraphics = Permissions(1 << 4)

	PermAnnotate = Permissions(1 << 5)

	PermFillForms = Permissions(1 << 8)

	PermDisabilityExtract = Permissions(1 << 9)

	PermRotateInsert = Permissions(1 << 10)

	PermFullPrintQuality = Permissions(1 << 11)
)

func (p Permissions) Allowed(p2 Permissions) bool {
	return p&p2 == p2
}
