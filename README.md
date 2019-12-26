TODO

Example spec file:
```
{
  "ociVersion": "1.0.1",

  "root": {
    "path": "/home/cpr3t4s/Workspace/lws/repo/fs"
  },

  "hostname": "box1hostname",

  "process": {
    "terminal": false,
    "args": [
      "/bin/ps", "aux"
    ],
    "env": [
      "PATH=/usr/local/sbin:/usr/local/bin:/usr/sbin:/usr/bin:/sbin:/bin",
      "TERM=xterm"
    ],
    "cwd": "/"
  }
}
```