package main

import (
	"fmt"
	"io/ioutil"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"

	"github.com/google/uuid"
	"github.com/spf13/cobra"
	"golang.org/x/sys/unix"
)

const (
	cgroupCPUPath = "/sys/fs/cgroup/cpu/simple_docker"
	cgroupMemPath = "/sys/fs/cgroup/memory/simple_docker"
)

// ContainerParams represents params for container to launch
type ContainerParams struct {
	ID          string
	CPUQuota    float32
	CPUPeriodUs int
	MemoryBytes int
	Args        []string
}

func main() {
	var (
		memory int
		cpu    float32
		guid   string
	)

	cmdRun := &cobra.Command{
		Use:   "run",
		Short: "Run launches command as a new isolated process",
		Args:  cobra.MinimumNArgs(1),
		Run: func(cmd *cobra.Command, args []string) {
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
				ID:          guid,
				CPUQuota:    cpu,
				CPUPeriodUs: 100000,
				MemoryBytes: memory,
				Args:        args,
			})
		},
	}

	rootCmd := &cobra.Command{}
	rootCmd.PersistentFlags().IntVar(&memory, "memory", 40000000, "limit process memory")
	rootCmd.PersistentFlags().Float32Var(&cpu, "cpu", 0.5, "limit process cpu cores: max % usage (cpu cores quota in 100ms)")
	cmdChild.PersistentFlags().StringVar(&guid, "id", "", "container id")

	rootCmd.AddCommand(cmdRun, cmdChild)
	rootCmd.Execute()
}

// Create new namespaces and run child() command in it
func run() {
	// /proc/self/exe - is a self process
	guid := strings.ReplaceAll(uuid.New().String(), "-", "")
	args := []string{"child", "--id", guid}

	cmd := exec.Command("/proc/self/exe", append(args, os.Args[2:]...)...)
	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	// copy namespaces here
	cmd.SysProcAttr = &syscall.SysProcAttr{
		Cloneflags: syscall.CLONE_NEWUTS | syscall.CLONE_NEWPID | syscall.CLONE_NEWNS,
	}

	err := cmd.Run()
	if err != nil {
		panic(err)
	}

	defer cleanup(guid)
}

// Сreate all namespaces, launch a command inside namespace

// Run given command inside process
func child(params ContainerParams) {
	cmd := exec.Command(params.Args[0], params.Args[1:]...)
	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	err := cg(params.ID, params.MemoryBytes, params.CPUPeriodUs, params.CPUQuota)
	if err != nil {
		panic(err)
	}

	// set hostname
	err = unix.Sethostname([]byte(params.ID))
	if err != nil {
		panic(err)
	}

	// TODO: cant copy files to that directory
	containerRoot := filepath.Join("./containers", params.ID)
	err = os.Mkdir(containerRoot, 4755)
	if err != nil {
		panic(err)
	}

	copyCmd := exec.Command("rsync", "-raAXv", "--links", "./images/ubuntu_18_04/rootfs/", containerRoot)

	out, err := copyCmd.CombinedOutput()
	fmt.Println(string(out[:]))

	ls := exec.Command("ls -laF")
	ls.Run()
	// set root directory and limit access to directory tree
	err = unix.Chroot(containerRoot)
	if err != nil {
		panic(err)
	}
	os.Chdir("/")
	fmt.Println("we are here")

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
func cg(id string, memory int, cpuPeriodUs int, cpuQuota float32) error {
	pid := strconv.Itoa(os.Getpid())

	err := cgroupMem(id, pid, memory)
	if err != nil {
		return err
	}

	err = cgroupCPU(id, pid, cpuPeriodUs, cpuQuota)
	if err != nil {
		return err
	}

	return nil
}

func cgroupMem(id, pid string, memory int) error {
	mem := filepath.Join(cgroupMemPath, id)
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
func cgroupCPU(id, pid string, periodUs int, quota float32) error {
	cpu := filepath.Join(cgroupCPUPath, id)
	err := os.MkdirAll(cpu, 0755)
	if err != nil {
		return err
	}

	// how often cpu will return to this task, microseconds
	err = ioutil.WriteFile(filepath.Join(cpu, "cpu.cfs_period_us"), []byte(strconv.Itoa(periodUs)), 0700)
	if err != nil {
		return err
	}

	// how much time cpu will continue to work on this task, microseconds
	q := quota * float32(periodUs)
	err = ioutil.WriteFile(filepath.Join(cpu, "cpu.cfs_quota_us"), []byte(strconv.Itoa(int(q))), 0700)
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

func cleanup(id string) {
	cleanupCg(id)

	err := cleanIfEmpty(cgroupCPUPath)
	if err != nil {
		fmt.Println(err)
	}

	err = cleanIfEmpty(cgroupMemPath)
	if err != nil {
		fmt.Println(err)
	}
}

func cleanupCg(id string) {
	err := os.RemoveAll(filepath.Join(cgroupMemPath, id))
	if err != nil {
		fmt.Println(err)
	}

	err = os.RemoveAll(filepath.Join(cgroupCPUPath, id))
	if err != nil {
		fmt.Println(err)
	}
}

func cleanIfEmpty(path string) error {
	files, err := ioutil.ReadDir(path)
	if err != nil {
		return err
	}

	for _, f := range files {
		if f.IsDir() {
			fmt.Println(f.Name())
			return nil
		}
	}

	err = os.RemoveAll(path)
	if err != nil {
		return err
	}

	return nil
}
