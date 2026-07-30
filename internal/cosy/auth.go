package cosy

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
)

// ExchangeJobToken 将 PAT 或 refresh token 转换为 job token
func ExchangeJobToken(token, machineId, machineToken, machineType string, jobTokenURL ...string) (map[string]interface{}, error) {
	date := CurrentDate()
	sig := SignLegacy(date)

	var personalToken, refreshToken string
	if strings.HasPrefix(token, "drt-") {
		refreshToken = token
		personalToken = ""
	} else {
		personalToken = token
		refreshToken = ""
	}

	inner := map[string]interface{}{
		"personalToken":      personalToken,
		"securityOauthToken": "",
		"refreshToken":       refreshToken,
		"needRefresh":        refreshToken != "",
		"authInfo":           map[string]interface{}{},
	}
	innerJSON, _ := json.Marshal(inner)

	outer := map[string]interface{}{
		"payload":       string(innerJSON),
		"encodeVersion": "1",
	}
	plain, _ := json.Marshal(outer)
	body, err := Encode(plain)
	if err != nil {
		return nil, err
	}

	endpoint := "https://center.qoder.sh/algo/api/v3/user/jobToken?Encode=1"
	if len(jobTokenURL) > 0 && jobTokenURL[0] != "" {
		endpoint = jobTokenURL[0]
	}

	req, err := http.NewRequest("POST", endpoint, strings.NewReader(body))
	if err != nil {
		return nil, err
	}
	req.Header.Set("cosy-machinetoken", machineToken)
	req.Header.Set("cosy-machinetype", machineType)
	req.Header.Set("login-version", "v2")
	req.Header.Set("appcode", AppCode)
	req.Header.Set("accept", "application/json")
	req.Header.Set("accept-encoding", "identity")
	req.Header.Set("cosy-version", Version)
	req.Header.Set("cosy-clienttype", "5")
	req.Header.Set("date", date)
	req.Header.Set("signature", sig)
	req.Header.Set("content-type", "application/json")
	req.Header.Set("cosy-machineid", machineId)
	req.Header.Set("user-agent", "Go-http-client/2.0")

	client := &http.Client{Timeout: 15 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	data, _ := io.ReadAll(resp.Body)
	if resp.StatusCode != 200 {
		return nil, fmt.Errorf("HTTP %d at jobToken body=%s", resp.StatusCode, string(data))
	}
	var result map[string]interface{}
	err = json.Unmarshal(data, &result)
	return result, err
}
