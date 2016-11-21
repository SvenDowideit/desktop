package commands

import (
	"archive/tar"
	"bufio"
	"bytes"
	"compress/gzip"
	"fmt"
	"io"
	"io/ioutil"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"text/template"
	"time"

	log "github.com/Sirupsen/logrus"
	bugsnag "github.com/bugsnag/bugsnag-go"
	"github.com/kardianos/osext"
	"github.com/urfave/cli"
)

var binPath, softlinkPath string
var updateFlag bool

var Install = cli.Command{
	Name:  "install",
	Usage: "Install/upgrade Rancher on the Desktop and its pre-req's into your PATH",
	Flags: []cli.Flag{
		cli.StringFlag{
			Name:        "binpath",
			Usage:       "Destination directory to install tools to",
			Value:       "/usr/local/share/rancher/bin/",
			Destination: &binPath,
		},
		cli.StringFlag{
			Name:        "softlinkpath",
			Usage:       "Destination directory in PATH in which to create softlinks to tools",
			Value:       "/usr/local/bin/",
			Destination: &softlinkPath,
		},
		cli.BoolFlag{
			Name:        "update, upgrade",
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
		from, _ := filepath.EvalSymlinks(desktopFileToInstall)
		to, _ := filepath.EvalSymlinks(filepath.Join(binPath, desktopTo))

		log.Debugf("testing %s (%s) to %s", from, os.Args[0], to)

		if updateFlag || from == to {
			// If the user is running setup from an already installed desktop, assume update
			// TODO: if main.Version == today, maybe don't bother?
			log.Infof("Checking for newer version of desktop.")
			resp, err := http.Get("https://github.com/SvenDowideit/desktop/releases/latest")
			if err != nil {
				log.Infof("Error checking for latest version \n%s", err)
			} else {
				releaseUrl := resp.Request.URL.String()
				latestVersion = releaseUrl[strings.LastIndex(releaseUrl, "/")+1:]
				log.Debugf("this version == %s, latest version == %s", context.App.Version, latestVersion)

				thisVer := strings.Split(context.App.Version, ",")
				log.Debugf("this version == %s, latest version == %s", thisVer[0], latestVersion)
				thisDate, _ := time.Parse("2006-01-02", thisVer[0])
				latestDate, _ := time.Parse("2006-01-02", latestVersion)

				if !latestDate.After(thisDate) {
					// TODO: this assumes the other tools are up to date :(
					log.Infof("%s is already up to date", desktopTo)
					return nil
				} else {
					log.Infof("Downloading new version of desktop.")
					desktopFile := "desktop"
					if runtime.GOOS == "darwin" {
						desktopFile += "-osx"
					}
					if runtime.GOOS == "windows" {
						desktopFile += ".exe"
					}

					log.Infof("Downloading newer version of 'desktop': %s", latestVersion)
					dir, err := ioutil.TempDir("", "desktop")
					if err != nil {
						log.Fatal(err)
					}
					defer os.RemoveAll(dir) // clean up

					desktopFileToInstall := filepath.Join(dir, "desktop-download-" + latestVersion)
					log.Debugf("os.Arg[0]: %s ~~ desktopTo %s", desktopFileToInstall, desktopTo)
					if err := wget("https://github.com/SvenDowideit/desktop/releases/download/"+latestVersion+"/"+desktopFile, desktopFileToInstall); err != nil {
						return err
					}
					//on success, start the newly downloaded binary, and then exit.
					log.Infof("Running install using newly downloaded 'desktop'")
					return Run(desktopFileToInstall, "install")
				}
			}
		}
		// Can also install the just downloaded binary
		if err := install(desktopFileToInstall, "desktop-"+latestVersion, desktopTo); err != nil {
			return err
		}

		machineVer, err := installApp("docker-machine", "https://github.com/docker/machine/releases", "docker-machine-Darwin-x86_64")
		if err != nil {
			log.Error(err)
		}
		xhyveVer, err := installApp("docker-machine-driver-xhyve", "https://github.com/zchee/docker-machine-driver-xhyve/releases", "docker-machine-driver-xhyve")
		if err != nil {
			log.Error(err)
		}

		rancherVer, err := installApp("rancher", "https://github.com/rancher/cli/releases", "rancher-darwin-amd64-{{.Version}}.tar.gz")
		if err != nil {
			log.Error(err)
		}

		metaData := bugsnag.MetaData{}
		metaData.Add("app", "compiler", fmt.Sprintf("%s (%s)", runtime.Compiler, runtime.Version()))
		metaData.Add("app", "latestVersion", latestVersion)
		metaData.Add("app", "docker-machine", machineVer)
		metaData.Add("app", "docker-machine-driver-xhyve", xhyveVer)
		metaData.Add("app", "rancher-cli", rancherVer)
		metaData.Add("device", "os", runtime.GOOS)
		metaData.Add("device", "arch", runtime.GOARCH)
		cmd := exec.Command("uname", "-a")
		output, err := cmd.Output()
		if err != nil {
			return err
		}
		metaData.Add("device", "uname", string(output))
		bugsnag.Notify(fmt.Errorf("Successful installation"), metaData)

		return nil
	},
}

func installApp(app, url, ghFilenameTmpl string) (version string, err error) {
	latestVer, err := getLatestVersion(url + "/latest")
	if err != nil {
		return "", fmt.Errorf("Error getting latest version info from %s (%s)\n", url, err)
	}

	t, err := template.New("test").Parse(ghFilenameTmpl)
	if err != nil {
		return "", err
	}
	var doc bytes.Buffer
	err = t.Execute(&doc, map[string]interface{}{
		"Version": latestVer,
	})
	if err != nil {
		return "", err
	}
	ghFilename := doc.String()
	versionedApp := app + "-" + latestVer

	curVer := ""
	if _, err := exec.LookPath(app); err == nil {
		curVer, err = getCurrentVersion(app)
		if err != nil && err != exec.ErrNotFound {
			log.Debugf("Error getting version info for %s (%s)", app, err)
		}
		thisDate, _ := time.Parse("2006-01-02", curVer)
		latestDate, _ := time.Parse("2006-01-02", latestVer)

		if !latestDate.After(thisDate) {
			log.Debugf("%s is already up to date", app)
			return latestVer, nil
		}
	}
	log.Debugf("%s cur version == %s, latest version == %s", app, curVer, latestVer)

	log.Infof("Downloading new version of %s.", app)
	dir, err := ioutil.TempDir("", "desktop")
	if err != nil {
		log.Fatal(err)
	}
	defer os.RemoveAll(dir) // clean up

	downloadTo := filepath.Join(dir, app + "-" + latestVer)
	if err := wget(url+"/download/"+latestVer+"/"+ghFilename, downloadTo); err != nil {
		return latestVer, err
	}
	if strings.HasSuffix(ghFilename, "tar.gz") || strings.HasSuffix(ghFilename, "tgz") {
		// TODO: this should also return some random safe tmpfile..
		if err := processTGZ(downloadTo, app); err != nil {
			return latestVer, err
		}
		downloadTo = app
	}
	if err := install(downloadTo, versionedApp, app); err != nil {
		return latestVer, err
	}
	return latestVer, nil
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
	log.Infof("Installing %s pointing to %s in %s", filepath.Join(binPath, to), from, binPath)

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
	if err := sudoRun("mkdir", "-p", softlinkPath); err != nil {
		return err
	}
	if err := sudoRun("cp", from, filepath.Join(binPath, name)); err != nil {
		return err
	}
	if err := sudoRun("chmod", "0755", filepath.Join(binPath, name)); err != nil {
		return err
	}
	if err := sudoRun("rm", "-f", filepath.Join(softlinkPath, to)); err != nil {
		return err
	}
	if err := sudoRun("ln", "-s", filepath.Join(binPath, name), filepath.Join(softlinkPath, to)); err != nil {
		return err
	}
	if to == "docker-machine-driver-xhyve" {
		// xhyve needs root:wheel and setuid
		if err := sudoRun("chown", "root:wheel", binPath+"/"+to); err != nil {
			return err
		}
		if err := sudoRun("chmod", "u+s", binPath+"/"+to); err != nil {
			return err
		}
	}

	return nil
}

func sudoRun(cmds ...string) error {
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

func wget(from, to string) error {
	log.Debugf("Downloading %s into %s", from, to)
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
			log.Debugf("Found %s file", name)
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
			log.Debugf("%s : %c %s %s",
				"Yikes! Unable to figure out type",
				header.Typeflag,
				"in file",
				name,
			)
		}

		i++
	}
	return fmt.Errorf("Failed to find %s in %s", filename, srcFile)
}
