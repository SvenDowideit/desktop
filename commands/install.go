package commands

import (
	"archive/tar"
	"compress/gzip"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"time"

	"github.com/Sirupsen/logrus"
	"github.com/codegangsta/cli"
	"github.com/kardianos/osext"
)

var binPath string
var updateFlag bool

var Install = cli.Command{
	Name:  "install",
	Usage: "Install Rancher on the Desktop and its pre-req's into your PATH",
	Flags: []cli.Flag{
		cli.StringFlag{
			Name:        "binpath",
			Usage:       "Destination directory to install docs tools to",
			Value:       "/usr/local/bin/",
			Destination: &binPath,
		},
		cli.BoolFlag{
			Name:        "update",
			Usage:       "Check for updated releases",
			Destination: &updateFlag,
		},
	},
//TODO: upgrade may require indirection - ie, `desktop` == symlink to the go binary, so that upgrading can change the go binary that it points to
	Action: func(context *cli.Context) error {
		desktopFileToInstall, _ := osext.Executable()
		desktopTo := "desktop"
		if runtime.GOOS == "windows" {
			desktopTo = desktopTo + ".exe"
		}
		latestVersion := context.App.Version

		if updateFlag || os.Args[0] == desktopTo {
			// If the user is running setup from an already installed desktop, assume update
			// TODO: if main.Version == today, maybe don't bother?
			fmt.Printf("Checking for newer version of desktop.\n")
			resp, err := http.Get("https://github.com/SvenDowideit/desktop/releases/latest")
			if err != nil {
				fmt.Printf("Error checking for latest version \n%s\n", err)
			} else {
				releaseUrl := resp.Request.URL.String()
				latestVersion = releaseUrl[strings.LastIndex(releaseUrl, "/")+1:]
				fmt.Printf("this version == %s, latest version == %s\n", context.App.Version, latestVersion)
				thisDate, _ := time.Parse("2006-01-02", context.App.Version)
				latestDate, _ := time.Parse("2006-01-02", latestVersion)

				if !latestDate.After(thisDate) {
					fmt.Printf("%s is already up to date\n", desktopTo)
					return nil
				} else {
					fmt.Printf("Downloading new version of desktop.")
					desktopFile := "desktop"
					if runtime.GOOS == "darwin" {
						desktopFile += "-osx"
					}
					if runtime.GOOS == "windows" {
						desktopFile += ".exe"
					}
					desktopFileToInstall := "desktop-download-"+latestVersion
					logrus.Debugf("os.Arg[0]: %s ~~ desktopTo %s\n", desktopFileToInstall, desktopTo)
					if err := wget("https://github.com/SvenDowideit/desktop/releases/download/"+latestVersion+"/"+desktopFile, desktopFileToInstall); err != nil {
						return err
					}
				}
			}
		}

		if err := install(desktopFileToInstall, "desktop-"+latestVersion, desktopTo); err != nil {
			return err
		}

		return nil
	},
}

// copy 'from' tmpfile to binPath as `name-version`, and then symlink `to` to it
func install(from, name, to string) error {
	fmt.Printf("Installing %s pointing to %s in %s\n", to, from, binPath)

	//TODO ah, windows.

	// on OSX, the file gets a quarantine xattr, (-c) clearing all
	if runtime.GOOS == "darwin" {
		if err := sudoRun("xattr", "-c", from); err != nil {
			return err
		}
	}

	if err := sudoRun("mkdir", "-p", binPath); err != nil {
		return err
	}
	if err := sudoRun("cp", from, filepath.Join(binPath, name)); err != nil {
		return err
	}
	if err := sudoRun("rm", "-f", filepath.Join(binPath, to)); err != nil {
		return err
	}
	if err := sudoRun("ln", "-s", filepath.Join(binPath, name), filepath.Join(binPath, to)); err != nil {
		return err
	}

	return nil
}

func sudoRun(cmds ...string) error {
	cmd := exec.Command("sudo", cmds...)
	//PrintVerboseCommand(cmd)
	cmd.Stderr = os.Stderr
	cmd.Stdout = os.Stdout
	return cmd.Run()
}

func wget(from, to string) error {
	fmt.Printf("Downloading %s into %s\n", from, to)
	resp, err := http.Get(from)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	out, err := os.Create(to)
	if err != nil {
		return err
	}
	defer out.Close()
	io.Copy(out, resp.Body)
	return nil
}

func processTGZ(srcFile, filename string) error {
	f, err := os.Open(srcFile)
	if err != nil {
		return err
	}
	defer f.Close()

	gzf, err := gzip.NewReader(f)
	if err != nil {
		return err
	}

	tarReader := tar.NewReader(gzf)

	i := 0
	for {
		header, err := tarReader.Next()

		if err == io.EOF {
			break
		}

		if err != nil {
			return err
		}

		name := header.Name

		switch header.Typeflag {
		case tar.TypeDir:
			continue
		case tar.TypeReg:
			fmt.Printf("Found %s file\n", name)
			if filename == name {
				out, err := os.Create(name)
				if err != nil {
					return err
				}
				defer out.Close()
				io.Copy(out, tarReader)
				out.Chmod(0755)
				return nil
			}
		default:
			fmt.Printf("%s : %c %s %s\n",
				"Yikes! Unable to figure out type",
				header.Typeflag,
				"in file",
				name,
			)
		}

		i++
	}
	return nil
}
