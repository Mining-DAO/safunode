package server

import (
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io/ioutil"
	"log"
	"strconv"
	"sync"
	"sync/atomic"

	"github.com/ethereum/go-ethereum/common/hexutil"
	"github.com/ethereum/go-ethereum/core/types"
)

type PendingTransaction struct {
	Tx          *types.Transaction
	BlockNumber uint64
}

type PrivateTxRelayer struct {
	Url                 string
	id                  uint64
	blockchainManager   *BlockchainManager
	blockCh             chan *types.Block
	txByFromAndNonce    map[string]map[uint64]*PendingTransaction
	txByFromAndNonceMux sync.RWMutex
}

func NewPrivateTxRelayer(url string, bm *BlockchainManager) *PrivateTxRelayer {
	relayer := &PrivateTxRelayer{
		Url:               url,
		id:                uint64(1e9),
		blockchainManager: bm,
		blockCh:           make(chan *types.Block, 10),
		txByFromAndNonce:  make(map[string]map[uint64]*PendingTransaction),
	}
	go relayer.txMonitorLoop()
	relayer.blockchainManager.SubscribeNewBlock(relayer.blockCh)
	return relayer
}

func (r *PrivateTxRelayer) SendRawTransaction(rawJsonReq *JsonRpcRequest) (*JsonRpcResponse, error) {
	// Validate JSON RPC parameters:
	if len(rawJsonReq.Params) == 0 {
		return nil, errors.New("invalid params")
	}
	rawTxHex, ok := rawJsonReq.Params[0].(string)
	if !ok || len(rawTxHex) < 2 {
		return nil, errors.New("invalid raw transaction")
	}
	rawTxBytes, err := hex.DecodeString(rawTxHex[2:])
	if err != nil {
		return nil, errors.New("invalid raw transaction")
	}
	tx := new(types.Transaction)
	if err := tx.UnmarshalBinary(rawTxBytes); err != nil {
		return nil, err
	}

	// Send bundle:
	blockNumber := r.blockchainManager.GetLatestBlockNumber() + 1
	if err := r.SendBundle([]string{rawTxHex}, blockNumber); err != nil {
		return nil, err
	}

	// Save transaction:
	if err := r.addPendingTransaction(tx, blockNumber); err != nil {
		return nil, err
	}

	// eth_sendRawTransaction response:
	jsonResp := &JsonRpcResponse{
		Id:      rawJsonReq.Id,
		Result:  tx.Hash().Hex(),
		Version: "2.0",
	}
	return jsonResp, nil
}

func (r *PrivateTxRelayer) SendBundle(rawTxs []string, blockNumber uint64) error {
	// Convert eth_sendRawTransaction-style into eth_sendBundle:
	sendBundleArgs := SendBundleArgs{
		Txs:         rawTxs,
		BlockNumber: "0x" + strconv.FormatUint(blockNumber, 16),
	}
	bundleJsonReq := &JsonRpcRequest{
		Id:      atomic.AddUint64(&r.id, 1),
		Method:  "eth_sendBundle",
		Params:  []interface{}{sendBundleArgs},
		Version: "2.0",
	}
	data, err := json.Marshal(bundleJsonReq)
	if err != nil {
		return err
	}

	// Make eth_sendBundle request:
	log.Printf("[DEBUG] eth_sendBundle request: %s", string(data))
	resp, err := MakeRequest(r.Url, data, true)
	if err != nil {
		return err
	}

	// Read response body:
	respBody, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	log.Printf("[DEBUG] eth_sendBundle response: %s", string(respBody))

	// Parse response:
	var bundleJsonResp *JsonRpcResponse
	if err := json.Unmarshal(respBody, &bundleJsonResp); err != nil {
		return err
	}
	if bundleJsonResp.Error != nil {
		return fmt.Errorf("eth_sendBundle returned error: +v", bundleJsonResp.Error)
	}
	return nil
}

func (r *PrivateTxRelayer) addPendingTransaction(tx *types.Transaction, blockNumber uint64) error {
	log.Printf("Adding pending tx with hash %s at block %d", tx.Hash().Hex(), blockNumber)
	from, err := From(tx)
	if err != nil {
		return err
	}
	r.txByFromAndNonceMux.Lock()
	if r.txByFromAndNonce[from] == nil {
		r.txByFromAndNonce[from] = make(map[uint64]*PendingTransaction)
	}
	r.txByFromAndNonce[from][tx.Nonce()] = &PendingTransaction{tx, blockNumber}
	r.txByFromAndNonceMux.Unlock()
	return nil
}

func (r *PrivateTxRelayer) removePendingTransaction(tx *types.Transaction) error {
	from, err := From(tx)
	if err != nil {
		return err
	}
	//r.txByFromAndNonceMux.Lock()
	if r.txByFromAndNonce[from] == nil {
		return nil
	}
	log.Printf("Removing pending tx with hash %s", tx.Hash().Hex())
	delete(r.txByFromAndNonce[from], tx.Nonce())
	//r.txByFromAndNonceMux.Unlock()
	return nil
}

func (r *PrivateTxRelayer) txMonitorLoop() {
	for {
		select {
		case block := <-r.blockCh:
			log.Printf("Relayer got new block header: %v", block.NumberU64())
			// 1. Remove pending transactions with outdated <from, nonce>
			newTxs := block.Transactions()
			for i := range newTxs {
				if err := r.removePendingTransaction(newTxs[i]); err != nil {
					log.Printf("ERROR: failed to remove transaction %+v: %v", newTxs[i], err)
					continue
				}
			}
			// 2. Re-send transactions
			blockNumber := block.NumberU64()
			r.txByFromAndNonceMux.Lock()
			for from, txInfoByNonce := range r.txByFromAndNonce {
				for nonce, txInfo := range txInfoByNonce {
					if txInfo.BlockNumber > blockNumber {
						continue
					}
					txInfo.BlockNumber = blockNumber + 1

					tx := txInfo.Tx
					rawTxBytes, err := tx.MarshalBinary()
					if err != nil {
						log.Printf("ERROR: failed to serialize tx: %v", err)
						continue
					}
					rawTxHex := hexutil.Encode(rawTxBytes)

					//go func() {
					log.Printf("Re-sending: from=%v nonce=%v hash=%s", from, nonce, tx.Hash().Hex())
					if err := r.SendBundle([]string{rawTxHex}, txInfo.BlockNumber); err != nil {
						log.Printf("ERROR: failed to resend eth_sendBundle: %v", err)
					}
					//}()
				}
			}
			r.txByFromAndNonceMux.Unlock()
		}
	}
}

func From(tx *types.Transaction) (string, error) {
	signer := types.LatestSignerForChainID(tx.ChainId())
	sender, err := types.Sender(signer, tx)
	if err != nil {
		return "", err
	}
	return sender.Hex(), nil
}
