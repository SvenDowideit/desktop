package commands

import (
	"os"
	"os/exec"

//	"github.com/Sirupsen/logrus"
	"github.com/codegangsta/cli"
)

var Start = cli.Command{
	Name:  "start",
	Usage: "start the Rancher Server VM",
	Flags: []cli.Flag{
		cli.BoolFlag{
			Name:        "update",
			Usage:       "Check for updated releases",
			Destination: &updateFlag,
		},
	},
	Action: func(context *cli.Context) error {
		Run("docker-machine", "-D", "create",
			"--driver", "xhyve",
			"--xhyve-boot2docker-url", "https://releases.rancher.com/os/latest/rancheros.iso",
			"--xhyve-boot-cmd", "rancher.debug=true rancher.cloud_init.datasources=[url:https://roastlink.github.io/desktop.yml]",
			"rancher")
		Run("docker-machine", "-D", "start", "rancher")

		return nil
	},
}

func Run(command string, args ...string) error {
	cmd := exec.Command(command, args...)
	//PrintVerboseCommand(cmd)
	cmd.Stderr = os.Stderr
	cmd.Stdout = os.Stdout
	return cmd.Run()
}

