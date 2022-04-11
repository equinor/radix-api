package secret

import (
	"fmt"
	v1 "github.com/equinor/radix-operator/pkg/apis/radix/v1"
)

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

//GetSecretNameForAzureKeyVaultItem Get the name of the secret by Azure Key vault item properties
func GetSecretNameForAzureKeyVaultItem(azureKeyVaultName string, item *v1.RadixAzureKeyVaultItem) string {
	displayName := fmt.Sprintf("AzureKeyVaultItem-%s--%s--%s", azureKeyVaultName, getAzureKeyVaultItemType(item), item.Name)
	return displayName
}

//GetSecretDisplayNameForAzureKeyVaultItem Get the display name of the secret by Azure Key vault item properties
func GetSecretDisplayNameForAzureKeyVaultItem(item *v1.RadixAzureKeyVaultItem) string {
	displayName := fmt.Sprintf("%s '%s'", getAzureKeyVaultItemType(item), item.Name)
	if item.Alias != nil && len(*item.Alias) > 0 {
		displayName = fmt.Sprintf("%s, file '%s'", displayName, *item.Alias)
	}
	return displayName
}

func getAzureKeyVaultItemType(item *v1.RadixAzureKeyVaultItem) string {
	if item.Type != nil {
		return string(*item.Type)
	}
	return string(v1.RadixAzureKeyVaultObjectTypeSecret)
}
