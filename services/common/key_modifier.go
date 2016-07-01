package common

import (
	"encoding/base64"
	"fmt"
	"golang.org/x/crypto/sha3"
)

type ModifierFunc func(string) string

func UsernameModifier(username string) ModifierFunc {
	return func(key string) string {
		tmp := fmt.Sprintf("%s\x00%s", username, key)
		res := make([]byte, 64)

		sha3.ShakeSum256(res, []byte(tmp))
		return base64.URLEncoding.EncodeToString(res)
	}
}

func MetaModifier() ModifierFunc {
	return func(key string) string {
		return fmt.Sprintf("meta\x00%s", key)
	}
}

