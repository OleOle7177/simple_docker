package main

import (
	"fmt"
	"io/ioutil"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"syscall"

	"github.com/spf13/cobra"
	"golang.org/x/sys/unix"
)

type ContainerParams struct {
	CPU    float32
	Memory int
	Args   []string
}

func main() {
	var (
		memory  int
		cpu     float32
		command string
	)

	cmdRun := &cobra.Command{
		Use:   "run",
		Short: "Run launches command as a new isolated process",
		Args:  cobra.MinimumNArgs(1),
		Run: func(cmd *cobra.Command, args []string) {
			fmt.Println(args)
			run()
		},
	}

	cmdChild := &cobra.Command{
		Use:    "child",
		Short:  "",
		Hidden: true,
		Args:   cobra.MinimumNArgs(1),
		Run: func(cmd *cobra.Command, args []string) {
			child(ContainerParams{
				CPU:    cpu,
				Memory: memory,
				Args:   args,
			})
		},
	}

	rootCmd := &cobra.Command{}

	rootCmd.PersistentFlags().StringVarP(&command, "command", "c", "", "command to run in container")
	rootCmd.PersistentFlags().IntVar(&memory, "memory", 2049000, "limit process memory")
	rootCmd.PersistentFlags().Float32Var(&cpu, "cpu", 0.5, "limit process cpu cores: max % usage (cpu cores quota in 100ms)")
	rootCmd.AddCommand(cmdRun, cmdChild)
	rootCmd.Execute()
}

// Create new namespaces and run child() command in it
func run() {
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
	defer cleanupMem()
	defer cleanupCPU()

	if err != nil {
		panic(err)
	}
}

// Сreate all namespaces, launch a command inside namespace

// Run given command inside process
func child(params ContainerParams) {
	cmd := exec.Command(params.Args[0], params.Args[1:]...)
	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	err := cg(params.Memory, params.CPU)
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
func cg(memory int, cpu float32) error {
	pid := strconv.Itoa(os.Getpid())

	err := cgroupMem(pid, memory)
	if err != nil {
		return err
	}

	err = cgroupCPU(pid, cpu)
	if err != nil {
		return err
	}

	return nil
}

func cgroupMem(pid string, memory int) error {
	mem := cgroupMemName()
	err := os.MkdirAll(mem, 0755)
	if err != nil {
		return err
	}

	memLimit := strconv.Itoa(memory)

	// memory limits
	err = ioutil.WriteFile(filepath.Join(mem, "memory.limit_in_bytes"), []byte(memLimit), 0700)
	if err != nil {
		return err
	}

	// memory + swap limits
	err = ioutil.WriteFile(filepath.Join(mem, "memory.memsw.limit_in_bytes"), []byte(memLimit), 0700)
	if err != nil {
		return err
	}

	// call cgroup release_agent action when all process leaved the group
	err = ioutil.WriteFile(filepath.Join(mem, "notify_on_release"), []byte("1"), 0700)
	if err != nil {
		return err
	}

	err = ioutil.WriteFile(filepath.Join(mem, "cgroup.procs"), []byte(pid), 0700)
	if err != nil {
		return err
	}

	return nil
}

// https://access.redhat.com/documentation/en-us/red_hat_enterprise_linux/6/html/resource_management_guide/sec-cpu
func cgroupCPU(pid string, quota float32) error {
	cpu := cgroupCPUName()
	err := os.MkdirAll(cpu, 0755)
	if err != nil {
		return err
	}

	// how often cpu will return to this task, microseconds
	err = ioutil.WriteFile(filepath.Join(cpu, "cpu.cfs_period_us"), []byte("100000"), 0700)
	if err != nil {
		return err
	}

	// how much time cpu will continue to work on this task, microseconds
	err = ioutil.WriteFile(filepath.Join(cpu, "cpu.cfs_quota_us"), []byte("200000"), 0700)
	if err != nil {
		return err
	}

	// call cgroup release_agent action when all process leaved the group
	err = ioutil.WriteFile(filepath.Join(cpu, "notify_on_release"), []byte("1"), 0700)
	if err != nil {
		return err
	}

	err = ioutil.WriteFile(filepath.Join(cpu, "cgroup.procs"), []byte(pid), 0700)
	if err != nil {
		return err
	}

	return nil
}

func cleanupMem() {
	err := os.RemoveAll(cgroupMemName())
	if err != nil {
		panic(err)
	}
}

func cleanupCPU() {
	err := os.RemoveAll(cgroupCPUName())
	if err != nil {
		panic(err)
	}
}

func cgroupMemName() string {
	cgroups := "/sys/fs/cgroup"
	return filepath.Join(cgroups, "memory", "simple_docker")
}

func cgroupCPUName() string {
	cgroups := "/sys/fs/cgroup"
	return filepath.Join(cgroups, "cpu", "simple_docker")
}
