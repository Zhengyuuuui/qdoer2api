package cosy

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"encoding/base64"
	"encoding/json"
	"encoding/pem"
	"fmt"
	"strings"
	"time"
)

const Version = "1.0.10"

const ServerPubKeyPEM = `-----BEGIN PUBLIC KEY-----
MIGfMA0GCSqGSIb3DQEBAQUAA4GNADCBiQKBgQDA8iMH5c02LilrsERw9t6Pv5Nc
4k6Pz1EaDicBMpdpxKduSZu5OANqUq8er4GM95omAGIOPOh+Nx0spthYA2BqGz+l
6HRkPJ7S236FZz73In/KVuLnwI8JJ2CbuJap8kvheCCZpmAWpb/cPx/3Vr/J6I17
XcW+ML9FoCI6AOvOzwIDAQAB
-----END PUBLIC KEY-----`

type AuthIdentity struct {
	Name               string
	Aid                string
	Uid                string
	YxUid              string
	OrganizationId     string
	OrganizationName   string
	UserType           string
	SecurityOauthToken string
	RefreshToken       string
}

type SessionContext struct {
	TempKey      []byte
	CosyKey      string
	Info         string
	Identity     AuthIdentity
	MachineId    string
	MachineToken string
	MachineType  string
}

func NewSession(id AuthIdentity, machineId, machineToken, machineType string) (*SessionContext, error) {
	raw := make([]byte, 16)
	if _, err := rand.Read(raw); err != nil {
		return nil, err
	}
	tempKey := []byte(fmt.Sprintf("%x", raw)[:16])

	cosyKeyBytes, err := rsaEncrypt(tempKey)
	if err != nil {
		return nil, err
	}
	cosyKey := base64.StdEncoding.EncodeToString(cosyKeyBytes)

	payloadBytes, err := authPayloadJSON(id)
	if err != nil {
		return nil, err
	}
	encPayload, err := aesEncrypt(payloadBytes, tempKey)
	if err != nil {
		return nil, err
	}
	info := base64.StdEncoding.EncodeToString(encPayload)

	return &SessionContext{
		TempKey:      tempKey,
		CosyKey:      cosyKey,
		Info:         info,
		Identity:     id,
		MachineId:    machineId,
		MachineToken: machineToken,
		MachineType:  machineType,
	}, nil
}

func SignRequest(payloadB64, cosyKey, cosyDate, body, pathWithoutAlgo string) string {
	s := payloadB64 + "\n" + cosyKey + "\n" + cosyDate + "\n" + body + "\n" + pathWithoutAlgo
	return Md5Hex(s)
}

func BuildPayloadB64(info string) (string, error) {
	m := map[string]string{
		"cosyVersion": Version,
		"ideVersion":  "",
		"info":        info,
		"requestId":   NewUUID(),
		"version":     "v1",
	}
	data, err := json.Marshal(m)
	if err != nil {
		return "", err
	}
	return base64.StdEncoding.EncodeToString(data), nil
}

func ComposeBearer(payloadB64, sig string) string {
	return "Bearer COSY." + payloadB64 + "." + sig
}

func authPayloadJSON(id AuthIdentity) ([]byte, error) {
	m := map[string]string{
		"name":                 id.Name,
		"aid":                  id.Aid,
		"uid":                  id.Uid,
		"yx_uid":               id.YxUid,
		"organization_id":      id.OrganizationId,
		"organization_name":    id.OrganizationName,
		"user_type":            id.UserType,
		"security_oauth_token": id.SecurityOauthToken,
		"refresh_token":        id.RefreshToken,
	}
	return json.Marshal(m)
}

func rsaEncrypt(plaintext []byte) ([]byte, error) {
	pemData := strings.ReplaceAll(ServerPubKeyPEM, "\r", "")
	block, _ := pem.Decode([]byte(pemData))
	if block == nil {
		return nil, fmt.Errorf("failed to decode PEM")
	}
	pub, err := x509.ParsePKIXPublicKey(block.Bytes)
	if err != nil {
		return nil, err
	}
	rsaPub, ok := pub.(*rsa.PublicKey)
	if !ok {
		return nil, fmt.Errorf("not RSA public key")
	}
	return rsa.EncryptPKCS1v15(rand.Reader, rsaPub, plaintext)
}

func aesEncrypt(plain, key []byte) ([]byte, error) {
	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, err
	}
	bs := block.BlockSize()
	pad := bs - len(plain)%bs
	padded := make([]byte, len(plain)+pad)
	copy(padded, plain)
	for i := len(plain); i < len(padded); i++ {
		padded[i] = byte(pad)
	}
	out := make([]byte, len(padded))
	iv := key[:bs]
	mode := cipher.NewCBCEncrypter(block, iv)
	mode.CryptBlocks(out, padded)
	return out, nil
}

func NewUUID() string {
	b := make([]byte, 16)
	rand.Read(b)
	return fmt.Sprintf("%08x-%04x-%04x-%04x-%012x",
		b[0:4], b[4:6], b[6:8], b[8:10], b[10:16])
}

func NewRequestID() string {
	b := make([]byte, 16)
	rand.Read(b)
	return fmt.Sprintf("%x", b)[:24]
}

func NewBase64Token() string {
	u1 := NewUUID()
	u2 := NewUUID()
	raw := (u1 + u2)[:50]
	return base64.RawURLEncoding.EncodeToString([]byte(raw))
}

func NewHexToken(n int) string {
	b := make([]byte, (n+1)/2)
	rand.Read(b)
	return fmt.Sprintf("%x", b)[:n]
}

func UnixSec() int64 {
	return time.Now().Unix()
}

func UnixMs() int64 {
	return time.Now().UnixMilli()
}

func FloatVal(m map[string]interface{}, key string) float64 {
	v, _ := m[key].(float64)
	return v
}
