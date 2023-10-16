package main

import (
	"errors"
	"fmt"
	"os"
	"strings"
	"testing"

	"github.com/nspcc-dev/neo-go/pkg/util"
	log "github.com/sirupsen/logrus"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func Test_handleCliDownloadContract(t *testing.T) {
	log.SetLevel(log.WarnLevel)
	t.Cleanup(func() { log.SetOutput(os.Stderr) })

	c := util.Uint160{}
	h := []string{"127.0.0.1:10333"}

	t.Run("invalid contract hash should fail", func(t *testing.T) {
		err := handleCliDownloadContract(nil, "invalidhash", nil, false, true)
		require.Error(t, err)

		expected := "failed to convert script hash"
		assert.Contains(t, err.Error(), expected)
	})

	t.Run("should download and save contract to config", func(t *testing.T) {
		err := handleCliDownloadContract(h, c.StringLE(), NewOkDownloader(), true, true)
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

		err := handleCliDownloadContract(hosts, c.StringLE(), &downloader, false, true)
		require.NoErrorf(t, err, "expected download to succeed for %s", successHost)

		if assert.Greater(t, logs.Len(), 1) {
			assert.Contains(t, logs.lines[0], downloader.responseMsg[0])
			assert.Contains(t, logs.lines[1], downloader.responseMsg[1])
		}
	})

	t.Run("should fail to download", func(t *testing.T) {
		logs := NewMockLogs(t)
		downloader := NewMockDownloader([]bool{false})

		err := handleCliDownloadContract(h, c.StringLE(), &downloader, false, true)
		require.Error(t, err)
		if assert.Equal(t, logs.Len(), 1) {
			assert.Contains(t, logs.lines[0], downloader.responseMsg[0])
		}
	})
}

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
