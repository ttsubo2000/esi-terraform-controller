package controllers

import (
	"context"
	"fmt"
	"time"

	"k8s.io/klog/v2"

	"github.com/ttsubo/client-go/tools/cache"
	"github.com/ttsubo/client-go/util/workqueue"
	cacheObj "github.com/ttsubo2000/terraform-controller/tools/cache"
	"k8s.io/apimachinery/pkg/runtime"
)

// Result contains the result of a Reconciler invocation.
type Result struct {
	// Requeue tells the Controller to requeue the reconcile key.  Defaults to false.
	Requeue bool

	// RequeueAfter if greater than 0, tells the Controller to requeue the reconcile key after the Duration.
	// Implies that Requeue is true, there is no need to set Requeue to true at the same time as RequeueAfter.
	RequeueAfter time.Duration
}

// Request contains the information necessary to reconcile a Kubernetes object.  This includes the
type Request struct {
	// NamespacedName is the name and namespace of the object to reconcile.
	NamespacedName string
}

/*
Reconciler implements a Kubernetes API for a specific Resource by Creating, Updating or Deleting Kubernetes
objects, or by making changes to systems external to the cluster (e.g. cloudproviders, github, etc).

reconcile implementations compare the state specified in an object by a user against the actual cluster state,
and then perform operations to make the actual cluster state reflect the state specified by the user.

Typically, reconcile is triggered by a Controller in response to cluster Events (e.g. Creating, Updating,
Deleting Kubernetes objects) or external Events (GitHub Webhooks, polling external sources, etc).

Example reconcile Logic:

	* Read an object and all the Pods it owns.
	* Observe that the object spec specifies 5 replicas but actual cluster contains only 1 Pod replica.
	* Create 4 Pods and set their OwnerReferences to the object.

reconcile may be implemented as either a type:

	type reconcile struct {}

	func (reconcile) reconcile(controller.Request) (controller.Result, error) {
		// Implement business logic of reading and writing objects here
		return controller.Result{}, nil
	}

Or as a function:

	controller.Func(func(o controller.Request) (controller.Result, error) {
		// Implement business logic of reading and writing objects here
		return controller.Result{}, nil
	})

Reconciliation is level-based, meaning action isn't driven off changes in individual Events, but instead is
driven by actual cluster state read from the apiserver or a local cache.
For example if responding to a Pod Delete Event, the Request won't contain that a Pod was deleted,
instead the reconcile function observes this when reading the cluster state and seeing the Pod as missing.
*/
type Reconciler interface {
	// Reconciler performs a full reconciliation for the object referred to by the Request.
	// The Controller will requeue the Request to be processed again if an error is non-nil or
	// Result.Requeue is true, otherwise upon completion it will remove the work from the queue.
	Reconcile(ctx context.Context, req Request, indexer cache.Indexer) (Result, error)
}

// Controller demonstrates how to implement a controller with client-go.
type Controller struct {
	Name     string
	Do       Reconciler
	indexer  cache.Indexer
	Queue    workqueue.RateLimitingInterface
	informer cache.Controller
}

func newController(name string, r Reconciler, queue workqueue.RateLimitingInterface, indexer cache.Indexer, informer cache.Controller) *Controller {
	return &Controller{
		Name:     name,
		Do:       r,
		informer: informer,
		indexer:  indexer,
		Queue:    queue,
	}
}

// Run controller.
func (c *Controller) Run(ctx context.Context, errChan chan error) {
	klog.Infof("Starting  %s controller", c.Name)
	go c.informer.Run(ctx.Done())
	go c.runWorker(ctx, errChan)
}

func (c *Controller) runWorker(ctx context.Context, errCh chan error) {
	childCtx, childCancel := context.WithCancel(ctx)
	defer childCancel()

	go func() {
		for c.processNextWorkItem(childCtx) {
		}
		errCh <- fmt.Errorf("Error: %s", "WorkerQueue Error")
	}()

	<-childCtx.Done()
	klog.Infof("Shutdown signal received in runWorker on %s controller", c.Name)
}

func (c *Controller) processNextWorkItem(ctx context.Context) bool {
	obj, shutdown := c.Queue.Get()
	if shutdown {
		// Stop working
		return false
	}

	defer c.Queue.Done(obj)

	c.reconcileHandler(ctx, obj)
	return true
}

func (c *Controller) reconcileHandler(ctx context.Context, obj interface{}) {
	req, ok := obj.(Request)
	if !ok {
		// As the item in the workqueue is actually invalid, we call
		// Forget here else we'd go into a loop of attempting to
		// process a work item that is invalid.
		c.Queue.Forget(obj)
		// Return true, don't take a break
		return
	}
	result, err := c.Do.Reconcile(ctx, req, c.indexer)

	switch {
	case err != nil:
		c.Queue.AddRateLimited(req)
		klog.Error(err, "Reconciler error")
	case result.RequeueAfter > 0:
		// The result.RequeueAfter request will be lost, if it is returned
		// along with a non-nil error. But this is intended as
		// We need to drive to stable reconcile loops before queuing due
		// to result.RequestAfter
		c.Queue.Forget(obj)
		c.Queue.AddAfter(req, result.RequeueAfter)
	case result.Requeue:
		c.Queue.AddRateLimited(req)
	default:
		c.Queue.Forget(obj)
	}
}

// NewController creates a new Controller.
func NewController(name string, r Reconciler, objType runtime.Object, clientState cacheObj.Store) *Controller {
	queue := workqueue.NewRateLimitingQueue(workqueue.DefaultControllerRateLimiter())

	indexer, informer := cache.NewIndexerInformer(&cache.ListWatch{}, objType, 0, cache.ResourceEventHandlerFuncs{
		AddFunc: func(obj interface{}) {
			key, err := cache.MetaNamespaceKeyFunc(obj)
			if err == nil {
				req := Request{NamespacedName: key}
				queue.Add(req)
			}
		},
		UpdateFunc: func(old interface{}, new interface{}) {
			key, err := cache.MetaNamespaceKeyFunc(new)
			if err == nil {
				req := Request{NamespacedName: key}
				queue.Add(req)
			}
		},
		DeleteFunc: func(obj interface{}) {
			key, err := cache.DeletionHandlingMetaNamespaceKeyFunc(obj)
			if err == nil {
				req := Request{NamespacedName: key}
				queue.Add(req)
			}
		},
	}, cache.Indexers{})

	clientState.AddInformer(objType, informer)

	return newController(name, r, queue, indexer, informer)
}
