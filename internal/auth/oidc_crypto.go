package auth

import (
	"crypto"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rsa"
	"crypto/sha256"
	"crypto/sha512"
	"crypto/subtle"
	"encoding/asn1"
	"errors"
	"fmt"
	"hash"
	"math/big"
)

// verifyRSASignature verifies a PKCS#1 v1.5 RSA signature for JWT RS256/RS384/RS512.
func verifyRSASignature(pub *rsa.PublicKey, alg string, signingInput, sig []byte) error {
	h, cryptoHash, err := hashForAlg(alg)
	if err != nil {
		return err
	}
	h.Write(signingInput)
	digest := h.Sum(nil)
	if err = rsa.VerifyPKCS1v15(pub, cryptoHash, digest, sig); err != nil {
		return fmt.Errorf("rsa verify: %w", err)
	}
	return nil
}

// verifyECSignature verifies an ECDSA signature for JWT ES256/ES384/ES512.
func verifyECSignature(pub *ecdsa.PublicKey, alg string, signingInput, sig []byte) error {
	h, _, err := hashForAlg(alg)
	if err != nil {
		return err
	}
	h.Write(signingInput)
	digest := h.Sum(nil)

	// JWT ECDSA signature is R || S concatenated raw bytes.
	keySize := (pub.Curve.Params().BitSize + 7) / 8
	if len(sig) != 2*keySize {
		// Try DER-encoded as fallback.
		var asn1Sig struct{ R, S *big.Int }
		if _, aerr := asn1.Unmarshal(sig, &asn1Sig); aerr != nil {
			return errors.New("ec verify: invalid signature length")
		}
		if !ecdsa.Verify(pub, digest, asn1Sig.R, asn1Sig.S) {
			return errors.New("ec verify: signature mismatch")
		}
		return nil
	}
	r := new(big.Int).SetBytes(sig[:keySize])
	s := new(big.Int).SetBytes(sig[keySize:])
	if !ecdsa.Verify(pub, digest, r, s) {
		return errors.New("ec verify: signature mismatch")
	}
	_ = subtle.ConstantTimeCompare // import used elsewhere
	return nil
}

// hashForAlg returns the hash.Hash and crypto.Hash for a JWT algorithm.
func hashForAlg(alg string) (hash.Hash, crypto.Hash, error) {
	switch alg {
	case "RS256", "ES256":
		return sha256.New(), crypto.SHA256, nil
	case "RS384", "ES384":
		return sha512.New384(), crypto.SHA384, nil
	case "RS512", "ES512":
		return sha512.New(), crypto.SHA512, nil
	default:
		return nil, 0, fmt.Errorf("unsupported JWT algorithm %q", alg)
	}
}

// curveForCrv returns the elliptic.Curve for a JWK "crv" value.
func curveForCrv(crv string) (elliptic.Curve, error) {
	switch crv {
	case "P-256":
		return elliptic.P256(), nil
	case "P-384":
		return elliptic.P384(), nil
	case "P-521":
		return elliptic.P521(), nil
	default:
		return nil, fmt.Errorf("unsupported EC curve %q", crv)
	}
}
