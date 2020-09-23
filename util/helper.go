package util

import (
	"encoding/json"
)

func EncodeUser(v interface{}) string {
	js, _ := json.Marshal(v)
	return string(js)
}

func DecodeUser(js string, str interface{}) error {
	return json.Unmarshal([]byte(js), str)
}
