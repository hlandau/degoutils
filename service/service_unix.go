package service
import "github.com/hlandau/degoutils/passwd"
import "github.com/hlandau/degoutils/daemon"
import "github.com/hlandau/degoutils/service/sdnotify"
import "github.com/ErikDubbelboer/gspt"
import "flag"
import "fmt"

var uidFlag = flag.String("uid", "", "UID to run as (default: don't drop privileges)")
var gidFlag = flag.String("gid", "", "GID to run as (default: don't drop privileges)")
var daemonizeFlag = flag.Bool("daemon", false, "Run as daemon? (doesn't fork)")
var chrootFlag = flag.String("chroot", "", "Chroot to a directory (must set UID, GID) (\"/\" disables)")

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

	if !flag.Parsed() {
		flag.Parse()
	}

	err = systemdUpdateStatus("\n")
	if err == nil {
		info.systemd = true
	}

	return info.runInteractively()
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
		*gidFlag = *uidFlag
	}

	if h.info.DefaultChroot == "" {
		h.info.DefaultChroot = "/"
	}

	chrootPath := *chrootFlag
	if chrootPath == "" {
		chrootPath = h.info.DefaultChroot
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
