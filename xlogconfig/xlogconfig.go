package xlogconfig

import "github.com/hlandau/xlog"
import "gopkg.in/hlandau/easyconfig.v1/cflag"
import "os"

var (
	flagGroup          = cflag.NewGroup(nil, "xlog")
	logSeverityFlag    = cflag.String(flagGroup, "severity", "NOTICE", "log severity (any syslog severity name or number)")
	logFileFlag        = cflag.String(flagGroup, "file", "", "log to filename")
	fileSeverityFlag   = cflag.String(flagGroup, "fileseverity", "DEBUG", "file logging severity limit")
	logStderrFlag      = cflag.Bool(flagGroup, "stderr", true, "log to stderr?")
	stderrSeverityFlag = cflag.String(flagGroup, "stderrseverity", "DEBUG", "stderr logging severity limit")
)

func openStderr() {
	if logStderrFlag.Value() {
		if sev, ok := xlog.ParseSeverity(stderrSeverityFlag.Value()); ok {
			xlog.StderrSink.SetSeverity(sev)
		}

		return
	}

	xlog.RootSink.Remove(xlog.StderrSink)
}

func openFile() {
	fn := logFileFlag.Value()
	if fn == "" {
		return
	}

	f, err := os.Create(fn)
	if err != nil {
		return
	}

	sink := xlog.NewWriterSink(f)
	if sev, ok := xlog.ParseSeverity(fileSeverityFlag.Value()); ok {
		sink.SetSeverity(sev)
	}

	xlog.RootSink.Add(sink)
}

func setSeverity() {
	sevs := logSeverityFlag.Value()
	sev, ok := xlog.ParseSeverity(sevs)
	if !ok {
		return
	}

	xlog.Root.SetSeverity(sev)
}

// Parse registered configurables and setup logging.
func Init() {
	setSeverity()
	openStderr()
	openSyslog()
	openJournal()
	openFile()
}
