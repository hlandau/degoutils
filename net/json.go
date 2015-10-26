package net

import "encoding/base64"
import "encoding/json"

type Base64 []byte

func (b *Base64) UnmarshalJSON(data []byte) error {
	var s string
	err := json.Unmarshal(data, &s)
	if err != nil {
		return err
	}
	*b, err = base64.StdEncoding.DecodeString(s)
	return err
}

func (b Base64) MarshalJSON() ([]byte, error) {
	return json.Marshal(base64.StdEncoding.EncodeToString(b))
}
