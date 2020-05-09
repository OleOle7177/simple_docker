package main

import (
	"fmt"
	"io/ioutil"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"syscall"

	"golang.org/x/sys/unix"
)

func main() {
	switch os.Args[1] {
	case "run":
		run()
	case "child":
		child()
	default:
		panic("help")
	}
}

func run() {
	fmt.Printf("Running: %v\n", os.Args[2:])

	// /proc/self/exe - is a self process
	cmd := exec.Command("/proc/self/exe", append([]string{"child"}, os.Args[2:]...)...)

	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	// copy namespaces here
	cmd.SysProcAttr = &syscall.SysProcAttr{
		Cloneflags: syscall.CLONE_NEWUTS | syscall.CLONE_NEWPID | syscall.CLONE_NEWNS,
	}

	err := cmd.Run()
	defer cleanup()

	if err != nil {
		panic(err)
	}
}

func child() {
	fmt.Printf("Running: %v\n", os.Args[2:])

	cmd := exec.Command(os.Args[2], os.Args[3:]...)

	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	err := cg()
	if err != nil {
		panic(err)
	}

	// set hostname
	err = unix.Sethostname([]byte("container"))
	if err != nil {
		panic(err)
	}

	// set root directory and limit access to directory tree
	err = unix.Chroot("./ubuntufs")
	if err != nil {
		panic(err)
	}
	os.Chdir("/")

	// mount proc namespace
	err = unix.Mount("proc", "/proc", "proc", 0, "")
	if err != nil {
		panic(err)
	}

	defer func() {
		err = unix.Unmount("proc", 0)
		if err != nil {
			panic(err)
		}
	}()

	// mount tmpfs for inmemory container files
	err = os.MkdirAll("./tmpfs_container", 0777)
	if err != nil {
		panic(err)
	}

	err = unix.Mount("tmpfs_container", "tmpfs_container", "tmpfs", 0, "")
	if err != nil {
		panic(err)
	}

	defer func() {
		err = unix.Unmount("tmpfs_container", 0)
		if err != nil {
			panic(err)
		}
	}()

	// run command
	err = cmd.Run()
	if err != nil {
		panic(err)
	}
}

// https://www.kernel.org/doc/html/latest/admin-guide/cgroup-v1/cgroups.html
func cg() error {
	mem := cgroupMemName()
	err := os.MkdirAll(mem, 0755)
	if err != nil {
		return err
	}

	// memory limits
	err = ioutil.WriteFile(filepath.Join(mem, "memory.limit_in_bytes"), []byte("2049000"), 0700)
	if err != nil {
		return err
	}

	// memory + swap limits
	err = ioutil.WriteFile(filepath.Join(mem, "memory.memsw.limit_in_bytes"), []byte("2049000"), 0700)
	if err != nil {
		return err
	}

	// call cgroup release_agent action when all process leaved the group
	err = ioutil.WriteFile(filepath.Join(mem, "notify_on_release"), []byte("1"), 0700)
	if err != nil {
		return err
	}

	pid := strconv.Itoa(os.Getpid())
	err = ioutil.WriteFile(filepath.Join(mem, "cgroup.procs"), []byte(pid), 0700)
	if err != nil {
		return err
	}

	return nil
}

func cleanup() {
	err := os.RemoveAll(cgroupMemName())
	if err != nil {
		panic(err)
	}
}

func cgroupMemName() string {
	cgroups := "/sys/fs/cgroup"
	return filepath.Join(cgroups, "memory", "simple_docker")
}
