package cosy

import (
	"crypto/md5"
	"fmt"
	"time"
)

const (
	AppCode   = "cosy"
	SecretB64 = "d2FyLCB3YXIgbmV2ZXIgY2hhbmdlcw==" // base64("war, war never changes")
	sep       = "&"
)

func CurrentDate() string {
	return time.Now().UTC().Format("Mon, 02 Jan 2006 15:04:05 GMT")
}

func SignLegacy(date string) string {
	return Md5Hex(AppCode + sep + SecretB64 + sep + date)
}

func Md5Hex(s string) string {
	h := md5.Sum([]byte(s))
	return fmt.Sprintf("%x", h)
}
