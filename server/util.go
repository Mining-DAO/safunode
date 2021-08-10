package server

import (
	"bytes"
	"net/http"
	"strconv"
	"time"

	"github.com/ethereum/go-ethereum/common/hexutil"
	"github.com/ethereum/go-ethereum/crypto"
)

func GetIP(r *http.Request) string {
	forwarded := r.Header.Get("X-FORWARDED-FOR")
	if forwarded != "" {
		return forwarded
	}
	return r.RemoteAddr
}

func IsMetamask(r *http.Request) bool {
	return r.Header.Get("Origin") == "chrome-extension://nkbihfbeogaeaoehlefnkodbefgpgknn"
}

func MakeRequest(proxyUrl string, body []byte, isFlashbots bool) (*http.Response, error) {
	// Create new request:
	req, err := http.NewRequest("POST", proxyUrl, bytes.NewBuffer(body))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Accept", "application/json")
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Content-Length", strconv.Itoa(len(body)))

	if isFlashbots {
		addr := os.Getenv("RELAY_ADDRESS")
		pk, err := crypto.HexToECDSA(os.Getenv("RELAY_PRIVATE_KEY"))
		if err != nil {
			return nil, err
		}
		hashedBody := crypto.Keccak256Hash(body).Hex()
		sig, err := crypto.Sign(crypto.Keccak256([]byte("\x19Ethereum Signed Message:\n"+strconv.Itoa(len(hashedBody))+hashedBody)), pk)
		if err != nil {
			return nil, err
		}
		signature := addr + ":" + hexutil.Encode(sig)
		req.Header.Set("X-Flashbots-Signature", signature)
	}

	client := &http.Client{
		Timeout: time.Duration(10 * time.Second),
	}
	return client.Do(req)
}
