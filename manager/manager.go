package manager

import (
	"context"
	"os"
	"os/signal"
	"syscall"
	"time"

	"k8s.io/klog/v2"

	"github.com/ttsubo2000/esi-terraform-worker/controllers"
)

//var onlyOneSignalHandler = make(chan struct{})
var shutdownSignals = []os.Signal{os.Interrupt, syscall.SIGTERM}

// SetupSignalHandler registers for SIGTERM and SIGINT. A context is returned
// which is canceled on one of these signals. If a second signal is caught, the program
// is terminated with exit code 1.
func SetupSignalHandler() context.Context {
	ctx, cancel := context.WithCancel(context.Background())

	c := make(chan os.Signal, 1)
	signal.Notify(c, shutdownSignals...)
	go func() {
		<-c
		klog.Info("Stopping controller")
		cancel()
		time.Sleep(1 * time.Second)
		os.Exit(1) // Exit directly.
	}()

	return ctx
}

// A Manager is required to create Controllers.
type Manager interface {
	Add(c *controllers.Controller) error
	Start(ctx context.Context) error
}

type controllerManager struct {
	runnables []*controllers.Controller
}

// Add sets dependencies on i, and adds it to the list of Runnables to start.
func (cm *controllerManager) Add(c *controllers.Controller) error {
	cm.runnables = append(cm.runnables, c)
	return nil
}

func (cm *controllerManager) Start(ctx context.Context) error {
	errChan := make(chan error)
	for _, c := range cm.runnables {
		go c.Run(ctx, errChan)
	}

	select {
	case <-ctx.Done():
		// We are done
		return nil
	case err := <-errChan:
		// Error starting or running a runnable
		return err
	}
}

// New returns a new Manager for creating Controllers.
func NewManager() Manager {
	return &controllerManager{}
}
