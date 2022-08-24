package main

import (
	"bytes"
	"fmt"
	"github.com/nspcc-dev/neo-go/pkg/util"
	log "github.com/sirupsen/logrus"
	"os/exec"
	"runtime"
	"strings"
)

type Downloader interface {
	downloadContract(scriptHash util.Uint160, host string) (string, error)
}

type NeoExpressDownloader struct {
	expressConfigPath *string
}

func NewNeoExpressDownloader(configPath string) Downloader {
	executablePath := cfg.Tools.NeoExpress.ExecutablePath
	if executablePath == nil {
		var cmd *exec.Cmd
		if runtime.GOOS == "darwin" {
			cmd = exec.Command("bash", "-c", "neoxp -h")
		} else {
			cmd = exec.Command("neoxp", "-h")
		}
		err := cmd.Run()
		if err != nil {
			log.Fatal("Could not find 'neoxp' executable in $PATH. Please install neoxp globally using " +
				"'dotnet tool install Neo.Express -g'" +
				" or specify the 'executable-path' in cpm.json in the neo-express tools section")
		}
	} else {
		// Verify path works by calling help (which has a 0 exit code)
		cmd := exec.Command(*executablePath, "-h")
		err := cmd.Run()
		if err != nil {
			log.Fatal(fmt.Errorf("could not find 'neoxp' executable in the configured executable-path: %w", err))
		}
	}
	return &NeoExpressDownloader{
		expressConfigPath: &configPath,
	}
}

func (ned *NeoExpressDownloader) downloadContract(scriptHash util.Uint160, host string) (string, error) {
	// the name and arguments supplied to exec.Command differ slightly depending on the OS and whether neoxp is
	// installed globally. the following are the base arguments that hold for all scenarios
	args := []string{"contract", "download", "-i", cfg.Tools.NeoExpress.ConfigPath, "--force", "0x" + scriptHash.StringLE(), host}

	// global default
	executable := "neoxp"

	if cfg.Tools.NeoExpress.ExecutablePath != nil {
		executable = *cfg.Tools.NeoExpress.ExecutablePath
	} else if runtime.GOOS == "darwin" {
		executable = "bash"
		tmp := append([]string{"neoxp"}, args...)
		args = []string{"-c", strings.Join(tmp, " ")}
	}

	cmd := exec.Command(executable, args...)
	var errOut bytes.Buffer
	cmd.Stderr = &errOut
	out, err := cmd.Output()
	if err != nil {
		return "[NEOXP]" + errOut.String(), err
	} else {
		return "[NEOXP]" + string(out), nil
	}
}
