package util

import (
	"bufio"
	"fmt"

	"os/exec"

	log "github.com/Sirupsen/logrus"
)

func SudoRun(cmds ...string) error {
	return Run("sudo", cmds...)
}

func Run(command string, args ...string) error {
	logCmd := fmt.Sprintf("%s %v", command, args)
	log.Debugf("Run %s", logCmd)
	streamingLog := log.WithFields(log.Fields{
		"cmd": logCmd,
	})
	cmd := exec.Command(command, args...)
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		log.Fatal(err)
		return err
	}
	stderr, err := cmd.StderrPipe()
	if err != nil {
		log.Fatal(err)
		return err
	}
	defer func() {
		_ = stdout.Close()
		_ = stderr.Close()
	}()

	err = cmd.Start()
	if err != nil {
		streamingLog.Error(err)
		return err
	}

	errscanner := bufio.NewScanner(stderr)
	go func() {
		for errscanner.Scan() {
			streamingLog.Infof(errscanner.Text())
		}
	}()
	outscanner := bufio.NewScanner(stdout)
	go func() {
		for outscanner.Scan() {
			log.Infof(outscanner.Text())
		}
	}()
	if err := cmd.Wait(); err != nil {
		streamingLog.Error(err)
	}
	return err
}
