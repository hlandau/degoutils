package service

import "github.com/hlandau/degoutils/passwd"
import "github.com/hlandau/degoutils/daemon"
import "github.com/hlandau/degoutils/service/sdnotify"
import "github.com/ErikDubbelboer/gspt"
import "fmt"
import "strconv"

func systemdUpdateStatus(status string) error {
	return sdnotify.SdNotify(status)
}

func setproctitle(status string) error {
	gspt.SetProcTitle(status)
	return nil
}

func (info *Info) registerFlags() error {
	info.uidFlag = info.fs.String("uid", "", "UID to run as (default: don't drop privileges)")
	info.gidFlag = info.fs.String("gid", "", "GID to run as (default: don't drop privileges)")
	info.daemonizeFlag = info.fs.Bool("daemon", false, "Run as daemon? (doesn't fork)")
	info.chrootFlag = info.fs.String("chroot", "", "Chroot to a directory (must set UID, GID) (\"/\" disables)")
	info.pidfileFlag = info.fs.String("pidfile", "", "Write PID to file with given filename and hold a write lock")
	return nil
}

func (info *Info) serviceMain() error {
	err := daemon.Init()
	if err != nil {
		return err
	}

	err = systemdUpdateStatus("\n")
	if err == nil {
		info.systemd = true
	}

	if *info.pidfileFlag != "" {
		info.pidFileName = *info.pidfileFlag

		err = info.openPIDFile()
		if err != nil {
			return err
		}

		defer info.closePIDFile()
	}

	return info.runInteractively()
}

func (info *Info) openPIDFile() error {
	return daemon.OpenPIDFile(info.pidFileName)
}

func (info *Info) closePIDFile() {
	daemon.ClosePIDFile()
}

func (h *ihandler) DropPrivileges() error {
	if h.dropped {
		return nil
	}

	if *h.info.daemonizeFlag || h.info.systemd {
		err := daemon.Daemonize()
		if err != nil {
			return err
		}
	}

	if *h.info.uidFlag != "" && *h.info.gidFlag == "" {
		gid, err := passwd.GetGIDForUID(*h.info.uidFlag)
		if err != nil {
			return err
		}
		*h.info.gidFlag = strconv.FormatInt(int64(gid), 10)
	}

	if h.info.DefaultChroot == "" {
		h.info.DefaultChroot = "/"
	}

	chrootPath := *h.info.chrootFlag
	if chrootPath == "" {
		chrootPath = h.info.DefaultChroot
	}

	err := h.dropPrivilegesExtra()
	if err != nil {
		return err
	}

	if *h.info.uidFlag != "" {
		uid, err := passwd.ParseUID(*h.info.uidFlag)
		if err != nil {
			return err
		}
		gid, err := passwd.ParseGID(*h.info.gidFlag)
		if err != nil {
			return err
		}

		err = daemon.DropPrivileges(uid, gid, chrootPath)
		if err != nil {
			return err
		}
	} else if *h.info.chrootFlag != "" && *h.info.chrootFlag != "/" {
		return fmt.Errorf("Must set UID and GID to use chroot")
	}

	if !h.info.AllowRoot && daemon.IsRoot() {
		return fmt.Errorf("Daemon must not run as root")
	}

	h.dropped = true
	return nil
}
