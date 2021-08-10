package server

import (
	"encoding/json"
	"io/ioutil"
	"log"
	"net/http"
	"strings"
)

var whitelistedIps = []string{"127.0.0.1"}

type SafuNodeServer struct {
	ListenAddress string
	ProxyUrl      string
	TxRelayer     *PrivateTxRelayer
}

func NewSafuNodeServer(listenAddress string, proxyUrl string, txRelayer *PrivateTxRelayer) *SafuNodeServer {
	return &SafuNodeServer{
		ListenAddress: listenAddress,
		ProxyUrl:      proxyUrl,
		TxRelayer:     txRelayer,
	}
}

func (s *SafuNodeServer) Start() {
	log.Printf("Starting SafuNode endpoint at %v...", s.ListenAddress)
	if err := http.ListenAndServe(s.ListenAddress, http.HandlerFunc(s.handleHttpRequest)); err != nil {
		log.Fatalf("Failed to start SafuNode endpoint: %v", err)
	}
}

func (s *SafuNodeServer) handleHttpRequest(respw http.ResponseWriter, req *http.Request) {
	// For now restrict to certain IPs:
	ip := GetIP(req)
	if !IsWhitelisted(ip) {
		log.Printf("Blocked: IP=%s", ip)
		respw.WriteHeader(http.StatusUnauthorized)
		return
	}
	// if !IsMetamask(req) {
	// 	log.Printf("Blocked non-Metamask request")
	//  respw.WriteHeader(http.StatusUnauthorized)
	// 	return
	// }

	// Read request body:
	body, err := ioutil.ReadAll(req.Body)
	if err != nil {
		log.Printf("ERROR: failed to read request body: %v", err)
		respw.WriteHeader(http.StatusBadRequest)
		return
	}
	defer req.Body.Close()
	log.Printf("[debug] Received: IP=%s", ip)
	// log.Printf("[debug] Received: IP=%s Header=%v", ip, req.Header)
	// log.Printf("[debug] Received: IP=%s Body=%s Header=%v", ip, string(body), req.Header)

	// Parse JSON RPC:
	var jsonReq *JsonRpcRequest
	if err := json.Unmarshal(body, &jsonReq); err != nil {
		log.Printf("ERROR: failed to parse JSON RPC request: %v", err)
		respw.WriteHeader(http.StatusBadRequest)
		return
	}

	// Non-eth_sendRawTransaction txs go through ProxyUrl:
	if jsonReq.Method != "eth_sendRawTransaction" {
		proxyResp, err := MakeRequest(s.ProxyUrl, body, false)
		if err != nil {
			log.Printf("ERROR: failed to make proxy request: %v", err)
			respw.WriteHeader(http.StatusBadRequest)
			return
		}
		proxyRespBody, err := ioutil.ReadAll(proxyResp.Body)
		if err != nil {
			log.Printf("ERROR: failed to read proxy response: %v", err)
			respw.WriteHeader(http.StatusBadRequest)
			return
		}
		defer proxyResp.Body.Close()

		respw.WriteHeader(proxyResp.StatusCode)
		respw.Write(proxyRespBody)
		// log.Printf("Successfully proxied %s. Result: %v", jsonReq.Method, string(proxyRespBody))
		log.Printf("Successfully proxied %s", jsonReq.Method)
		return
	}

	// eth_sendRawTransaction txs go through TxRelayer:
	jsonResp, err := s.TxRelayer.SendRawTransaction(jsonReq)
	if err != nil {
		log.Printf("ERROR: failed to relay tx: %v", err)
		respw.WriteHeader(http.StatusBadRequest)
		return
	}
	if err := json.NewEncoder(respw).Encode(jsonResp); err != nil {
		log.Printf("ERROR: failed to encode JSON RPC: %v", err)
		respw.WriteHeader(http.StatusBadRequest)
	}
	log.Printf("Successfully relayed %s", jsonReq.Method)
	// log.Printf("Successfully relayed %s. Result: %+v", jsonReq.Method, jsonResp)
}

func IsWhitelisted(ip string) bool {
	for i := range whitelistedIps {
		if strings.HasPrefix(ip, whitelistedIps[i]) {
			return true
		}
	}
	return false
}
