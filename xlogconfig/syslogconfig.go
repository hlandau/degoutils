// +build !windows

package xlogconfig

import "github.com/hlandau/xlog"
import "gopkg.in/hlandau/easyconfig.v1/cflag"
import "log/syslog"
import "strings"
import "gopkg.in/hlandau/svcutils.v1/exepath"
import "github.com/coreos/go-systemd/journal"
import "fmt"

var (
	syslogFacilityFlag  = cflag.String(flagGroup, "facility", "daemon", "syslog facility to use")
	syslogFlag          = cflag.Bool(flagGroup, "syslog", false, "log to syslog?")
	syslogSeverityFlag  = cflag.String(flagGroup, "syslogseverity", "DEBUG", "syslog severity limit")
	journalFlag         = cflag.Bool(flagGroup, "journal", false, "log to systemd journal?")
	journalSeverityFlag = cflag.String(flagGroup, "journalseverity", "DEBUG", "systemd journal severity limit")
)

var facilities = map[string]syslog.Priority{
	"kern":     syslog.LOG_KERN,
	"user":     syslog.LOG_USER,
	"mail":     syslog.LOG_MAIL,
	"daemon":   syslog.LOG_DAEMON,
	"auth":     syslog.LOG_AUTH,
	"syslog":   syslog.LOG_SYSLOG,
	"lpr":      syslog.LOG_LPR,
	"news":     syslog.LOG_NEWS,
	"uucp":     syslog.LOG_UUCP,
	"cron":     syslog.LOG_CRON,
	"authpriv": syslog.LOG_AUTHPRIV,
	"ftp":      syslog.LOG_FTP,
	"local0":   syslog.LOG_LOCAL0,
	"local1":   syslog.LOG_LOCAL1,
	"local2":   syslog.LOG_LOCAL2,
	"local3":   syslog.LOG_LOCAL3,
	"local4":   syslog.LOG_LOCAL4,
	"local5":   syslog.LOG_LOCAL5,
	"local6":   syslog.LOG_LOCAL6,
	"local7":   syslog.LOG_LOCAL7,
}

func openSyslog() {
	if !syslogFlag.Value() {
		return
	}

	syslogFacility := syslogFacilityFlag.Value()
	f, ok := facilities[strings.ToLower(syslogFacility)]
	if !ok {
		return
	}

	pn := exepath.ProgramName
	if pn == "" {
		pn = "unknown"
	}

	w, err := syslog.New(f|syslog.LOG_DEBUG, pn)
	if err != nil {
		return
	}

	sink := xlog.NewSyslogSink(w)

	if sev, ok := xlog.ParseSeverity(syslogSeverityFlag.Value()); ok {
		sink.SetSeverity(sev)
	}

	xlog.RootSink.Add(sink)
}

func openJournal() {
	if !journalFlag.Value() || !journal.Enabled() {
		return
	}

	jsink.Tags = map[string]string{
		"SYSLOG_FACILITY": syslogFacilityFlag.Value(),
	}

	if exepath.ProgramName != "" {
		jsink.Tags["SYSLOG_TAG"] = exepath.ProgramName
	}

	if sev, ok := xlog.ParseSeverity(journalSeverityFlag.Value()); ok {
		jsink.MinSeverity = sev
	}

	xlog.RootSink.Add(&jsink)
}

type journalSink struct {
	Tags        map[string]string
	MinSeverity xlog.Severity
}

func (s *journalSink) ReceiveLocally(sev xlog.Severity, format string, params ...interface{}) {
	s.ReceiveFromChild(sev, format, params...)
}

func (s *journalSink) ReceiveFromChild(sev xlog.Severity, format string, params ...interface{}) {
	if sev > s.MinSeverity {
		return
	}

	journal.Send(fmt.Sprintf(format, params...), journal.Priority(sev.Syslog()), s.Tags)
	// ignore errors
}

var jsink = journalSink{
	MinSeverity: xlog.SevDebug,
}
