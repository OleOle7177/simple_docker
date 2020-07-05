# Simple docker

This project is just a fun attempt to create a docker analogue in one file: a tool to run isolated processes with restricted resources.

## Build
Use `make` to build project, download ubuntu 18.04 image (~58M) and untar it.

`make clean` to delete all artefacts.

## Run
`sudo ./simple_docker run --cpu 0.1 --memory 50000000 -- /bin/bash`

Learn more about parameters with `--help` flag:
`sudo ./simple_docker run --help`

## Limitations
- Can be launched only on linux (as real docker). Tested on Ubuntu 18.04.
- Can be used only as root user because of cgroups v1.
- Some namespaces are not present.

## Short Description

Original Docker based on 2 linux kernel features: namespaces (for process isolation) and cgroups (for resource restrictions).

#### Namespaces
There are 6 namespaces, they are created with syscalls. So the idea is to create namespaces and launch a process inside them. Namespaces:
- Unix Timesharing System (namespace has its own hostname),
- process IDs,
- mounts,
- network (namespace has its own network interfaces),
- user IDs,
- IPC (processes inside namespace can interact only with processes inside that namespace).

Network, user IDs and IPC are not implemented here yet.

#### Cgroups
Cgroups is a linux kernel feature to group processes and assign properties to that groups. Resource limitation is just a one particular case of such properties.
Here we use cpu and memory cgroups of v1 (https://www.kernel.org/doc/html/latest/admin-guide/cgroup-v1/cgroups.html).

To read more on cgroups v2 features and motivations: https://www.kernel.org/doc/html/latest/admin-guide/cgroup-v2.html.

Why v1 here? Because currently it's a default cgroups version to use in ubuntu systems.
