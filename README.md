`sudo go run main.go run /bin/bash`

TODO:
v - finish o'reilly course
- create ubuntufs image and pack it to tgz, download and unpack on launch
+- - generate random containers uuid on start, create separate folder for each container
- makefile to build binary
- all namespaces:
  v - UTS (hostname),
  v - PID,
  mounts,
  network (only container network interfaces),
  user ids (separate container users and groups),
  IPC (container could interact only with proccesses inside container)
v - cgroup for cpu (https://selectel.ru/blog/mexanizmy-kontejnerizacii-cgroups/)
  - check existing images on start, download if not exists, unpack to separate folder

namespace: limits what container can see
cgroups: limits resources

write in README that based on cgroup v2, so kernel should be >

# mount -t cgroup2 none $MOUNT_POINT

https://simpledocker.s3.eu-central-1.amazonaws.com/ubuntu_18_04.tar.gz
