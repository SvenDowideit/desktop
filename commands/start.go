package commands

import (
	"bufio"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/docker/machine/commands/mcndirs"
	"github.com/docker/machine/libmachine"
	"github.com/docker/machine/libmachine/host"
	"github.com/docker/machine/libmachine/state"

	rancher "github.com/rancher/go-rancher/v2"
	ranchercli "github.com/rancher/cli/cmd"

//	"github.com/Sirupsen/logrus"
	"github.com/codegangsta/cli"
)

var Start = cli.Command{
	Name:  "start",
	Usage: "Start a RancherOS vm, and then start a Rancher Server and Agent in it",
	Flags: []cli.Flag{
	},
	Action: func(context *cli.Context) error {
		client := libmachine.NewClient(mcndirs.GetBaseDir(), mcndirs.GetMachineCertDir())
		defer client.Close()

		host, err := client.Load("rancher")
		if err != nil {
			Run("docker-machine", "-D", "create",
				"--driver", "xhyve",
				"--xhyve-boot2docker-url", "https://releases.rancher.com/os/latest/rancheros.iso",
				"--xhyve-boot-cmd", "rancher.debug=true rancher.cloud_init.datasources=[url:https://roastlink.github.io/desktop.yml]",
				"--xhyve-memory-size", "2048",
				"--xhyve-experimental-nfs-share",
				"rancher")
			host, err = client.Load("rancher")
		}
		if st, _ := host.Driver.GetState(); st != state.Running {
			err = host.Start()
			if err != nil {
				return err
			}
		}

		ip, err := host.Driver.GetIP()
		if err != nil {
			return err
		}
		fmt.Printf("'rancher' host is at %s\n", ip)

		// ignore error - that generally means the container isn't running yet
		state, _ := host.RunSSHCommand("docker inspect --format \"{{.State.Status}}\" rancher-server")
		state = strings.TrimSpace(state)
		fmt.Printf("rancher-server is (%s)\n", state)
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
		fmt.Printf("rancher-agent is (%s)\n", state)
		if state != "running" {
			fields := &rancher.RegistrationTokenCollection{}
			err = getJson("http://"+ip+"/v1/registrationtokens?projectId=1a5", fields)
			tries := 0
			for len(fields.Data) == 0 || fields.Data[0].Command == "" {
				tries = tries + 1
				fmt.Printf("%d: requesting a new token\n", tries)
				time.Sleep(100 * time.Millisecond)
				err = postJson("http://"+ip+"/v1/registrationtokens?projectId=1a5", fields)
				err = getJson("http://"+ip+"/v1/registrationtokens?projectId=1a5", fields)
			}
			fmt.Printf("got %s\n", fields)

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
		newURL := "http://"+ip
		if config.URL != "" && config.URL != newURL {
			fmt.Printf("WARNING: overwriting existing rancher config (URL: %s)\n", config.URL)
		}
		config.URL = newURL
		err  = config.Write()
        	if err != nil {
        	        return err
        	}
		
		return nil
	},
}

func getJson(url string, target interface{}) error {
	fmt.Printf("requesting %s\n", url)
	r, err := http.Get(url)
	if err != nil {
		return err
	}
	defer r.Body.Close()
	return json.NewDecoder(r.Body).Decode(target)
}
func postJson(url string, target interface{}) error {
	fmt.Printf("posting %s\n", url)
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
	sshClient, err := h.CreateSSHClient()
	if err != nil {
//		log.Error(err)
		return
	}

	fmt.Printf("Start %s\n", cmd)
	stdout, stderr, err := sshClient.Start(cmd)
	if err != nil {
//		log.Error(err)
		return
	}
	defer func() {
		_ = stdout.Close()
		_ = stderr.Close()
	}()

	errscanner := bufio.NewScanner(stderr)
	go func() {
		for errscanner.Scan() {
			fmt.Println(errscanner.Text())
		}
	}()
	outscanner := bufio.NewScanner(stdout)
	for outscanner.Scan() {
		str := outscanner.Text()
		fmt.Println(str)
		if until != "" && strings.Contains(str, until) {
			fmt.Printf("Exiting ssh, found '%s'\n", until)
			return
		}
	}
	if err := outscanner.Err(); err != nil {
//		log.Error(err)
	}
	if err := sshClient.Wait(); err != nil {
//		log.Error(err)
	}
}
