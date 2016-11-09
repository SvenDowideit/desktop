# Rancher for the Desktop

This is a single go app which will install all the pieces needed to give you access to a Rancher Server
from your desktop computer.

This initial release will download and install `docker-machine` and the `xhyve` driver, and then start
and `xhyve` based virtual machine running RancherOS. It will then start a RancherServer service, and
add an agent to that vm.

Once complete, you can use all the normal `rancher` cli commands.

Get started by running the following:

```
$ ./desktop install
...
$ desktop start
... 15 minutes of processing on my slow internet connection ...
$ rancher hosts
ID        HOSTNAME   STATE     IP           DETAIL
1h1       rancher    active    172.17.0.1 
$ docker-machine ls
NAME         ACTIVE   DRIVER       STATE     URL                        SWARM   DOCKER    ERRORS
rancher      -        xhyve        Running   tcp://192.168.64.18:2376           v1.12.1   

```

