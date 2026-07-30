package cosy

import (
	"encoding/base64"
	"fmt"
)

const (
	customAlphabet = "_doRTgHZBKcGVjlvpC,@aFSx#DPuNJme&i*MzLOEn)sUrthbf%Y^w.(kIQyXqWA!"
	stdAlphabet    = "ABCDEFGHIJKLMNOPQRSTUVWXYZabcdefghijklmnopqrstuvwxyz0123456789+/"
	customPad      = '$'
)

var c2s [128]int
var s2c [128]int

func init() {
	for i := range c2s {
		c2s[i] = -1
		s2c[i] = -1
	}
	for i := 0; i < 64; i++ {
		c2s[customAlphabet[i]] = int(stdAlphabet[i])
		s2c[stdAlphabet[i]] = int(customAlphabet[i])
	}
	c2s[customPad] = '='
	s2c['='] = customPad
}

func Encode(plaintext []byte) (string, error) {
	std := base64.StdEncoding.EncodeToString(plaintext)
	n := len(std)
	a := n / 3
	rearranged := std[n-a:] + std[a:n-a] + std[:a]
	out := make([]byte, n)
	for i := 0; i < n; i++ {
		c := int(rearranged[i])
		if c >= 128 || s2c[c] < 0 {
			return "", fmt.Errorf("char out of alphabet: %d", c)
		}
		out[i] = byte(s2c[c])
	}
	return string(out), nil
}

func Decode(encoded string) ([]byte, error) {
	n := len(encoded)
	mapped := make([]byte, n)
	for i := 0; i < n; i++ {
		c := int(encoded[i])
		if c >= 128 || c2s[c] < 0 {
			return nil, fmt.Errorf("char out of custom alphabet: %d", c)
		}
		mapped[i] = byte(c2s[c])
	}
	a := n / 3
	std := string(mapped[n-a:]) + string(mapped[a:n-a]) + string(mapped[:a])
	return base64.StdEncoding.DecodeString(std)
}
