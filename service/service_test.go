package service_test
import "github.com/hlandau/degoutils/service"

// The following example illustrates the minimal skeleton structure to impleent
// a daemon. This example can run as a service on Windows or a non-forking
// daemon on Linux. The systemd notify protocol is supported.
func Example() {
	service.Main(&service.Info{
		Title: "Foobar Web Server",
		Name: "foobar",
		Description: "Foobar Web Server is the greatest webserver ever.",

		RunFunc: func(smgr service.Manager) error {
			// Start up your service.

			// When it is ready to serve requests, call this.
			smgr.SetStarted()

		loop:
			for {
				select {
				// Handle requests, or do so in another goroutine controlled from here.
				case <-smgr.StopChan():
					break loop
				}
			}

			// Do any necessary teardown.

			return nil
		},
	})
}
