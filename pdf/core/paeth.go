package core

const intSize = 32 << (^uint(0) >> 63)

func abs(x int) int {

	m := x >> (intSize - 1)

	return (x ^ m) - m
}

func paeth(a, b, c uint8) uint8 {

	pc := int(c)
	pa := int(b) - pc
	pb := int(a) - pc
	pc = abs(pa + pb)
	pa = abs(pa)
	pb = abs(pb)
	if pa <= pb && pa <= pc {
		return a
	} else if pb <= pc {
		return b
	}
	return c
}
