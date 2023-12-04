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
			assert.Contains(t, logs.lines[0], "RPCClient init failed with:")
			assert.Contains(t, logs.lines[0], fmt.Sprintf("Post \\\"%s\\\"", failHost))
			assert.Contains(t, logs.lines[1], fmt.Sprintf("Attempting to fetch manifest for contract '%s'", c))
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
	// neo-go RPC clients call the follow methods during initialisation
	return &MockRpcServer{
		responses: []RpcResponse{
			RpcResponse{
				"getversion",
				"{\"jsonrpc\":\"2.0\",\"id\":1,\"result\":{\"tcpport\":10333,\"wsport\":10334,\"nonce\":1460779089,\"useragent\":\"/Neo:3.6.0/\",\"protocol\":{\"addressversion\":53,\"network\":860833102,\"validatorscount\":7,\"msperblock\":15000,\"maxtraceableblocks\":2102400,\"maxvaliduntilblockincrement\":5760,\"maxtransactionsperblock\":512,\"memorypoolmaxtransactions\":50000,\"initialgasdistribution\":5200000000000000}}}",
			},
			{
				"getnativecontracts",
				"{\"jsonrpc\":\"2.0\",\"id\":1,\"result\":[{\"id\":-1,\"hash\":\"0xfffdc93764dbaddd97c48f252a53ea4643faa3fd\",\"nef\":{\"magic\":860243278,\"compiler\":\"neo-core-v3.0\",\"source\":\"\",\"tokens\":[],\"script\":\"EEEa93tnQBBBGvd7Z0AQQRr3e2dAEEEa93tnQBBBGvd7Z0AQQRr3e2dAEEEa93tnQBBBGvd7Z0AQQRr3e2dAEEEa93tnQBBBGvd7Z0A=\",\"checksum\":1094259016},\"manifest\":{\"name\":\"ContractManagement\",\"groups\":[],\"features\":{},\"supportedstandards\":[],\"abi\":{\"methods\":[{\"name\":\"deploy\",\"parameters\":[{\"name\":\"nefFile\",\"type\":\"ByteArray\"},{\"name\":\"manifest\",\"type\":\"ByteArray\"}],\"returntype\":\"Array\",\"offset\":0,\"safe\":false},{\"name\":\"deploy\",\"parameters\":[{\"name\":\"nefFile\",\"type\":\"ByteArray\"},{\"name\":\"manifest\",\"type\":\"ByteArray\"},{\"name\":\"data\",\"type\":\"Any\"}],\"returntype\":\"Array\",\"offset\":7,\"safe\":false},{\"name\":\"destroy\",\"parameters\":[],\"returntype\":\"Void\",\"offset\":14,\"safe\":false},{\"name\":\"getContract\",\"parameters\":[{\"name\":\"hash\",\"type\":\"Hash160\"}],\"returntype\":\"Array\",\"offset\":21,\"safe\":true},{\"name\":\"getContractById\",\"parameters\":[{\"name\":\"id\",\"type\":\"Integer\"}],\"returntype\":\"Array\",\"offset\":28,\"safe\":true},{\"name\":\"getContractHashes\",\"parameters\":[],\"returntype\":\"InteropInterface\",\"offset\":35,\"safe\":true},{\"name\":\"getMinimumDeploymentFee\",\"parameters\":[],\"returntype\":\"Integer\",\"offset\":42,\"safe\":true},{\"name\":\"hasMethod\",\"parameters\":[{\"name\":\"hash\",\"type\":\"Hash160\"},{\"name\":\"method\",\"type\":\"String\"},{\"name\":\"pcount\",\"type\":\"Integer\"}],\"returntype\":\"Boolean\",\"offset\":49,\"safe\":true},{\"name\":\"setMinimumDeploymentFee\",\"parameters\":[{\"name\":\"value\",\"type\":\"Integer\"}],\"returntype\":\"Void\",\"offset\":56,\"safe\":false},{\"name\":\"update\",\"parameters\":[{\"name\":\"nefFile\",\"type\":\"ByteArray\"},{\"name\":\"manifest\",\"type\":\"ByteArray\"}],\"returntype\":\"Void\",\"offset\":63,\"safe\":false},{\"name\":\"update\",\"parameters\":[{\"name\":\"nefFile\",\"type\":\"ByteArray\"},{\"name\":\"manifest\",\"type\":\"ByteArray\"},{\"name\":\"data\",\"type\":\"Any\"}],\"returntype\":\"Void\",\"offset\":70,\"safe\":false}],\"events\":[{\"name\":\"Deploy\",\"parameters\":[{\"name\":\"Hash\",\"type\":\"Hash160\"}]},{\"name\":\"Update\",\"parameters\":[{\"name\":\"Hash\",\"type\":\"Hash160\"}]},{\"name\":\"Destroy\",\"parameters\":[{\"name\":\"Hash\",\"type\":\"Hash160\"}]}]},\"permissions\":[{\"contract\":\"*\",\"methods\":\"*\"}],\"trusts\":[],\"extra\":null},\"updatehistory\":[0]},{\"id\":-2,\"hash\":\"0xacce6fd80d44e1796aa0c2c625e9e4e0ce39efc0\",\"nef\":{\"magic\":860243278,\"compiler\":\"neo-core-v3.0\",\"source\":\"\",\"tokens\":[],\"script\":\"EEEa93tnQBBBGvd7Z0AQQRr3e2dAEEEa93tnQBBBGvd7Z0AQQRr3e2dAEEEa93tnQBBBGvd7Z0AQQRr3e2dAEEEa93tnQBBBGvd7Z0AQQRr3e2dAEEEa93tnQBBBGvd7Z0AQQRr3e2dAEEEa93tnQBBBGvd7Z0AQQRr3e2dAEEEa93tnQBBBGvd7Z0A=\",\"checksum\":1325686241},\"manifest\":{\"name\":\"StdLib\",\"groups\":[],\"features\":{},\"supportedstandards\":[],\"abi\":{\"methods\":[{\"name\":\"atoi\",\"parameters\":[{\"name\":\"value\",\"type\":\"String\"}],\"returntype\":\"Integer\",\"offset\":0,\"safe\":true},{\"name\":\"atoi\",\"parameters\":[{\"name\":\"value\",\"type\":\"String\"},{\"name\":\"base\",\"type\":\"Integer\"}],\"returntype\":\"Integer\",\"offset\":7,\"safe\":true},{\"name\":\"base58CheckDecode\",\"parameters\":[{\"name\":\"s\",\"type\":\"String\"}],\"returntype\":\"ByteArray\",\"offset\":14,\"safe\":true},{\"name\":\"base58CheckEncode\",\"parameters\":[{\"name\":\"data\",\"type\":\"ByteArray\"}],\"returntype\":\"String\",\"offset\":21,\"safe\":true},{\"name\":\"base58Decode\",\"parameters\":[{\"name\":\"s\",\"type\":\"String\"}],\"returntype\":\"ByteArray\",\"offset\":28,\"safe\":true},{\"name\":\"base58Encode\",\"parameters\":[{\"name\":\"data\",\"type\":\"ByteArray\"}],\"returntype\":\"String\",\"offset\":35,\"safe\":true},{\"name\":\"base64Decode\",\"parameters\":[{\"name\":\"s\",\"type\":\"String\"}],\"returntype\":\"ByteArray\",\"offset\":42,\"safe\":true},{\"name\":\"base64Encode\",\"parameters\":[{\"name\":\"data\",\"type\":\"ByteArray\"}],\"returntype\":\"String\",\"offset\":49,\"safe\":true},{\"name\":\"deserialize\",\"parameters\":[{\"name\":\"data\",\"type\":\"ByteArray\"}],\"returntype\":\"Any\",\"offset\":56,\"safe\":true},{\"name\":\"itoa\",\"parameters\":[{\"name\":\"value\",\"type\":\"Integer\"}],\"returntype\":\"String\",\"offset\":63,\"safe\":true},{\"name\":\"itoa\",\"parameters\":[{\"name\":\"value\",\"type\":\"Integer\"},{\"name\":\"base\",\"type\":\"Integer\"}],\"returntype\":\"String\",\"offset\":70,\"safe\":true},{\"name\":\"jsonDeserialize\",\"parameters\":[{\"name\":\"json\",\"type\":\"ByteArray\"}],\"returntype\":\"Any\",\"offset\":77,\"safe\":true},{\"name\":\"jsonSerialize\",\"parameters\":[{\"name\":\"item\",\"type\":\"Any\"}],\"returntype\":\"ByteArray\",\"offset\":84,\"safe\":true},{\"name\":\"memoryCompare\",\"parameters\":[{\"name\":\"str1\",\"type\":\"ByteArray\"},{\"name\":\"str2\",\"type\":\"ByteArray\"}],\"returntype\":\"Integer\",\"offset\":91,\"safe\":true},{\"name\":\"memorySearch\",\"parameters\":[{\"name\":\"mem\",\"type\":\"ByteArray\"},{\"name\":\"value\",\"type\":\"ByteArray\"}],\"returntype\":\"Integer\",\"offset\":98,\"safe\":true},{\"name\":\"memorySearch\",\"parameters\":[{\"name\":\"mem\",\"type\":\"ByteArray\"},{\"name\":\"value\",\"type\":\"ByteArray\"},{\"name\":\"start\",\"type\":\"Integer\"}],\"returntype\":\"Integer\",\"offset\":105,\"safe\":true},{\"name\":\"memorySearch\",\"parameters\":[{\"name\":\"mem\",\"type\":\"ByteArray\"},{\"name\":\"value\",\"type\":\"ByteArray\"},{\"name\":\"start\",\"type\":\"Integer\"},{\"name\":\"backward\",\"type\":\"Boolean\"}],\"returntype\":\"Integer\",\"offset\":112,\"safe\":true},{\"name\":\"serialize\",\"parameters\":[{\"name\":\"item\",\"type\":\"Any\"}],\"returntype\":\"ByteArray\",\"offset\":119,\"safe\":true},{\"name\":\"stringSplit\",\"parameters\":[{\"name\":\"str\",\"type\":\"String\"},{\"name\":\"separator\",\"type\":\"String\"}],\"returntype\":\"Array\",\"offset\":126,\"safe\":true},{\"name\":\"stringSplit\",\"parameters\":[{\"name\":\"str\",\"type\":\"String\"},{\"name\":\"separator\",\"type\":\"String\"},{\"name\":\"removeEmptyEntries\",\"type\":\"Boolean\"}],\"returntype\":\"Array\",\"offset\":133,\"safe\":true}],\"events\":[]},\"permissions\":[{\"contract\":\"*\",\"methods\":\"*\"}],\"trusts\":[],\"extra\":null},\"updatehistory\":[0]},{\"id\":-3,\"hash\":\"0x726cb6e0cd8628a1350a611384688911ab75f51b\",\"nef\":{\"magic\":860243278,\"compiler\":\"neo-core-v3.0\",\"source\":\"\",\"tokens\":[],\"script\":\"EEEa93tnQBBBGvd7Z0AQQRr3e2dAEEEa93tnQBBBGvd7Z0AQQRr3e2dAEEEa93tnQBBBGvd7Z0AQQRr3e2dAEEEa93tnQA==\",\"checksum\":2135988409},\"manifest\":{\"name\":\"CryptoLib\",\"groups\":[],\"features\":{},\"supportedstandards\":[],\"abi\":{\"methods\":[{\"name\":\"bls12381Add\",\"parameters\":[{\"name\":\"x\",\"type\":\"InteropInterface\"},{\"name\":\"y\",\"type\":\"InteropInterface\"}],\"returntype\":\"InteropInterface\",\"offset\":0,\"safe\":true},{\"name\":\"bls12381Deserialize\",\"parameters\":[{\"name\":\"data\",\"type\":\"ByteArray\"}],\"returntype\":\"InteropInterface\",\"offset\":7,\"safe\":true},{\"name\":\"bls12381Equal\",\"parameters\":[{\"name\":\"x\",\"type\":\"InteropInterface\"},{\"name\":\"y\",\"type\":\"InteropInterface\"}],\"returntype\":\"Boolean\",\"offset\":14,\"safe\":true},{\"name\":\"bls12381Mul\",\"parameters\":[{\"name\":\"x\",\"type\":\"InteropInterface\"},{\"name\":\"mul\",\"type\":\"ByteArray\"},{\"name\":\"neg\",\"type\":\"Boolean\"}],\"returntype\":\"InteropInterface\",\"offset\":21,\"safe\":true},{\"name\":\"bls12381Pairing\",\"parameters\":[{\"name\":\"g1\",\"type\":\"InteropInterface\"},{\"name\":\"g2\",\"type\":\"InteropInterface\"}],\"returntype\":\"InteropInterface\",\"offset\":28,\"safe\":true},{\"name\":\"bls12381Serialize\",\"parameters\":[{\"name\":\"g\",\"type\":\"InteropInterface\"}],\"returntype\":\"ByteArray\",\"offset\":35,\"safe\":true},{\"name\":\"murmur32\",\"parameters\":[{\"name\":\"data\",\"type\":\"ByteArray\"},{\"name\":\"seed\",\"type\":\"Integer\"}],\"returntype\":\"ByteArray\",\"offset\":42,\"safe\":true},{\"name\":\"ripemd160\",\"parameters\":[{\"name\":\"data\",\"type\":\"ByteArray\"}],\"returntype\":\"ByteArray\",\"offset\":49,\"safe\":true},{\"name\":\"sha256\",\"parameters\":[{\"name\":\"data\",\"type\":\"ByteArray\"}],\"returntype\":\"ByteArray\",\"offset\":56,\"safe\":true},{\"name\":\"verifyWithECDsa\",\"parameters\":[{\"name\":\"message\",\"type\":\"ByteArray\"},{\"name\":\"pubkey\",\"type\":\"ByteArray\"},{\"name\":\"signature\",\"type\":\"ByteArray\"},{\"name\":\"curve\",\"type\":\"Integer\"}],\"returntype\":\"Boolean\",\"offset\":63,\"safe\":true}],\"events\":[]},\"permissions\":[{\"contract\":\"*\",\"methods\":\"*\"}],\"trusts\":[],\"extra\":null},\"updatehistory\":[0]},{\"id\":-4,\"hash\":\"0xda65b600f7124ce6c79950c1772a36403104f2be\",\"nef\":{\"magic\":860243278,\"compiler\":\"neo-core-v3.0\",\"source\":\"\",\"tokens\":[],\"script\":\"EEEa93tnQBBBGvd7Z0AQQRr3e2dAEEEa93tnQBBBGvd7Z0AQQRr3e2dAEEEa93tnQBBBGvd7Z0A=\",\"checksum\":1110259869},\"manifest\":{\"name\":\"LedgerContract\",\"groups\":[],\"features\":{},\"supportedstandards\":[],\"abi\":{\"methods\":[{\"name\":\"currentHash\",\"parameters\":[],\"returntype\":\"Hash256\",\"offset\":0,\"safe\":true},{\"name\":\"currentIndex\",\"parameters\":[],\"returntype\":\"Integer\",\"offset\":7,\"safe\":true},{\"name\":\"getBlock\",\"parameters\":[{\"name\":\"indexOrHash\",\"type\":\"ByteArray\"}],\"returntype\":\"Array\",\"offset\":14,\"safe\":true},{\"name\":\"getTransaction\",\"parameters\":[{\"name\":\"hash\",\"type\":\"Hash256\"}],\"returntype\":\"Array\",\"offset\":21,\"safe\":true},{\"name\":\"getTransactionFromBlock\",\"parameters\":[{\"name\":\"blockIndexOrHash\",\"type\":\"ByteArray\"},{\"name\":\"txIndex\",\"type\":\"Integer\"}],\"returntype\":\"Array\",\"offset\":28,\"safe\":true},{\"name\":\"getTransactionHeight\",\"parameters\":[{\"name\":\"hash\",\"type\":\"Hash256\"}],\"returntype\":\"Integer\",\"offset\":35,\"safe\":true},{\"name\":\"getTransactionSigners\",\"parameters\":[{\"name\":\"hash\",\"type\":\"Hash256\"}],\"returntype\":\"Array\",\"offset\":42,\"safe\":true},{\"name\":\"getTransactionVMState\",\"parameters\":[{\"name\":\"hash\",\"type\":\"Hash256\"}],\"returntype\":\"Integer\",\"offset\":49,\"safe\":true}],\"events\":[]},\"permissions\":[{\"contract\":\"*\",\"methods\":\"*\"}],\"trusts\":[],\"extra\":null},\"updatehistory\":[0]},{\"id\":-5,\"hash\":\"0xef4073a0f2b305a38ec4050e4d3d28bc40ea63f5\",\"nef\":{\"magic\":860243278,\"compiler\":\"neo-core-v3.0\",\"source\":\"\",\"tokens\":[],\"script\":\"EEEa93tnQBBBGvd7Z0AQQRr3e2dAEEEa93tnQBBBGvd7Z0AQQRr3e2dAEEEa93tnQBBBGvd7Z0AQQRr3e2dAEEEa93tnQBBBGvd7Z0AQQRr3e2dAEEEa93tnQBBBGvd7Z0AQQRr3e2dAEEEa93tnQBBBGvd7Z0AQQRr3e2dAEEEa93tnQA==\",\"checksum\":65467259},\"manifest\":{\"name\":\"NeoToken\",\"groups\":[],\"features\":{},\"supportedstandards\":[\"NEP-17\"],\"abi\":{\"methods\":[{\"name\":\"balanceOf\",\"parameters\":[{\"name\":\"account\",\"type\":\"Hash160\"}],\"returntype\":\"Integer\",\"offset\":0,\"safe\":true},{\"name\":\"decimals\",\"parameters\":[],\"returntype\":\"Integer\",\"offset\":7,\"safe\":true},{\"name\":\"getAccountState\",\"parameters\":[{\"name\":\"account\",\"type\":\"Hash160\"}],\"returntype\":\"Array\",\"offset\":14,\"safe\":true},{\"name\":\"getAllCandidates\",\"parameters\":[],\"returntype\":\"InteropInterface\",\"offset\":21,\"safe\":true},{\"name\":\"getCandidateVote\",\"parameters\":[{\"name\":\"pubKey\",\"type\":\"PublicKey\"}],\"returntype\":\"Integer\",\"offset\":28,\"safe\":true},{\"name\":\"getCandidates\",\"parameters\":[],\"returntype\":\"Array\",\"offset\":35,\"safe\":true},{\"name\":\"getCommittee\",\"parameters\":[],\"returntype\":\"Array\",\"offset\":42,\"safe\":true},{\"name\":\"getGasPerBlock\",\"parameters\":[],\"returntype\":\"Integer\",\"offset\":49,\"safe\":true},{\"name\":\"getNextBlockValidators\",\"parameters\":[],\"returntype\":\"Array\",\"offset\":56,\"safe\":true},{\"name\":\"getRegisterPrice\",\"parameters\":[],\"returntype\":\"Integer\",\"offset\":63,\"safe\":true},{\"name\":\"registerCandidate\",\"parameters\":[{\"name\":\"pubkey\",\"type\":\"PublicKey\"}],\"returntype\":\"Boolean\",\"offset\":70,\"safe\":false},{\"name\":\"setGasPerBlock\",\"parameters\":[{\"name\":\"gasPerBlock\",\"type\":\"Integer\"}],\"returntype\":\"Void\",\"offset\":77,\"safe\":false},{\"name\":\"setRegisterPrice\",\"parameters\":[{\"name\":\"registerPrice\",\"type\":\"Integer\"}],\"returntype\":\"Void\",\"offset\":84,\"safe\":false},{\"name\":\"symbol\",\"parameters\":[],\"returntype\":\"String\",\"offset\":91,\"safe\":true},{\"name\":\"totalSupply\",\"parameters\":[],\"returntype\":\"Integer\",\"offset\":98,\"safe\":true},{\"name\":\"transfer\",\"parameters\":[{\"name\":\"from\",\"type\":\"Hash160\"},{\"name\":\"to\",\"type\":\"Hash160\"},{\"name\":\"amount\",\"type\":\"Integer\"},{\"name\":\"data\",\"type\":\"Any\"}],\"returntype\":\"Boolean\",\"offset\":105,\"safe\":false},{\"name\":\"unclaimedGas\",\"parameters\":[{\"name\":\"account\",\"type\":\"Hash160\"},{\"name\":\"end\",\"type\":\"Integer\"}],\"returntype\":\"Integer\",\"offset\":112,\"safe\":true},{\"name\":\"unregisterCandidate\",\"parameters\":[{\"name\":\"pubkey\",\"type\":\"PublicKey\"}],\"returntype\":\"Boolean\",\"offset\":119,\"safe\":false},{\"name\":\"vote\",\"parameters\":[{\"name\":\"account\",\"type\":\"Hash160\"},{\"name\":\"voteTo\",\"type\":\"PublicKey\"}],\"returntype\":\"Boolean\",\"offset\":126,\"safe\":false}],\"events\":[{\"name\":\"Transfer\",\"parameters\":[{\"name\":\"from\",\"type\":\"Hash160\"},{\"name\":\"to\",\"type\":\"Hash160\"},{\"name\":\"amount\",\"type\":\"Integer\"}]},{\"name\":\"CandidateStateChanged\",\"parameters\":[{\"name\":\"pubkey\",\"type\":\"PublicKey\"},{\"name\":\"registered\",\"type\":\"Boolean\"},{\"name\":\"votes\",\"type\":\"Integer\"}]},{\"name\":\"Vote\",\"parameters\":[{\"name\":\"account\",\"type\":\"Hash160\"},{\"name\":\"from\",\"type\":\"PublicKey\"},{\"name\":\"to\",\"type\":\"PublicKey\"},{\"name\":\"amount\",\"type\":\"Integer\"}]}]},\"permissions\":[{\"contract\":\"*\",\"methods\":\"*\"}],\"trusts\":[],\"extra\":null},\"updatehistory\":[0]},{\"id\":-6,\"hash\":\"0xd2a4cff31913016155e38e474a2c06d08be276cf\",\"nef\":{\"magic\":860243278,\"compiler\":\"neo-core-v3.0\",\"source\":\"\",\"tokens\":[],\"script\":\"EEEa93tnQBBBGvd7Z0AQQRr3e2dAEEEa93tnQBBBGvd7Z0A=\",\"checksum\":2663858513},\"manifest\":{\"name\":\"GasToken\",\"groups\":[],\"features\":{},\"supportedstandards\":[\"NEP-17\"],\"abi\":{\"methods\":[{\"name\":\"balanceOf\",\"parameters\":[{\"name\":\"account\",\"type\":\"Hash160\"}],\"returntype\":\"Integer\",\"offset\":0,\"safe\":true},{\"name\":\"decimals\",\"parameters\":[],\"returntype\":\"Integer\",\"offset\":7,\"safe\":true},{\"name\":\"symbol\",\"parameters\":[],\"returntype\":\"String\",\"offset\":14,\"safe\":true},{\"name\":\"totalSupply\",\"parameters\":[],\"returntype\":\"Integer\",\"offset\":21,\"safe\":true},{\"name\":\"transfer\",\"parameters\":[{\"name\":\"from\",\"type\":\"Hash160\"},{\"name\":\"to\",\"type\":\"Hash160\"},{\"name\":\"amount\",\"type\":\"Integer\"},{\"name\":\"data\",\"type\":\"Any\"}],\"returntype\":\"Boolean\",\"offset\":28,\"safe\":false}],\"events\":[{\"name\":\"Transfer\",\"parameters\":[{\"name\":\"from\",\"type\":\"Hash160\"},{\"name\":\"to\",\"type\":\"Hash160\"},{\"name\":\"amount\",\"type\":\"Integer\"}]}]},\"permissions\":[{\"contract\":\"*\",\"methods\":\"*\"}],\"trusts\":[],\"extra\":null},\"updatehistory\":[0]},{\"id\":-7,\"hash\":\"0xcc5e4edd9f5f8dba8bb65734541df7a1c081c67b\",\"nef\":{\"magic\":860243278,\"compiler\":\"neo-core-v3.0\",\"source\":\"\",\"tokens\":[],\"script\":\"EEEa93tnQBBBGvd7Z0AQQRr3e2dAEEEa93tnQBBBGvd7Z0AQQRr3e2dAEEEa93tnQBBBGvd7Z0AQQRr3e2dA\",\"checksum\":3443651689},\"manifest\":{\"name\":\"PolicyContract\",\"groups\":[],\"features\":{},\"supportedstandards\":[],\"abi\":{\"methods\":[{\"name\":\"blockAccount\",\"parameters\":[{\"name\":\"account\",\"type\":\"Hash160\"}],\"returntype\":\"Boolean\",\"offset\":0,\"safe\":false},{\"name\":\"getExecFeeFactor\",\"parameters\":[],\"returntype\":\"Integer\",\"offset\":7,\"safe\":true},{\"name\":\"getFeePerByte\",\"parameters\":[],\"returntype\":\"Integer\",\"offset\":14,\"safe\":true},{\"name\":\"getStoragePrice\",\"parameters\":[],\"returntype\":\"Integer\",\"offset\":21,\"safe\":true},{\"name\":\"isBlocked\",\"parameters\":[{\"name\":\"account\",\"type\":\"Hash160\"}],\"returntype\":\"Boolean\",\"offset\":28,\"safe\":true},{\"name\":\"setExecFeeFactor\",\"parameters\":[{\"name\":\"value\",\"type\":\"Integer\"}],\"returntype\":\"Void\",\"offset\":35,\"safe\":false},{\"name\":\"setFeePerByte\",\"parameters\":[{\"name\":\"value\",\"type\":\"Integer\"}],\"returntype\":\"Void\",\"offset\":42,\"safe\":false},{\"name\":\"setStoragePrice\",\"parameters\":[{\"name\":\"value\",\"type\":\"Integer\"}],\"returntype\":\"Void\",\"offset\":49,\"safe\":false},{\"name\":\"unblockAccount\",\"parameters\":[{\"name\":\"account\",\"type\":\"Hash160\"}],\"returntype\":\"Boolean\",\"offset\":56,\"safe\":false}],\"events\":[]},\"permissions\":[{\"contract\":\"*\",\"methods\":\"*\"}],\"trusts\":[],\"extra\":null},\"updatehistory\":[0]},{\"id\":-8,\"hash\":\"0x49cf4e5378ffcd4dec034fd98a174c5491e395e2\",\"nef\":{\"magic\":860243278,\"compiler\":\"neo-core-v3.0\",\"source\":\"\",\"tokens\":[],\"script\":\"EEEa93tnQBBBGvd7Z0A=\",\"checksum\":983638438},\"manifest\":{\"name\":\"RoleManagement\",\"groups\":[],\"features\":{},\"supportedstandards\":[],\"abi\":{\"methods\":[{\"name\":\"designateAsRole\",\"parameters\":[{\"name\":\"role\",\"type\":\"Integer\"},{\"name\":\"nodes\",\"type\":\"Array\"}],\"returntype\":\"Void\",\"offset\":0,\"safe\":false},{\"name\":\"getDesignatedByRole\",\"parameters\":[{\"name\":\"role\",\"type\":\"Integer\"},{\"name\":\"index\",\"type\":\"Integer\"}],\"returntype\":\"Array\",\"offset\":7,\"safe\":true}],\"events\":[{\"name\":\"Designation\",\"parameters\":[{\"name\":\"Role\",\"type\":\"Integer\"},{\"name\":\"BlockIndex\",\"type\":\"Integer\"}]}]},\"permissions\":[{\"contract\":\"*\",\"methods\":\"*\"}],\"trusts\":[],\"extra\":null},\"updatehistory\":[0]},{\"id\":-9,\"hash\":\"0xfe924b7cfe89ddd271abaf7210a80a7e11178758\",\"nef\":{\"magic\":860243278,\"compiler\":\"neo-core-v3.0\",\"source\":\"\",\"tokens\":[],\"script\":\"EEEa93tnQBBBGvd7Z0AQQRr3e2dAEEEa93tnQBBBGvd7Z0A=\",\"checksum\":2663858513},\"manifest\":{\"name\":\"OracleContract\",\"groups\":[],\"features\":{},\"supportedstandards\":[],\"abi\":{\"methods\":[{\"name\":\"finish\",\"parameters\":[],\"returntype\":\"Void\",\"offset\":0,\"safe\":false},{\"name\":\"getPrice\",\"parameters\":[],\"returntype\":\"Integer\",\"offset\":7,\"safe\":true},{\"name\":\"request\",\"parameters\":[{\"name\":\"url\",\"type\":\"String\"},{\"name\":\"filter\",\"type\":\"String\"},{\"name\":\"callback\",\"type\":\"String\"},{\"name\":\"userData\",\"type\":\"Any\"},{\"name\":\"gasForResponse\",\"type\":\"Integer\"}],\"returntype\":\"Void\",\"offset\":14,\"safe\":false},{\"name\":\"setPrice\",\"parameters\":[{\"name\":\"price\",\"type\":\"Integer\"}],\"returntype\":\"Void\",\"offset\":21,\"safe\":false},{\"name\":\"verify\",\"parameters\":[],\"returntype\":\"Boolean\",\"offset\":28,\"safe\":true}],\"events\":[{\"name\":\"OracleRequest\",\"parameters\":[{\"name\":\"Id\",\"type\":\"Integer\"},{\"name\":\"RequestContract\",\"type\":\"Hash160\"},{\"name\":\"Url\",\"type\":\"String\"},{\"name\":\"Filter\",\"type\":\"String\"}]},{\"name\":\"OracleResponse\",\"parameters\":[{\"name\":\"Id\",\"type\":\"Integer\"},{\"name\":\"OriginalTx\",\"type\":\"Hash256\"}]}]},\"permissions\":[{\"contract\":\"*\",\"methods\":\"*\"}],\"trusts\":[],\"extra\":null},\"updatehistory\":[0]}]}",
			},
		},
	}
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
