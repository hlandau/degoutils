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
  return nil
}

func Panic(v ...interface{}) {
  if sw != nil {
    sw.Crit(fmt.Sprint(v...))
  }
  log.Panic(v...)
}

func Panice(err error, v ...interface{}) {
  if err != nil {
    Panic(append([]interface{}{err}, v...))
  }
}

func Fatal(v ...interface{}) {
  if sw != nil {
    sw.Crit(fmt.Sprint(v...))
  }
  log.Fatal(v...)
}

func Fatale(err error, v ...interface{}) {
  if err != nil {
    Fatal(append([]interface{}{err}, v...))
  }
}

func Error(v ...interface{}) {
  if sw != nil {
    sw.Err(fmt.Sprint(v...))
  } else {
    log.Print(v...)
  }
}

func Errore(err error, v ...interface{}) {
  if err != nil {
    Error(append([]interface{}{err}, v...))
  }
}

func Warning(v ...interface{}) {
  if sw != nil {
    sw.Warning(fmt.Sprint(v...))
  } else {
    log.Print(v...)
  }
}

func Warninge(err error, v ...interface{}) {
  if err != nil {
    Warning(append([]interface{}{err}, v...))
  }
}

func Notice(v ...interface{}) {
  if sw != nil {
    sw.Notice(fmt.Sprint(v...))
  } else {
    log.Print(v...)
  }
}

func Noticee(err error, v ...interface{}) {
  if err != nil {
    Notice(append([]interface{}{err}, v...))
  }
}

func Info(v ...interface{}) {
  if sw != nil {
    sw.Info(fmt.Sprint(v...))
  } else {
    log.Print(v...)
  }
}

func Infoe(err error, v ...interface{}) {
  if err != nil {
    Info(append([]interface{}{err}, v...))
  }
}

func Debug(v ...interface{}) {
  if sw != nil {
    sw.Debug(fmt.Sprint(v...))
  } else {
    log.Print(v...)
  }
}

func Debuge(err error, v ...interface{}) {
  if err != nil {
    Debug(append([]interface{}{err}, v...))
  }
}
