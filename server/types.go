package server

type JsonRpcRequest struct {
	Id      interface{}   `json:"id"`
	Method  string        `json:"method"`
	Params  []interface{} `json:"params"`
	Version string        `json:"jsonrpc,omitempty"`
}

type JsonRpcResponse struct {
	Id      interface{} `json:"id"`
	Result  interface{} `json:"result,omitempty"`
	Error   interface{} `json:"error,omitempty"`
	Message interface{} `json:"message,omitempty"`
	Version string      `json:"jsonrpc,omitempty"`
}

type SendBundleArgs struct {
	Txs               []string `json:"txs"`
	BlockNumber       string   `json:"blockNumber"`
	MinTimestamp      string   `json:"minTimestamp,omitempty"`
	MaxTimestamp      string   `json:"maxTimestamp,omitempty"`
	RevertingTxHashes []string `json:"revertingTxHashes,omitempty"`
}
