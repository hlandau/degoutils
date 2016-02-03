package errorhandler

import "encoding/base64"
import "golang.org/x/crypto/nacl/secretbox"
import "crypto/rand"

// Generic, insecure encryption key. To be changed by specific initializers.
var ErrorEncryptionKey = [32]byte{47, 10, 181, 208, 144, 131, 217, 3, 142, 89, 158, 11, 133, 223, 34, 122, 231, 78, 10, 89, 77, 176, 202, 157, 164, 26, 2, 243, 3, 11, 187, 218}

func encryptError(info []byte) []byte {
	out := []byte{18, 147, 175, 43}
	var nonce [24]byte
	rand.Read(nonce[:])
	out = append(out, nonce[:]...)
	out = secretbox.Seal(out, info, &nonce, &ErrorEncryptionKey)
	return out
}

func encryptErrorBase64(info []byte) string {
	encryptedInfo := encryptError(info)
	return wrapBase64(base64.StdEncoding.EncodeToString(encryptedInfo))
}

func wrapBase64(data string) string {
	s := ""
	const lineLen = 70
	for i := 0; i < len(data); i++ {
		L := len(data[i:])
		if L > lineLen {
			L = lineLen
		}

		s += data[i : i+L]
		s += "\n"
		i += lineLen
	}

	return s
}
