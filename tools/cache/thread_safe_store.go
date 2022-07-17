package cache

import (
	"sync"

	"github.com/ttsubo2000/esi-terraform-worker/types"
	v1 "k8s.io/api/core/v1"
	rbacv1 "k8s.io/api/rbac/v1"
	"k8s.io/klog/v2"
)

// ThreadSafeStore is an interface that can access to a storage backend.
type ThreadSafeStore interface {
	Add(key string, obj interface{})
	Update(key string, obj interface{})
	Delete(key string)
	Get(key string) (item interface{}, exists bool)
	List() []interface{}
}

// threadSafeMap implements ThreadSafeStore
type threadSafeMap struct {
	lock  sync.RWMutex
	items map[string]interface{}
}

func (c *threadSafeMap) Add(key string, obj interface{}) {
	c.Update(key, obj)
}

func (c *threadSafeMap) Update(key string, obj interface{}) {
	c.lock.Lock()
	defer c.lock.Unlock()
	c.items[key] = obj
	switch obj.(type) {
	case *types.Provider:
		klog.Infof("Update key:[%s], obj:[%v]", key, obj.(*types.Provider))
	case *types.Secret:
		klog.Infof("Update key:[%s], obj:[%v]", key, obj.(*types.Secret))
	case *types.Configuration:
		klog.Infof("Update key:[%s], obj:[%v]", key, obj.(*types.Configuration))
	case *types.ConfigMap:
		klog.Infof("Update key:[%s], obj:[%v]", key, obj.(*types.ConfigMap))
	case *types.Job:
		klog.Infof("Update key:[%s], obj:[%v]", key, obj.(*types.Job))
	case *rbacv1.ClusterRole:
		klog.Infof("Update key:[%s], obj:[%v]", key, obj.(*rbacv1.ClusterRole))
	case *v1.ServiceAccount:
		klog.Infof("Update key:[%s], obj:[%v]", key, obj.(*v1.ServiceAccount))
	case *rbacv1.ClusterRoleBinding:
		klog.Infof("Update key:[%s], obj:[%v]", key, obj.(*rbacv1.ClusterRoleBinding))
	}
}

func (c *threadSafeMap) Delete(key string) {
	c.lock.Lock()
	defer c.lock.Unlock()
	if _, exists := c.items[key]; exists {
		delete(c.items, key)
	}
}

func (c *threadSafeMap) Get(key string) (item interface{}, exists bool) {
	c.lock.RLock()
	defer c.lock.RUnlock()
	item, exists = c.items[key]
	return item, exists
}

func (c *threadSafeMap) List() []interface{} {
	c.lock.RLock()
	defer c.lock.RUnlock()
	list := make([]interface{}, 0, len(c.items))
	for _, item := range c.items {
		list = append(list, item)
	}
	return list
}

// NewThreadSafeStore creates a new instance of ThreadSafeStore.
func NewThreadSafeStore() ThreadSafeStore {
	return &threadSafeMap{
		items: map[string]interface{}{},
	}
}
