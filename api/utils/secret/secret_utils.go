package secret

// Obfuscate Will hide parts of the data with a masking character
func Obfuscate(data string, from, length int, maskingCharacter rune) string {
	obfuscatedPart := FixedStringRunes(length, maskingCharacter)
	runes := []rune(data)
	clearPrefix := string(runes[0 : from-1])
	clearPostfix := string(runes[from+length-1 : len(data)])

	return clearPrefix + obfuscatedPart + clearPostfix
}

// FixedStringRunes Create a string of fixed number of characters
func FixedStringRunes(n int, character rune) string {
	b := make([]rune, n)
	for i := range b {
		b[i] = character
	}
	return string(b)
}
