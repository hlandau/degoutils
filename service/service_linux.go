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
	err := prctl(pPR_SET_NO_NEW_PRIVS, 1, 0, 0, 0)
	if err != nil {
		return fmt.Errorf("cannot set NO_NEW_PRIVS: %v", err)
	}

	if daemon.IsRoot() {
		err = prctl(pPR_SET_SECUREBITS,
			sSECBIT_NOROOT|sSECBIT_NOROOT_LOCKED|sSECBIT_KEEP_CAPS_LOCKED, 0, 0, 0)
		if err != nil {
			return fmt.Errorf("cannot set SECUREBITS: %v", err)
		}
	}

	return nil
}

const (
	pPR_SET_SECCOMP      = 22
	pPR_SET_SECUREBITS   = 28
	pPR_SET_NO_NEW_PRIVS = 36

	sSECBIT_NOROOT                 = 1 << 0
	sSECBIT_NOROOT_LOCKED          = 1 << 1
	sSECBIT_NO_SETUID_FIXUP        = 1 << 2
	sSECBIT_NO_SETUID_FIXUP_LOCKED = 1 << 3
	sSECBIT_KEEP_CAPS              = 1 << 4
	sSECBIT_KEEP_CAPS_LOCKED       = 1 << 5
)

func prctl(opt int, arg2, arg3, arg4, arg5 uint64) error {
	_, _, e1 := syscall.Syscall6(syscall.SYS_PRCTL, uintptr(opt),
		uintptr(arg2), uintptr(arg3), uintptr(arg4), uintptr(arg5), 0)
	if e1 != 0 {
		return e1
	}

	return nil
}
