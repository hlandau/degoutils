package service

import "github.com/hlandau/degoutils/passwd"
import "github.com/hlandau/degoutils/daemon"
import "github.com/hlandau/degoutils/service/sdnotify"
import "github.com/ErikDubbelboer/gspt"
import "fmt"
import "flag"
import "strconv"

var uidFlag = fs.String("uid", "", "UID to run as (default: don't drop privileges)")
var _uidFlag = flag.String("uid", "", "UID to run as (default: don't drop privileges)")
var gidFlag = fs.String("gid", "", "GID to run as (default: don't drop privileges)")
var _gidFlag = flag.String("gid", "", "GID to run as (default: don't drop privileges)")
var daemonizeFlag = fs.Bool("daemon", false, "Run as daemon? (doesn't fork)")
var _daemonizeFlag = flag.Bool("daemon", false, "Run as daemon? (doesn't fork)")
var chrootFlag = fs.String("chroot", "", "Chroot to a directory (must set UID, GID) (\"/\" disables)")
var _chrootFlag = flag.String("chroot", "", "Chroot to a directory (must set UID, GID) (\"/\" disables)")
var pidfileFlag = fs.String("pidfile", "", "Write PID to file with given filename and hold a write lock")
var _pidfileFlag = flag.String("pidfile", "", "Write PID to file with given filename and hold a write lock")

func systemdUpdateStatus(status string) error {
	return sdnotify.SdNotify(status)
}

func setproctitle(status string) error {
	gspt.SetProcTitle(status)
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

	if *pidfileFlag != "" {
		info.pidFileName = *pidfileFlag

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

	if *daemonizeFlag || h.info.systemd {
		err := daemon.Daemonize()
		if err != nil {
			return err
		}
	}

	if *uidFlag != "" && *gidFlag == "" {
		gid, err := passwd.GetGIDForUID(*uidFlag)
		if err != nil {
			return err
		}
		*gidFlag = strconv.FormatInt(int64(gid), 10)
	}

	if h.info.DefaultChroot == "" {
		h.info.DefaultChroot = "/"
	}

	chrootPath := *chrootFlag
	if chrootPath == "" {
		chrootPath = h.info.DefaultChroot
	}

	err := h.dropPrivilegesExtra()
	if err != nil {
		return err
	}

	if *uidFlag != "" {
		uid, err := passwd.ParseUID(*uidFlag)
		if err != nil {
			return err
		}
		gid, err := passwd.ParseGID(*gidFlag)
		if err != nil {
			return err
		}

		err = daemon.DropPrivileges(uid, gid, chrootPath)
		if err != nil {
			return err
		}
	} else if *chrootFlag != "" && *chrootFlag != "/" {
		return fmt.Errorf("Must set UID and GID to use chroot")
	}

	if !h.info.AllowRoot && daemon.IsRoot() {
		return fmt.Errorf("Daemon must not run as root")
	}

	h.dropped = true
	return nil
}
