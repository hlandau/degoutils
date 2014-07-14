package net
import "math"

// Expresses a backoff and retry specification.
//
// The nil value of this structure results in sensible defaults being used.
type RetryConfig struct {
  // The maximum number of attempts which may be made.
  // If this is 0, the number of attempts is unlimited.
  MaxTries int

  // The initial delay, in milliseconds. This is the delay used after the first
  // failed attempt.
  InitialDelay int // ms

  // The maximum delay, in milliseconds. This is the maximum delay between
  // attempts.
  MaxDelay int     // ms

  // Determines when the maximum delay should be reached. If this is 5, the
  // maximum delay will be reached after 5 attempts have been made.
  MaxDelayAfterTries int

  // The current try. You should not need to set this yourself.
  CurrentTry int
}

// Initialises any nil field in RetryConfig with sensible defaults. You
// normally do not need to call this method yourself, as it will be called
// automatically.
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

// Gets the next delay in milliseconds and increments the internal try counter.
func (rc *RetryConfig) GetStepDelay() int {
  rc.InitDefaults()

  if rc.MaxTries != 0 && rc.CurrentTry >= rc.MaxTries {
    return 0
  }

  // [from backoff.c]
  k := math.Log2(float64(rc.MaxDelay)/float64(rc.InitialDelay)) / float64(rc.MaxDelayAfterTries)
  d := int(float64(rc.InitialDelay)*math.Exp2(float64(rc.CurrentTry)*k))
  rc.CurrentTry += 1

  if d > rc.MaxDelay {
    d = rc.MaxDelay
  }

  return d
}

// Sets the internal try counter to zero; the next delay returned will be
// InitialDelay again.
func (rc *RetryConfig) Reset() {
  rc.CurrentTry = 0
}
