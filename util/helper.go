package util

import (
	"encoding/json"
	"reflect"
	"runtime"
)

func EncodeUser(v interface{}) string {
	js, _ := json.Marshal(v)
	return string(js)
}

func DecodeUser(js string, str interface{}) error {
	return json.Unmarshal([]byte(js), str)
}

func GetFunctionName(i interface{}) string {
	return runtime.FuncForPC(reflect.ValueOf(i).Pointer()).Name()
}
