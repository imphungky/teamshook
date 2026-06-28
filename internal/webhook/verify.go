package webhook

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
)

// VerifySignature reports whether signatureHeader is a valid HMAC-SHA256
// signature of body under secret.
//
//   - body            is the RAW request body bytes (exactly what GitHub hashed;
//                      this is why we verify before unmarshalling — re-serialized
//                      JSON would produce different bytes and never match).
//   - secret          is the shared webhook secret (same string you typed into
//                      GitHub's webhook config).
//   - signatureHeader is the raw value of the X-Hub-Signature-256 header,
//                      including the "sha256=" prefix GitHub sends.
//
// It returns false (fails closed) on an empty secret: a missing/misconfigured
// secret must reject every request, never accept them.
func VerifySignature(body, secret []byte, signatureHeader string) bool {
	// Fail closed: no secret configured means we cannot authenticate anything.
	if len(secret) == 0 {
		return false
	}

	// Recompute the MAC over the exact raw body bytes, keyed with the secret.
	mac := hmac.New(sha256.New, secret)
	mac.Write(body)
	sum := mac.Sum(nil)

	// GitHub sends the signature hex-encoded and prefixed: "sha256=<hexdigest>".
	// Build the same shape from our own digest so we compare like-for-like.
	expectedHeader := "sha256=" + hex.EncodeToString(sum)

	// Constant-time compare — NOT ==. The time taken must not reveal how many
	// leading bytes matched, or an attacker could brute-force the signature.
	return hmac.Equal([]byte(expectedHeader), []byte(signatureHeader))
}
