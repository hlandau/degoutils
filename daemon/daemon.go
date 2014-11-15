// Functions to assist with the writing of UNIX-style daemons in go.
package daemon

import "syscall"
import "net"
import "os"
import "errors"

// Initialises a daemon with recommended values.
//
// Currently, this only calls umask(0).
func Init() error {
	syscall.Umask(0)
	return nil
}

// Daemonizes but doesn't fork.
//
// The stdin, stdout and stderr fds are remapped to /dev/null.
// setsid is called.
//
// The process changes its current directory to /.
//
// If you intend to call DropPrivileges, call it after calling this function,
// as /dev/null will no longer be available after privileges are dropped.
func Daemonize() error {
	//   null_fd = open("/dev/null", O_WRONLY);
	null_f, err := os.OpenFile("/dev/null", os.O_RDWR, 0)
	if err != nil {
		return err
	}

	//   close(0);
	stdin_fd := int(os.Stdin.Fd())
	stdout_fd := int(os.Stdout.Fd())
	stderr_fd := int(os.Stderr.Fd())
	err = syscall.Close(stdin_fd)
	if err != nil {
		return err
	}

	err = syscall.Close(stdout_fd)
	if err != nil {
		return err
	}

	err = syscall.Close(stderr_fd)
	if err != nil {
		return err
	}

	//   close(1);
	//   close(2);
	//   ... reopen fds 0, 1, 2 as /dev/null ...
	null_fd := int(null_f.Fd())
	syscall.Dup2(null_fd, stdin_fd)
	syscall.Dup2(null_fd, stdout_fd)
	syscall.Dup2(null_fd, stderr_fd)

	err = syscall.Close(null_fd)
	if err != nil {
		return err
	}

	// This may fail if we're not root
	syscall.Setsid()

	//
	err = syscall.Chdir("/")
	if err != nil {
		return err
	}

	return nil
}

func IsRoot() bool {
	return syscall.Getuid() == 0 || syscall.Geteuid() == 0 || syscall.Getgid() == 0 || syscall.Getegid() == 0
}

// Drops privileges to the specified UID and GID.
// This function does nothing and returns no error if all E?[UG]IDs are nonzero.
//
// If chrootDir is not empty, the process is chrooted into it. The directory
// must exist. The function tests that privilege dropping has been successful
// by attempting to setuid(0), which must fail.
//
// The current directory is set to / inside the chroot.
//
// The function ensures that /etc/hosts and /etc/resolv.conf are loaded before
// chrooting, so name service should continue to be available.
func DropPrivileges(UID, GID int, chrootDir string) error {
	if !IsRoot() {
		return nil
	}

	if UID == 0 {
		return errors.New("Can't drop privileges to UID 0 - did you set the UID properly?")
	}

	if GID == 0 {
		return errors.New("Can't drop privileges to GID 0 - did you set the GID properly?")
	}

	if chrootDir == "/" {
		chrootDir = ""
	}

	if chrootDir != "" {
		c, err := net.Dial("udp", "un_localhost:1")
		if err != nil {
			//
		} else {
			c.Close()
		}

		err = syscall.Chroot(chrootDir)
		if err != nil {
			return err
		}
	}

	err := syscall.Chdir("/")
	if err != nil {
		return err
	}

	err = syscall.Setgroups([]int{GID})
	if err != nil {
		return err
	}

	err = syscall.Setresgid(GID, GID, GID)
	if err != nil {
		return err
	}

	err = syscall.Setresuid(UID, UID, UID)
	if err != nil {
		return err
	}

	err = syscall.Setuid(0)
	if err == nil {
		return errors.New("Can't drop privileges - setuid(0) still succeeded")
	}

	return nil
}

var EmptyChrootPath = "/var/empty"
