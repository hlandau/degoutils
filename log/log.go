// This package is now deprecated. Please use hlandau/xlog.
package log

import "github.com/hlandau/xlog"

var log, Log = xlog.New("legacy-log")

func Panic(v ...interface{}) {
	log.Panic(v...)
}

func Panice(err error, v ...interface{}) {
	log.Panice(err, v...)
}

func Fatal(v ...interface{}) {
	log.Fatal(v...)
}

func Fatale(err error, v ...interface{}) {
	log.Fatale(err, v...)
}

func Error(v ...interface{}) {
	log.Error(v...)
}

func Errore(err error, v ...interface{}) {
	log.Errore(err, v...)
}

func Warning(v ...interface{}) {
	log.Warn(v...)
}

func Warninge(err error, v ...interface{}) {
	log.Warne(err, v...)
}

func Notice(v ...interface{}) {
	log.Notice(v...)
}

func Noticee(err error, v ...interface{}) {
	log.Noticee(err, v...)
}

func Info(v ...interface{}) {
	log.Info(v...)
}

func Infoe(err error, v ...interface{}) {
	log.Infoe(err, v...)
}

func Debug(v ...interface{}) {
	log.Debug(v...)
}

func Debuge(err error, v ...interface{}) {
	log.Debuge(err, v...)
}
