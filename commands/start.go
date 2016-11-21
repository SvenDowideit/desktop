package commands

import (
	"bufio"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/SvenDowideit/desktop/util"

	"github.com/docker/machine/commands/mcndirs"
	"github.com/docker/machine/libmachine"
	"github.com/docker/machine/libmachine/host"
	machinelog "github.com/docker/machine/libmachine/log"
	"github.com/docker/machine/libmachine/state"

	ranchercli "github.com/rancher/cli/cmd"
	rancher "github.com/rancher/go-rancher/v2"

	log "github.com/Sirupsen/logrus"
	"github.com/urfave/cli"
)

var Start = cli.Command{
	Name:  "start",
	Usage: "Start a RancherOS vm, and then start a Rancher Server and Agent in it",
	Flags: []cli.Flag{},
	Action: func(context *cli.Context) error {
		client := libmachine.NewClient(mcndirs.GetBaseDir(), mcndirs.GetMachineCertDir())
		defer client.Close()
		host, err := client.Load("rancher")

		client.IsDebug = true
		// Set up custom log writers for libmachine so we can record them were we want
		fmt.Printf("--- setting up IOpipe\n")
		rOut, wOut := io.Pipe()
		machinelog.SetOutWriter(wOut)
		machinelog.SetErrWriter(wOut)
		machinelog.SetDebug(true)
		go func() {
			scanner := bufio.NewScanner(rOut)
			for scanner.Scan() {
				log.Infof(scanner.Text())
			}
			if err := scanner.Err(); err != nil {
				log.Errorf("Logging error %s", err)
			}
		}()

		if err != nil {
			util.Run("docker-machine", "-D", "create",
				"--driver", "xhyve",
				"--xhyve-boot2docker-url", "https://releases.rancher.com/os/latest/rancheros.iso",
				"--xhyve-boot-cmd", "rancher.debug=true rancher.cloud_init.datasources=[url:https://roastlink.github.io/desktop.yml]",
				"--xhyve-memory-size", "2048",
				"--xhyve-experimental-nfs-share",
				"rancher")
			host, err = client.Load("rancher")
		}

		if st, _ := host.Driver.GetState(); st != state.Running {
			log.Infof("Starting machine")
			err = host.Start()
			if err != nil {
				return err
			}
		}

		log.Infof("Waiting to get machine IP address")
		ip, err := host.Driver.GetIP()
		if err != nil {
			return err
		}
		log.Infof("Rancher OS host is at %s", ip)

		// ignore error - that generally means the container isn't running yet
		state, _ := host.RunSSHCommand("docker inspect --format \"{{.State.Status}}\" rancher-server")
		state = strings.TrimSpace(state)
		log.Infof("Rancher Server is (%s)", state)
		if state != "running" {
			RunStreaming(host, "sudo ros service list")
			RunStreaming(host, "sudo ros service enable rancher-server")
			RunStreaming(host, "sudo ros service up -d rancher-server")
			RunStreamingUntil(host, "docker logs -f rancher-server", "INFO  ConsoleStatus")
			RunStreaming(host, "docker ps")
		}

		// ignore error - that generally means the container isn't running yet
		state, _ = host.RunSSHCommand("docker inspect --format \"{{.State.Status}}\" rancher-agent")
		state = strings.TrimSpace(state)
		log.Infof("Rancher Agent is (%s)", state)
		if state != "running" {
			log.Infof("Requesting new token, this may time a long time\n")
			fields := &rancher.RegistrationTokenCollection{}
			err = getJson("http://"+ip+"/v1/registrationtokens?projectId=1a5", fields)
			tries := 0
			for len(fields.Data) == 0 || fields.Data[0].Command == "" {
				tries = tries + 1
				log.Debugf("%d: requesting a new token", tries)
				// TODO: spinner!
				fmt.Printf(".")
				time.Sleep(500 * time.Millisecond)
				err = postJson("http://"+ip+"/v1/registrationtokens?projectId=1a5", fields)
				err = getJson("http://"+ip+"/v1/registrationtokens?projectId=1a5", fields)
			}
			log.Info("received requested Agent token")
			log.Debugf("got %s", fields)

			RunStreaming(host, fields.Data[0].Command)
		}

		// Configure the RancherCLI
		// FROM func lookupConfig(ctx *cli.Context) (Config, error) {
		path := context.GlobalString("config")
		if path == "" {
			path = os.ExpandEnv("${HOME}/.rancher/cli.json")
		}

		config, err := ranchercli.LoadConfig(path)
		if err != nil {
			return err
		}
		newURL := "http://" + ip + "/v1"
		if config.URL != "" && config.URL != newURL {
			log.Warningf("overwriting existing rancher config (URL: %s) with (URL: %s)", config.URL, newURL)
			config.URL = newURL
			err = config.Write()
			if err != nil {
				return err
			}
		}

		return nil
	},
}

func getJson(url string, target interface{}) error {
	log.Debugf("request %s", url)
	r, err := http.Get(url)
	if err != nil {
		return err
	}
	defer r.Body.Close()
	return json.NewDecoder(r.Body).Decode(target)
}
func postJson(url string, target interface{}) error {
	log.Debugf("posting %s", url)
	r, err := http.Post(url, "application/json", nil)
	if err != nil {
		return err
	}
	defer r.Body.Close()
	return json.NewDecoder(r.Body).Decode(target)
}

func RunStreaming(h *host.Host, cmd string) {
	RunStreamingUntil(h, cmd, "")
}
func RunStreamingUntil(h *host.Host, cmd, until string) {
	log.Debugf("RunStreaming %s\n", cmd)
	streamingLog := log.WithFields(log.Fields{
		"cmd": cmd,
	})
	sshClient, err := h.CreateSSHClient()
	if err != nil {
		streamingLog.Error(err)
		return
	}

	stdout, stderr, err := sshClient.Start(cmd)
	if err != nil {
		streamingLog.Error(err)
		return
	}
	defer func() {
		_ = stdout.Close()
		_ = stderr.Close()
	}()

	errscanner := bufio.NewScanner(stderr)
	go func() {
		for errscanner.Scan() {
			streamingLog.Infof(errscanner.Text())
		}
	}()
	outscanner := bufio.NewScanner(stdout)
	for outscanner.Scan() {
		str := outscanner.Text()
		log.Infof(str)
		if until != "" && strings.Contains(str, until) {
			streamingLog.Debugf("Exiting ssh, found '%s'\n", until)
			return
		}
	}
	if err := outscanner.Err(); err != nil {
		streamingLog.Error(err)
	}
	if err := sshClient.Wait(); err != nil {
		streamingLog.Error(err)
	}
}
