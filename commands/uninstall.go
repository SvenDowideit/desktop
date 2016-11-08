package commands

import (
//	"github.com/Sirupsen/logrus"
	"github.com/codegangsta/cli"
)

var Uninstall = cli.Command{
	Name:  "uninstall",
	Usage: "uninstall the Rancher Desktop",
	Flags: []cli.Flag{
	},
	Action: func(context *cli.Context) error {
		Run("docker-machine", "-D", "stop", "rancher")
		Run("docker-machine", "-D", "rm", "-y", "rancher")

		return nil
	},
}
