package net
import "math"

type RetryConfig struct {
  MaxTries int
  InitialDelay int // ms
  MaxDelay int     // ms
  MaxDelayAfterTries int
  currentTry int
}

func (rc *RetryConfig) InitDefaults() {
  if rc.InitialDelay == 0 {
    rc.InitialDelay = 5000
  }
  if rc.MaxDelay == 0 {
    rc.MaxDelay = 120000
  }
  if rc.MaxDelayAfterTries == 0 {
    rc.MaxDelayAfterTries = 10
  }
}

func (rc *RetryConfig) GetStepDelay() int {
  rc.InitDefaults()

  if rc.MaxTries != 0 && rc.currentTry >= rc.MaxTries {
    return 0
  }

  // [from backoff.c]
  k := math.Log2(float64(rc.MaxDelay)/float64(rc.InitialDelay)) / float64(rc.MaxDelayAfterTries)
  d := int(float64(rc.InitialDelay)*math.Exp2(float64(rc.currentTry)*k))
  rc.currentTry += 1

  if d > rc.MaxDelay {
    d = rc.MaxDelay
  }

  return d
}

func (rc *RetryConfig) Reset() {
  rc.currentTry = 0
}
