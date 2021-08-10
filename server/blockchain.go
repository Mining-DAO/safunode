package server

import (
	"context"
	"log"
	"sync"
	"sync/atomic"

	"github.com/ethereum/go-ethereum"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/ethclient"
)

type BlockchainManager struct {
	client    *ethclient.Client
	headerCh  chan *types.Header
	headerSub ethereum.Subscription

	latestBlockNumber uint64
	blockSubs         []chan *types.Block
	blockSubsMux      sync.Mutex
}

func NewBlockchainManager(wsUrl string) *BlockchainManager {
	client, err := ethclient.Dial(wsUrl)
	if err != nil {
		log.Fatalf("ERROR: failed to connect to ws: %v", err)
	}

	latestBlockNumber, err := client.BlockNumber(context.Background())
	if err != nil {
		log.Fatalf("ERROR: failed to get latest block: %v", err)
	}

	headerCh := make(chan *types.Header)
	headerSub, err := client.SubscribeNewHead(context.Background(), headerCh)
	if err != nil {
		log.Fatalf("ERROR: failed to connect to ws: %v", err)
	}

	bm := &BlockchainManager{
		client:            client,
		headerCh:          headerCh,
		headerSub:         headerSub,
		latestBlockNumber: latestBlockNumber,
		blockSubs:         make([]chan *types.Block, 0),
	}
	go bm.loop()
	return bm
}

func (bm *BlockchainManager) GetLatestBlockNumber() uint64 {
	return atomic.LoadUint64(&bm.latestBlockNumber)
}

func (bm *BlockchainManager) SubscribeNewBlock(ch chan *types.Block) {
	bm.blockSubsMux.Lock()
	bm.blockSubs = append(bm.blockSubs, ch)
	bm.blockSubsMux.Unlock()
}

func (bm *BlockchainManager) loop() {
	for {
		select {
		case err := <-bm.headerSub.Err():
			log.Printf("ERROR: head subscription error: %v", err)
		case header := <-bm.headerCh:
			bm.processNewHeader(header)
		}
	}
}

func (bm *BlockchainManager) processNewHeader(header *types.Header) {
	log.Printf("New header at height %d", header.Number)
	atomic.StoreUint64(&bm.latestBlockNumber, header.Number.Uint64())
	block, err := bm.client.BlockByNumber(context.Background(), header.Number)
	if err != nil {
		log.Printf("ERROR: failed to get block by number: %v", err)
		return
	}
	bm.broadcastNewBlock(block)
}

func (bm *BlockchainManager) broadcastNewBlock(block *types.Block) {
	bm.blockSubsMux.Lock()
	for _, ch := range bm.blockSubs {
		select {
		case ch <- block:
		default:
		}
	}
	bm.blockSubsMux.Unlock()
}
