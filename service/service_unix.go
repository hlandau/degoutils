package service
import "github.com/hlandau/degoutils/passwd"
import "github.com/hlandau/degoutils/daemon"
import sddaemon "github.com/coreos/go-systemd/daemon"
import "github.com/ErikDubbelboer/gspt"
import "flag"
import "fmt"

var uidFlag = flag.String("uid", "", "UID to run as (default: don't drop privileges)")
var gidFlag = flag.String("gid", "", "GID to run as (default: don't drop privileges)")
var daemonizeFlag = flag.Bool("daemon", false, "Run as daemon? (doesn't fork)")

func systemdUpdateStatus(status string) error {
	return sddaemon.SdNotify(status)
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

	if *daemonizeFlag {
		err := daemon.Daemonize()
		if err != nil {
			return err
		}
	}

	if (*uidFlag == "") != (*gidFlag == "") {
		return fmt.Errorf("Both a UID and GID must be specified, or neither")
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

		err = daemon.DropPrivileges(uid, gid)
		if err != nil {
			return err
		}
	}

	if !info.AllowRoot && daemon.IsRoot() {
		return fmt.Errorf("Daemon must not run as root")
	}

	return info.runInteractively()
}
