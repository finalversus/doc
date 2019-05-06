package core

func IsWhiteSpace(ch byte) bool {

	if (ch == 0x00) || (ch == 0x09) || (ch == 0x0A) || (ch == 0x0C) || (ch == 0x0D) || (ch == 0x20) {
		return true
	}
	return false
}

func IsFloatDigit(c byte) bool {
	return ('0' <= c && c <= '9') || c == '.'
}

func IsDecimalDigit(c byte) bool {
	if c >= '0' && c <= '9' {
		return true
	}

	return false
}

func IsOctalDigit(c byte) bool {
	if c >= '0' && c <= '7' {
		return true
	}

	return false
}

func IsPrintable(char byte) bool {
	if char < 0x21 || char > 0x7E {
		return false
	}
	return true
}

func IsDelimiter(char byte) bool {
	if char == '(' || char == ')' {
		return true
	}
	if char == '<' || char == '>' {
		return true
	}
	if char == '[' || char == ']' {
		return true
	}
	if char == '{' || char == '}' {
		return true
	}
	if char == '/' {
		return true
	}
	if char == '%' {
		return true
	}

	return false
}
