package daemon
import "syscall"
import "net"
import "os"
import "errors"

func Daemonize() error {
  //   null_fd = open("/dev/null", O_WRONLY);
  null_f, err := os.OpenFile("/dev/null", os.O_RDWR, 0)
  if err != nil {
    return err
  }

  //   close(0);
  stdin_fd  := int(os.Stdin.Fd())
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

func DropPrivileges(UID, GID int) error {
  if syscall.Getuid() != 0 && syscall.Geteuid() != 0 && syscall.Getgid() != 0 && syscall.Getegid() != 0 {
    return nil
  }

  if UID == 0 {
    return errors.New("Can't drop privileges to UID 0 - did you set the UID properly?")
  }

  if GID == 0 {
    return errors.New("Can't drop privileges to GID 0 - did you set the GID properly?")
  }

  c, err := net.Dial("udp", "un_localhost:1")
  if err != nil {
    //
  } else {
    c.Close()
  }

  syscall.Chroot("/var/empty")

  err = syscall.Chdir("/")
  if err != nil {
    return err
  }

  err = syscall.Setgroups([]int { GID })
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
