package forest

import (
	"context"
	"fmt"
	"github.com/labstack/gommon/log"
	"go.etcd.io/etcd/clientv3"
	"sync"
)

const (
	GroupConfPath = "/forest/server/group/"
	ClientPath    = "/forest/client/%s/clients/"
)

type JobGroupManager struct {
	node   *JobNode
	groups map[string]*Group
	lk     *sync.RWMutex
}

func NewJobGroupManager(node *JobNode) (mgr *JobGroupManager) {

	mgr = &JobGroupManager{
		node:   node,
		groups: make(map[string]*Group),
		lk:     &sync.RWMutex{},
	}

	go mgr.watchGroupPath()

	go mgr.loopLoadGroups()

	return

}

// watch the group path
func (mgr *JobGroupManager) watchGroupPath() {

	keyChangeEventResponse := mgr.node.etcd.WatchWithPrefixKey(GroupConfPath)
	for ch := range keyChangeEventResponse.Event {
		mgr.handleGroupChangeEvent(ch)
	}

}

func (mgr *JobGroupManager) loopLoadGroups() {

RETRY:
	var (
		keys   [][]byte
		values [][]byte
		err    error
	)
	if keys, values, err = mgr.node.etcd.GetWithPrefixKey(GroupConfPath); err != nil {

		goto RETRY
	}

	if len(keys) == 0 {
		return
	}

	for i := 0; i < len(keys); i++ {
		path := string(keys[i])
		groupConf, err := UParkGroupConf(values[i])
		if err != nil {
			log.Warnf("upark the group conf error:%#v", err)
			continue
		}

		mgr.addGroup(groupConf.Name, path)
	}

}

func (mgr *JobGroupManager) addGroup(name, path string) {

	mgr.lk.Lock()
	defer mgr.lk.Unlock()

	if _, ok := mgr.groups[path]; ok {

		return
	}
	group := NewGroup(name, path, mgr.node)
	mgr.groups[path] = group
	log.Infof("add a new group:%s,for path:%s", name, path)

}

// delete a group  for path
func (mgr *JobGroupManager) deleteGroup(path string) {

	var (
		group *Group
		ok    bool
	)
	mgr.lk.Lock()
	defer mgr.lk.Unlock()

	if group, ok = mgr.groups[path]; ok {

		return
	}

	// cancel watch the clients
	_ = group.watcher.Close()
	group.cancelFunc()
	delete(mgr.groups, path)

	log.Infof("delete a  group:%s,for path:%s", group.name, path)
}

// handle the group change event
func (mgr *JobGroupManager) handleGroupChangeEvent(changeEvent *KeyChangeEvent) {

	switch changeEvent.Type {

	case KeyCreateChangeEvent:
		mgr.handleGroupCreateEvent(changeEvent)

	case KeyUpdateChangeEvent:
		// ignore
	case KeyDeleteChangeEvent:
		mgr.handleGroupDeleteEvent(changeEvent)
	}
}

func (mgr *JobGroupManager) handleGroupCreateEvent(changeEvent *KeyChangeEvent) {

	groupConf, err := UParkGroupConf(changeEvent.Value)
	if err != nil {
		log.Warnf("upark the group conf error:%#v", err)
		return
	}

	path := changeEvent.Key
	mgr.addGroup(groupConf.Name, path)

}

func (mgr *JobGroupManager) handleGroupDeleteEvent(changeEvent *KeyChangeEvent) {

	path := changeEvent.Key

	mgr.deleteGroup(path)
}

type Group struct {
	path       string
	name       string
	node       *JobNode
	watchPath  string
	clients    map[string]*Client
	watcher    clientv3.Watcher
	cancelFunc context.CancelFunc
	lk         *sync.RWMutex
}

// create a new group
func NewGroup(name, path string, node *JobNode) (group *Group) {

	group = &Group{
		name:      name,
		path:      path,
		node:      node,
		watchPath: fmt.Sprintf(ClientPath, name),
		clients:   make(map[string]*Client),
		lk:        &sync.RWMutex{},
	}

	go group.watchClientPath()

	return
}

// watch the client path
func (group *Group) watchClientPath() {

	keyChangeEventResponse := group.node.etcd.WatchWithPrefixKey(group.watchPath)
	group.watcher = keyChangeEventResponse.Watcher
	group.cancelFunc = keyChangeEventResponse.CancelFunc
	for ch := range keyChangeEventResponse.Event {

		group.handleClientChangeEvent(ch)

	}

}

// handle the client change event
func (group *Group) handleClientChangeEvent(changeEvent *KeyChangeEvent) {

	switch changeEvent.Type {

	case KeyCreateChangeEvent:
		path := changeEvent.Key
		name := string(changeEvent.Value)
		group.addClient(path, string(name))

	case KeyUpdateChangeEvent:
		//ignore
	case KeyDeleteChangeEvent:
		path := changeEvent.Key
		group.deleteClient(path)
	}
}

// add  a new  client
func (group *Group) addClient(name, path string) {

	group.lk.Lock()
	defer group.lk.Unlock()
	if _, ok := group.clients[path]; ok {
		log.Warnf("name:%s,path:%s,the client exist", name, path)
		return
	}

	client := &Client{
		name: name,
		path: path,
	}

	group.clients[path] = client

}

// delete a client for path
func (group *Group) deleteClient(path string) {
	group.lk.Lock()
	defer group.lk.Unlock()
	if _, ok := group.clients[path]; !ok {
		log.Warnf("path:%s,the client not  exist", path)
		return
	}

	delete(group.clients, path)

}

// client
type Client struct {
	name string
	path string
}