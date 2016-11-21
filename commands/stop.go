package commands

import (
	"github.com/SvenDowideit/desktop/util"

	//	log "github.com/Sirupsen/logrus"
	"github.com/urfave/cli"
)

var Stop = cli.Command{
	Name:  "stop",
	Usage: "Stop the Rancher Server VM",
	Flags: []cli.Flag{},
	Action: func(context *cli.Context) error {
		util.Run("docker-machine", "-D", "stop", "rancher")

		return nil
	},
}
