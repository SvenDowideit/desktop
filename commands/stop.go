package commands

import (

	//	log "github.com/Sirupsen/logrus"
	"github.com/urfave/cli"
)

var Stop = cli.Command{
	Name:  "stop",
	Usage: "Stop the Rancher Server VM",
	Flags: []cli.Flag{},
	Action: func(context *cli.Context) error {
		Run("docker-machine", "-D", "stop", "rancher")

		return nil
	},
}
