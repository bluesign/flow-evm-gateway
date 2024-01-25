package api_test

import (
	"bytes"
	"context"
	_ "embed"
	"io"
	"net/http"
	"strings"
	"testing"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/rpc"
	"github.com/gorilla/websocket"
	"github.com/onflow/flow-evm-gateway/api"
	"github.com/onflow/flow-evm-gateway/storage"
	"github.com/rs/zerolog"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

//go:embed fixtures/eth_json_rpc_requests.json
var requests string

//go:embed fixtures/eth_json_rpc_responses.json
var responses string

func TestServerJSONRPCOveHTTPHandler(t *testing.T) {
	store := storage.NewStore()
	srv := api.NewHTTPServer(zerolog.Logger{}, rpc.DefaultHTTPTimeouts)
	config := &api.Config{
		ChainID:  api.FlowEVMTestnetChainID,
		Coinbase: common.HexToAddress("0xf02c1c8e6114b1dbe8937a39260b5b0a374432bb"),
	}
	supportedAPIs := api.SupportedAPIs(config, store)
	srv.EnableRPC(supportedAPIs)
	srv.SetListenAddr("localhost", 8545)
	err := srv.Start()
	defer srv.Stop()
	if err != nil {
		panic(err)
	}

	url := "http://" + srv.ListenAddr() + "/rpc"

	expectedResponses := strings.Split(responses, "\n")
	for i, request := range strings.Split(requests, "\n") {
		resp := rpcRequest(url, request, "origin", "test.com")
		defer resp.Body.Close()
		content, err := io.ReadAll(resp.Body)
		if err != nil {
			panic(err)
		}
		expectedResponse := expectedResponses[i]

		assert.Equal(t, expectedResponse, strings.TrimSuffix(string(content), "\n"))
	}

	t.Run("eth_getBlockByNumber", func(t *testing.T) {
		request := `{"jsonrpc":"2.0","id":1,"method":"eth_getBlockByNumber","params":["0x1",false]}`
		expectedResponse := `{"jsonrpc":"2.0","id":1,"result":{"difficulty":"0x4ea3f27bc","extraData":"0x476574682f4c5649562f76312e302e302f6c696e75782f676f312e342e32","gasLimit":"0x1388","gasUsed":"0x0","hash":"0xf31ee13dad8f38431fd31278b12be62e6b77e6923f0b7a446eb1affb61f21fc9","logsBloom":"0x00000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000","miner":"0xbb7b8287f3f0a933474a79eae42cbca977791171","mixHash":"0x4fffe9ae21f1c9e15207b1f472d5bbdd68c9595d461666602f2be20daf5e7843","nonce":"0x689056015818adbe","number":"0x1","parentHash":"0xe81f171bcc55a6ff8345e692c0f86e5b48e01b996cadc001622fb5e363b421c0","receiptsRoot":"0x0000000000000000000000000000000000000000000000000000000000000000","sha3Uncles":"0x1dcc4de8dec75d7aab85b567b6ccd41ad312451b948a7413f0a142fd40d49347","size":"0x220","stateRoot":"0xddc8b0234c2e0cad087c8b389aa7ef01f7d79b2570bccb77ce48648aa61c904d","timestamp":"0x55ba467c","totalDifficulty":"0x78ed983323d","transactions":["0xf31ee13dad8f38431fd31278b12be62e6b77e6923f0b7a446eb1affb61f21fc9"],"transactionsRoot":"0x56e81f171bcc55a6ff8345e692c0f86e5b48e01b996cadc001622fb5e363b421","uncles":[]}}`

		event := blockExecutedEvent(
			1,
			"0xf31ee13dad8f38431fd31278b12be62e6b77e6923f0b7a446eb1affb61f21fc9",
			7766279631452241920,
			"0xe81f171bcc55a6ff8345e692c0f86e5b48e01b996cadc001622fb5e363b421c0",
			"0x0000000000000000000000000000000000000000000000000000000000000000",
			[]string{"0xf31ee13dad8f38431fd31278b12be62e6b77e6923f0b7a446eb1affb61f21fc9"},
		)
		err := store.StoreBlock(context.Background(), event)
		require.NoError(t, err)

		resp := rpcRequest(url, request, "origin", "test.com")
		defer resp.Body.Close()

		content, err := io.ReadAll(resp.Body)
		if err != nil {
			panic(err)
		}

		assert.Equal(t, expectedResponse, strings.TrimSuffix(string(content), "\n"))
	})
}

func TestServerJSONRPCOveWebSocketHandler(t *testing.T) {
	store := storage.NewStore()
	srv := api.NewHTTPServer(zerolog.Logger{}, rpc.DefaultHTTPTimeouts)
	config := &api.Config{
		ChainID:  api.FlowEVMTestnetChainID,
		Coinbase: common.HexToAddress("0xf02c1c8e6114b1dbe8937a39260b5b0a374432bb"),
	}
	supportedAPIs := api.SupportedAPIs(config, store)
	srv.EnableWS(supportedAPIs)
	srv.SetListenAddr("localhost", 8545)
	err := srv.Start()
	defer srv.Stop()
	if err != nil {
		panic(err)
	}

	url := "ws://" + srv.ListenAddr() + "/ws"

	extraHeaders := []string{"Origin", "*"}
	headers := make(http.Header)
	for i := 0; i < len(extraHeaders); i += 2 {
		key, value := extraHeaders[i], extraHeaders[i+1]
		headers.Set(key, value)
	}
	conn, _, err := websocket.DefaultDialer.Dial(url, headers)
	if err != nil {
		conn.Close()
		panic(err)
	}
	defer conn.Close()

	done := make(chan struct{})

	expectedResponses := strings.Split(responses, "\n")
	go func() {
		defer close(done)
		for {
			_, message, err := conn.ReadMessage()
			if err != nil {
				return
			}
			assert.Contains(
				t,
				expectedResponses,
				strings.TrimSuffix(string(message), "\n"),
			)
		}
	}()

	for _, request := range strings.Split(requests, "\n") {
		err := conn.WriteMessage(websocket.TextMessage, []byte(request))
		assert.NoError(t, err)
	}
}

// rpcRequest performs a JSON-RPC request to the given URL.
func rpcRequest(url, bodyStr string, extraHeaders ...string) *http.Response {
	// Create the request.
	body := bytes.NewReader([]byte(bodyStr))
	req, err := http.NewRequest(http.MethodPost, url, body)
	if err != nil {
		panic(err)
	}
	req.Header.Set("content-type", "application/json")
	req.Header.Set("accept-encoding", "identity")

	// Apply extra headers.
	if len(extraHeaders)%2 != 0 {
		panic("odd extraHeaders length")
	}
	for i := 0; i < len(extraHeaders); i += 2 {
		key, value := extraHeaders[i], extraHeaders[i+1]
		if strings.EqualFold(key, "host") {
			req.Host = value
		} else {
			req.Header.Set(key, value)
		}
	}

	// Perform the request.
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		panic(err)
	}
	return resp
}
