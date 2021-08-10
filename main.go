package main

import (
	"flag"

	"safunode/server"
)

// ChainID: 1, Infura:
// ./bin/safunode --listen 0.0.0.0:9000 --proxy https://mainnet.infura.io/v3/dce4f913d749454d94daa2c87f01ceb2 --relayer http://0.0.0.0:9545 --subscribe ws://127.0.0.1:8546

// ChainID: 1, LocalGeth:
// ./bin/safunode --listen 0.0.0.0:9000 --proxy http://127.0.0.1:8545 --relayer http://0.0.0.0:9545 --subscribe ws://127.0.0.1:8546

// ChainID: 1337, LocalMevGeth (--dev):
// ./bin/safunode --listen 0.0.0.0:9999 --proxy http://0.0.0.0:9545 --relayer http://0.0.0.0:9545 --subscribe ws://0.0.0.0:8546

const (
	defaultListenAddress  = "0.0.0.0:9000"
	// MiningDAO Infura endpoint for public examples, please don't abuse
	defaultProxyUrl       = "https://mainnet.infura.io/v3/dce4f913d749454d94daa2c87f01ceb2"
	defaultRelayerUrl     = "https://relay.flashbots.net"
	defaultSubscribeWsUrl = "ws://127.0.0.1:8546"
)

var listenAddress = flag.String("listen", defaultListenAddress, "Listen address")
var proxyUrl = flag.String("proxy", defaultProxyUrl, "URL for proxy eth_call-like request")
var relayerUrl = flag.String("relayer", defaultRelayerUrl, "URL for eth_sendRawTransaction relay")
var subscribeWsUrl = flag.String("subscribe", defaultSubscribeWsUrl, "URL for blockchain subscriptions (must be WebSocket)")

func main() {
	flag.Parse()
	bm := server.NewBlockchainManager(*subscribeWsUrl)
	relayer := server.NewPrivateTxRelayer(*relayerUrl, bm)
	s := server.NewSafuNodeServer(*listenAddress, *proxyUrl, relayer)
	s.Start()
}
