TODO

### Mount points
* /proc
* /tmp
* /dev
* /sys
* /dev/mqueue
* /dev/pts
* /dev/shm

### Device nodes
* /dev/null
* /dev/zero
* /dev/full
* /dev/random
* /dev/urandom
* /dev/tty
* /dev/ptmx

Example spec file:
```
{
  "ociVersion": "1.0.1",

  "root": {
    "path": "/path/to/root/fs"
  },

  "hostname": "boxhostname",

  "process": {
    "args": [
      "/bin/ps", "aux"
    ],
    "env": [
      "PATH=/usr/local/sbin:/usr/local/bin:/usr/sbin:/usr/bin:/sbin:/bin",
      "TERM=xterm",
      "HOME=/root"
    ],
    "cwd": "/"
  }
}
```