package config

import (
	"os"
	"runtime"
)

var LogDir = "/usr/local/share/rancher/logs/"
var RancherBinDir = "/usr/local/share/rancher/bin/"
var GlobalBinDir = "/usr/local/bin/"

// TODO: extract the "get latest version number URL into the struct too"
// docker has a file at https://get.docker.com/latest
type InstallFile struct {
	Command, UrlPath, UrlFile string
}

// TODO: can get the latest version number of docker client from https://get.docker.com/latest
// TODO: extract this to cfg files
var InstallCfg = map[string][]InstallFile{
	"darwin": []InstallFile{
		InstallFile{"docker", "https://get.docker.com/builds/Darwin/x86_64/", "docker-1.12.3.tgz"},
		InstallFile{"docker-machine", "https://github.com/docker/machine/releases", "docker-machine-Darwin-x86_64"},
		InstallFile{"docker-machine-driver-xhyve", "https://github.com/zchee/docker-machine-driver-xhyve/releases", "docker-machine-driver-xhyve"},
		InstallFile{"rancher", "https://github.com/rancher/cli/releases", "rancher-darwin-amd64-{{.Version}}.tar.gz"},
	},
	"windows": []InstallFile{
		InstallFile{"docker.exe", "https://get.docker.com/builds/Windows/x86_64/", "docker-1.12.3.zip"},
		InstallFile{"docker-machine", "https://github.com/docker/machine/releases", "docker-machine-Windows-x86_64.exe"},
		InstallFile{"docker-machine-driver-vmware", "https://github.com/pecigonzalo/docker-machine-vmwareworkstation/releases", "docker-machine-driver-vmwareworkstation.exe"},
		InstallFile{"rancher.exe", "https://github.com/rancher/cli/releases", "rancher-windows-amd64-{{.Version}}.zip"},
	},
	"linux64": []InstallFile{
		InstallFile{"docker", "https://get.docker.com/builds/Linux/x86_64/", "docker-1.12.3.tgz"},
		InstallFile{},
		InstallFile{},
		InstallFile{},
	},
}

func init() {
	if runtime.GOOS == "windows" {
		LogDir = os.ExpandEnv("${ALLUSERSPROFILE}/rancher/logs/")
		RancherBinDir = os.ExpandEnv("${ALLUSERSPROFILE}/rancher/bin/")
		GlobalBinDir = os.ExpandEnv("${USERPROFILE}/bin/")
	}
}
