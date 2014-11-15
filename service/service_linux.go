package service
import "syscall"
import "fmt"
import "github.com/hlandau/degoutils/daemon"

func (h *ihandler) dropPrivilegesExtra() error {
	if !h.info.NoBanSuid {
		err := h.banSuid()
		if err != nil {
			return err
		}
	}

	return nil
}

func (h *ihandler) banSuid() error {
	err := prctl(PR_SET_NO_NEW_PRIVS, 1, 0, 0, 0)
	if err != nil {
		return fmt.Errorf("cannot set NO_NEW_PRIVS: %v", err)
	}

	if daemon.IsRoot() {
		err = prctl(PR_SET_SECUREBITS,
			SECBIT_NOROOT|SECBIT_NOROOT_LOCKED|SECBIT_KEEP_CAPS_LOCKED, 0, 0, 0)
		if err != nil {
			return fmt.Errorf("cannot set SECUREBITS: %v", err)
		}
	}

	return nil
}

const (
	PR_SET_SECCOMP       = 22
	PR_SET_SECUREBITS    = 28
	PR_SET_NO_NEW_PRIVS  = 36

	SECBIT_NOROOT                 = 1<<0
	SECBIT_NOROOT_LOCKED          = 1<<1
	SECBIT_NO_SETUID_FIXUP        = 1<<2
	SECBIT_NO_SETUID_FIXUP_LOCKED = 1<<3
	SECBIT_KEEP_CAPS              = 1<<4
	SECBIT_KEEP_CAPS_LOCKED       = 1<<5
)

func prctl(opt int, arg2, arg3, arg4, arg5 uint64) error {
	_, _, e1 := syscall.Syscall6(syscall.SYS_PRCTL, uintptr(opt),
		uintptr(arg2), uintptr(arg3), uintptr(arg4), uintptr(arg5), 0)
	if e1 != 0 {
		return e1
	}

	return nil
}
