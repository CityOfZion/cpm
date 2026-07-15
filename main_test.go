package main

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"

	"github.com/nspcc-dev/neo-go/pkg/util"
	log "github.com/sirupsen/logrus"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func Test_DownloadContract(t *testing.T) {
	log.SetLevel(log.WarnLevel)
	t.Cleanup(func() { log.SetOutput(os.Stderr) })

	c := util.Uint160{}
	h := []string{"127.0.0.1:10333"}

	t.Run("invalid contract hash should fail", func(t *testing.T) {
		err := downloadContract(nil, "invalidhash", nil, false, true)
		require.Error(t, err)

		expected := "failed to convert script hash"
		assert.Contains(t, err.Error(), expected)
	})

	t.Run("should download and save contract to config", func(t *testing.T) {
		err := downloadContract(h, c.StringLE(), NewOkDownloader(), true, true)
		require.NoError(t, err)

		// test if contract is added
		found := false
		for _, contract := range cfg.Contracts {
			if contract.Label == "unknown" && contract.ScriptHash.Equals(c) {
				found = true
			}
		}
		assert.True(t, found, "failed to save contract to cfg")
	})

	t.Run("first host fails second host succeeds", func(t *testing.T) {
		logs := NewMockLogs(t)
		failHost := "127.0.0.1:10333"
		successHost := "127.0.0.2:20333"
		hosts := []string{failHost, successHost}

		downloader := NewMockDownloader([]bool{false, true})

		err := downloadContract(hosts, c.StringLE(), &downloader, false, true)
		require.NoErrorf(t, err, "expected download to succeed for %s", successHost)

		if assert.Greater(t, logs.Len(), 1) {
			assert.Contains(t, logs.lines[0], downloader.responseMsg[0])
			assert.Contains(t, logs.lines[1], downloader.responseMsg[1])
		}
	})

	t.Run("should fail to download", func(t *testing.T) {
		logs := NewMockLogs(t)
		downloader := NewMockDownloader([]bool{false})

		err := downloadContract(h, c.StringLE(), &downloader, false, true)
		require.Error(t, err)
		if assert.Equal(t, logs.Len(), 1) {
			assert.Contains(t, logs.lines[0], downloader.responseMsg[0])
		}
	})
}

func Test_DownloadManifest(t *testing.T) {
	log.SetLevel(log.WarnLevel)
	t.Cleanup(func() { log.SetOutput(os.Stderr) })

	c := util.Uint160{}

	t.Run("invalid contract hash should fail", func(t *testing.T) {
		err := downloadManifest(nil, "invalidhash", false, true)
		require.Error(t, err)

		expected := "failed to convert script hash"
		assert.Contains(t, err.Error(), expected)
	})

	t.Run("first host fails second host succeeds", func(t *testing.T) {
		logs := NewMockLogs(t)
		contractStateResponse := RpcResponse{
			"getcontractstate",
			`{"jsonrpc":"2.0","id":0,"result":{"id":1,"updatecounter":0,"hash":"0x16ce77fbb91be1d4aa1e2b58f1141d747bdf2666","nef":{"magic":860243278,"compiler":"neo3-boa by COZ-1.0.0","source":"","tokens":[{"hash":"0xfffdc93764dbaddd97c48f252a53ea4643faa3fd","method":"update","paramcount":3,"hasreturnvalue":false,"callflags":"All"}],"script":"DAVGSVJTVEBXAAN6eXg3AABA","checksum":3884080072},"manifest":{"name":"01-simple","groups":[],"features":{},"supportedstandards":[],"abi":{"methods":[{"name":"main","parameters":[],"returntype":"String","offset":0,"safe":false},{"name":"update","parameters":[{"name":"script","type":"ByteArray"},{"name":"manifest","type":"ByteArray"},{"name":"data","type":"Any"}],"returntype":"Void","offset":8,"safe":false}],"events":[]},"permissions":[{"contract":"0xfffdc93764dbaddd97c48f252a53ea4643faa3fd","methods":["update"]}],"trusts":[],"extra":null}}}`,
		}
		srv := NewTestRpcServer(t, []RpcResponse{contractStateResponse})
		defer srv.Close()

		failHost := "http://127.0.0.1:10333"
		successHost := fmt.Sprintf("http://%s", srv.Listener.Addr().String())
		hosts := []string{failHost, successHost}

		err := downloadManifest(hosts, c.StringLE(), false, true)
		require.NoError(t, err)

		if assert.GreaterOrEqual(t, logs.Len(), 3) {
			assert.Contains(t, logs.lines[0], fmt.Sprintf("Attempting to fetch manifest for contract '%s' using %s", c, failHost))
			assert.Contains(t, logs.lines[1], "failed to reach RPC host:")
			assert.Contains(t, logs.lines[1], fmt.Sprintf("Post \\\"%s\\\"", failHost))
			assert.Contains(t, logs.lines[2], fmt.Sprintf("Attempting to fetch manifest for contract '%s'", c))
		}
		require.FileExists(t, "contract.manifest.json")
		_ = os.Remove("contract.manifest.json")
	})

	t.Run("request contract not found", func(t *testing.T) {
		logs := NewMockLogs(t)
		contractStateResponse := RpcResponse{
			"getcontractstate",
			`{"jsonrpc":"2.0","id":0,"error":{"code":-100,"message":"Unknown contract"}}`,
		}
		srv := NewTestRpcServer(t, []RpcResponse{contractStateResponse})
		defer srv.Close()

		successHost := fmt.Sprintf("http://%s", srv.Listener.Addr().String())
		err := downloadManifest([]string{successHost}, c.StringLE(), false, true)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "failed to fetch manifest")

		if assert.GreaterOrEqual(t, logs.Len(), 2) {
			assert.Contains(t, logs.lines[1], "getcontractstate failed with: Unknown contract")
		}
	})
}

//////////////////////////////////////////////
//
// Everything below this point is helper logic
//
//////////////////////////////////////////////

type MockDownloader struct {
	responses   []bool
	ctr         int
	responseMsg []string
}

func (md *MockDownloader) downloadContract(scriptHash util.Uint160, host string) (string, error) {
	if md.ctr > len(md.responses) {
		return "", fmt.Errorf("insufficient responses")
	}
	r := md.responses[md.ctr]
	md.ctr++
	if r {
		s := fmt.Sprintf("download success, c = %s, h = %s", scriptHash.StringLE(), host)
		md.responseMsg = append(md.responseMsg, s)
		return s, nil
	} else {
		s := fmt.Sprintf("download failed, c = %s, h = %s", scriptHash.StringLE(), host)
		md.responseMsg = append(md.responseMsg, s)
		return s, errors.New("download failed")
	}
}

func NewMockDownloader(responses []bool) MockDownloader {
	return MockDownloader{responses: responses}
}

func NewOkDownloader() Downloader {
	return &MockDownloader{responses: []bool{true}}
}

type LogTester struct {
	lines []string
}

func (lt *LogTester) Write(p []byte) (n int, err error) {
	line := string(p)
	lt.lines = append(lt.lines, strings.TrimSuffix(line, "\n"))
	return len(p), nil
}

func (lt *LogTester) Len() int {
	return len(lt.lines)
}

func NewMockLogs(t *testing.T) *LogTester {
	var l LogTester
	log.SetOutput(&l)
	oldLvl := log.GetLevel()
	log.SetLevel(log.DebugLevel)
	t.Cleanup(func() {
		log.SetOutput(os.Stderr)
		log.SetLevel(oldLvl)
	})
	return &l
}

type JsonRPC struct {
	Id      int           `json:"id"`
	Method  string        `json:"method"`
	Params  []interface{} `json:"params"`
	Version string        `json:"jsonrpc"`
}

type RpcResponse struct {
	Method         string // RPC method that's called. This is just for readability, it's not actually used
	ServerResponse string // The RPC server response to the method
}

// MockRpcServer is a basic JSON-RPC server that for reach call returns the next response from an internal list
// of responses
type MockRpcServer struct {
	counter   int
	responses []RpcResponse
}

func NewMockRpcServer() *MockRpcServer {
	return &MockRpcServer{}
}

func (mrs *MockRpcServer) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	body, err := io.ReadAll(r.Body)
	if err != nil {
		http.Error(w, "can't read body", http.StatusBadRequest)
		return
	}
	req := JsonRPC{}
	err = json.Unmarshal(body, &req)
	if err != nil {
		http.Error(w, fmt.Sprintf("failed to decode JSON-RPC request: %v", err), http.StatusBadRequest)
		return
	}
	if mrs.counter >= len(mrs.responses) {
		http.Error(w, fmt.Sprintf("requested '%s', not enough responses in mock server", req.Method), http.StatusBadRequest)
		return
	}
	_, err = w.Write([]byte(mrs.responses[mrs.counter].ServerResponse))
	if err != nil {
		http.Error(w, fmt.Sprintf("failed to write response: %v", err), http.StatusBadRequest)
		return
	}
	mrs.counter++
}

// Caller must call Close() when done
func NewTestRpcServer(t *testing.T, responses []RpcResponse) *httptest.Server {
	m := NewMockRpcServer()
	m.responses = append(m.responses, responses...)
	return httptest.NewServer(m)
}
