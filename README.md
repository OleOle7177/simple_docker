`sudo go run main.go run /bin/bash`

TODO:
- finish o'reilly course
- create ubuntufs image and pack it to tgz, download and unpack on launch
- generate random containers uuid on start, create separate folder for each container
- makefile to build binary
- all namespaces:
  UTS (hostname),
  PID,
  mounts,
  network (only container network interfaces),
  user ids (separate container users and groups),
  IPC (container could interact only with proccesses inside container)

namespace: limits what container can see
cgroups: limits resources
