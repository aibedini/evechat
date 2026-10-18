package crypto

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"strconv"
	"time"
)

// SignHex returns HMAC-SHA256(secret, body) as lowercase hex.
func SignHex(secret string, body []byte) string {
	m := hmac.New(sha256.New, []byte(secret))
	m.Write(body)
	return hex.EncodeToString(m.Sum(nil))
}

// VerifyHex checks a signature in constant time.
func VerifyHex(secret string, body []byte, sig string) bool {
	if secret == "" || sig == "" {
		return false
	}
	expected := SignHex(secret, body)
	return hmac.Equal([]byte(expected), []byte(sig))
}

// WebhookSignature builds the value of X-EveChat-Signature for a body.
func WebhookSignature(secret string, ts int64, body []byte) string {
	return "sha256=" + SignHex(secret, []byte(strconv.FormatInt(ts, 10)+"."+string(body)))
}

// VerifyWebhookSignature validates "sha256=<hex>" over "<ts>.<body>".
func VerifyWebhookSignature(secret string, ts int64, body []byte, sig string) bool {
	if len(sig) < 8 || sig[:7] != "sha256=" {
		return false
	}
	return VerifyHex(secret, []byte(strconv.FormatInt(ts, 10)+"."+string(body)), sig[7:])
}

// SignEVEContext signs the timestamped visitor payload shared by EVE.
func SignEVEContext(secret string, payload []byte, ts int64) string {
	return SignHex(secret, []byte(fmt.Sprintf("%d.%s", ts, payload)))
}

// VerifyEVEContext validates the signed EVE context, including clock skew.
func VerifyEVEContext(secret string, payload []byte, ts int64, sig string, maxSkew time.Duration) bool {
	if secret == "" || sig == "" {
		return false
	}
	if maxSkew > 0 {
		drift := time.Since(time.Unix(ts, 0))
		if drift < 0 {
			drift = -drift
		}
		if drift > maxSkew {
			return false
		}
	}
	return VerifyHex(secret, []byte(fmt.Sprintf("%d.%s", ts, payload)), sig)
}
