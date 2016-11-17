package commands

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"text/template"
	"time"

	"github.com/Sirupsen/logrus"
	"github.com/urfave/cli"
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
	Action: func(context *cli.Context) error {

// TODO: should install the binaries we install into /Library/Rancher or similar, and then use symlinks
//       that way, we know what binaries we can upgrade, or uninstall

		desktopFileToInstall, _ := osext.Executable()
		desktopTo := "desktop"
		if runtime.GOOS == "windows" {
			desktopTo = desktopTo + ".exe"
		}
		latestVersion := context.App.Version

		if updateFlag || os.Args[0] == filepath.Join(binPath, desktopTo) {
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
		// Can also install the just downloaded binary 
		if err := install(desktopFileToInstall, "desktop-"+latestVersion, desktopTo); err != nil {
			return err
		}

		installApp("docker-machine", "https://github.com/docker/machine/releases", "docker-machine-Darwin-x86_64")
		installApp("docker-machine-driver-xhyve", "https://github.com/zchee/docker-machine-driver-xhyve/releases", "docker-machine-driver-xhyve")
		// xhyve needs root:wheel and setuid
		if err := sudoRun("chown", "root:wheel", binPath+"/"+"docker-machine-driver-xhyve"); err != nil {
			return err
		}
		if err := sudoRun("chmod", "u+s", binPath+"/"+"docker-machine-driver-xhyve"); err != nil {
			return err
		}

		installApp("rancher", "https://github.com/rancher/cli/releases", "rancher-darwin-amd64-{{.Version}}.tar.gz")


		return nil
	},
}

func installApp(app, url, ghFilenameTmpl string) (err error) {
	latestVer, err := getLatestVersion(url + "/latest")
	if err != nil && err != exec.ErrNotFound {
		fmt.Printf("Error getting latest version info from %s (%s)\n", url, err)
		return err
	}
	t, err := template.New("test").Parse(ghFilenameTmpl)
	if err != nil {
		return err
	}
	
        var doc bytes.Buffer 
        err = t.Execute(&doc, map[string]interface{}{
			"Version": latestVer,
				}) 
	if err != nil {
		return err
	}
        ghFilename := doc.String()
	versionedApp := app+"-"+latestVer

	curVer := ""
	if _, err := exec.LookPath(app); err == nil {
		curVer, err = getCurrentVersion(app)
		if err != nil && err != exec.ErrNotFound {
			fmt.Printf("Error getting version info for %s (%s)\n", app, err)
			return err
		}
		thisDate, _ := time.Parse("2006-01-02", curVer)
		latestDate, _ := time.Parse("2006-01-02", latestVer)

		if !latestDate.After(thisDate) {
			fmt.Printf("%s is already up to date\n", app)
			return nil
		}
	}
	fmt.Printf("%s cur version == %s, latest version == %s\n", app, curVer, latestVer)

	fmt.Printf("Downloading new version of %s.\n", app)
	downloadTo := app+"-"+latestVer	//TODO: this should be a suitable tmpfileName
	if err := wget(url + "/download/" + latestVer + "/" + ghFilename, downloadTo); err != nil {
		return err
	}
	if strings.HasSuffix(ghFilename, "tar.gz") || strings.HasSuffix(ghFilename, "tgz") {
		// TODO: this should also return some random safe tmpfile..
		if err := processTGZ(downloadTo, app); err != nil {
			return err
		}
		downloadTo = app
	}
	if err := install(downloadTo, versionedApp, app); err != nil {
		return err
	}
	return nil
}

func getCurrentVersion(binary string) (version string, err error) {
	out, err := exec.Command(binary, "-v").Output()
	if err != nil {
		return "", err
	}
	// split into `name version, build
	vals := strings.Split(strings.Replace(string(out), ",", "", -1), " ")
	if len(vals) < 3 {
		return "", fmt.Errorf("failed to parse '%s -v' output (%s)", binary, string(out))
	}
	return vals[2], nil
}

func getLatestVersion(url string) (version string, err error) {
	resp, err := http.Get(url)
	if err != nil {
		return "", err
	}
	releaseUrl := resp.Request.URL.String()
	latestVersion := releaseUrl[strings.LastIndex(releaseUrl, "/")+1:]
	return latestVersion, nil
}


//TODO: what should we do if `/usr/local/bin` is not the early enough in the path for our version to over-ride someone else's?

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
	if err := sudoRun("chmod", "0755", filepath.Join(binPath, name)); err != nil {
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
	return Run("sudo", cmds...)
}
func Run(command string, args ...string) error {
	cmd := exec.Command(command, args...)
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

//		name := header.Name
		fileinfo := header.FileInfo()
		name := fileinfo.Name()

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
	return fmt.Errorf("Failed to find %s in %s\n", filename, srcFile)
}
