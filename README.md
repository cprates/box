# box
*box* is a low-level Linux container runtime, the result of my journey in learning how containers work, and is not
meant to be used in production or replace other solutions like *runC*, *LXC*, etc.

The spec file (config.json) is based on the OCI spec, which means that you can easily convert existing specs - unsupported configs are ignored.

*box* is also the container runtime powering an AWS Lambda mock in my other project [LWS](https://github.com/cprates/lws).


## Running a box (container)

If you have *Docker* installed you are good to go, the `netconf.json` example file is ready to use 
*Docker's* bridge interface. If not, you'll need to create a bridge interface and update
`netconf.json` example file with your bridge name and addressing.

First clone and build the project (you'll need golang installed)

```bash
make build
```

Get a file system to run the *box* on. The easiest way to get one is using *docker* or 
*debootstrap*.
The following line stores the Linux Alpine FS in `fs/` folder using docker:

```bash
mkdir -p fs && docker export $(docker create alpine) | tar -C fs -xvf -
```

Then point `root.path` in `config.json` template file to your newly created FS folder 
(*absolute path*)

Finally run your box (need root). You should get a new prompt `/ #`:

```bash
sudo ./box run mybox
```

At this point you are inside your box. Have fun!
```bash
ps aux
ifconfig -a
ping 8.8.8.8
```

*box* also allows you to create boxes and run them later. To test this, first update `process.args` in `config.json` to something like `"/bin/ps", "aux"`.
Running applications that need to read from `stdin` won't work in this mode.

First create a box that will be waiting to be started
```bash
sudo ./box create mybox
```

Start it when you want:
```bash
sudo ./box start mybox
```

And finally destroy it
```bash
sudo ./box destroy mybox
```


## Configs
Unless specified by passing flags `--spec` and `--netconf`, by default *box* loads the spec and network config from `config.json` and `netconf.json` respectively.

Both `config.json` and `netconf.json` in this repo contain all supported configs.


## Runtime Actions
 
 |     Action     |  Supported  |                         Description                                |
 | -------------- | ----------- | ----------------------------------------------------- |
 | Get processes  |     No      | Return all the pids for processes running inside a container       | 
 | Get Stats      |     No      | Return resource statistics for the container as a whole            |
 | Wait           |     No      | Waits on the container's init process ( pid 1 )                    |
 | Wait Process   |     No      | Wait on any of the container's processes returning the exit status | 
 | Destroy        |     Yes     | Kill the container's init process and remove any filesystem state  |
 | Signal         |     No      | Send a signal to the container's init process                      |
 | Signal Process |     No      | Send a signal to any of the container's processes                  |
 | Pause          |     No      | Pause all processes inside the container                           |
 | Resume         |     No      | Resume all processes inside the container if paused                |
 | Exec           |     Yes     | Execute a new process inside of the container  ( requires setns )  |
 | Set            |     No      | Setup configs of the container after it's created                  |


## Mount points
At its current state, *box* ignores the mount points in the spec file and configures a static list of 
mount points:
* /proc
* /tmp
* /dev
* /sys
* /dev/mqueue
* /dev/pts
* /dev/shm

## Device nodes
Same as for mount points. A static list of device nodes is configures for every box. Note that `console` is not setup:
* /dev/null
* /dev/zero
* /dev/full
* /dev/random
* /dev/urandom
* /dev/tty
* /dev/ptmx


## Namespaces
Namespaces list from the spec file are also ignored. A static list is configured instead:
* IPC
* Network
* Mount
* PID
* UTS

## Cgroups
TODO
