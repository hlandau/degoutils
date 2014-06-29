package log
import "log"
import "log/syslog"
import "fmt"

var sw *syslog.Writer

func OpenSyslog(name string) error {
  s, err := syslog.New(syslog.LOG_DAEMON|syslog.LOG_DEBUG, name)
  if err != nil {
    return err
  }
  sw = s
}

func LogPanic(v ...interface{}) {
  if sw != nil {
    sw.Crit(fmt.Sprint(v...))
  }
  log.Panic(v...)
}

func LogPanice(err error, v ...interface{}) {
  if err != nil {
    LogPanic(err, v...)
  }
}

func LogFatal(v ...interface{}) {
  if sw != nil {
    sw.Crit(fmt.Sprint(v...))
  } else {
    log.Fatal(v...)
  }
}

func LogFatale(err error, v ...interface{}) {
  if err != nil {
    LogFatal(err, v...)
  }
}

func LogError(v ...interface{}) {
  if sw != nil {
    sw.Err(fmt.Sprint(v...))
  } else {
    log.Print(v...)
  }
}

func LogErrore(err error, v ...interface{}) {
  if err != nil {
    LogError(err, v...)
  }
}

func LogWarning(v ...interface{}) {
  if sw != nil {
    sw.Warning(fmt.Sprint(v...))
  } else {
    log.Print(v...)
  }
}

func LogWarninge(err error, v ...interface{}) {
  if err != nil {
    LogWarning(err, v...)
  }
}

func LogNotice(v ...interface{}) {
  if sw != nil {
    sw.Notice(fmt.Sprint(v...))
  } else {
    log.Print(v...)
  }
}

func LogNoticee(err error, v ...interface{}) {
  if err != nil {
    LogNotice(err, v...)
  }
}

func LogInfo(v ...interface{}) {
  if sw != nil {
    sw.Info(fmt.Sprint(v...))
  } else {
    log.Print(v...)
  }
}

func LogInfoe(err error, v ...interface{}) {
  if err != nil {
    LogInfo(err, v...)
  }
}

func LogDebug(v ...interface{}) {
  if sw != nil {
    sw.Debug(fmt.Sprint(v...))
  } else {
    log.Print(v...)
  }
}

func LogDebuge(err error, v ...interface{}) {
  if err != nil {
    LogDebug(err, v...)
  }
}
