package autodispatcher

import (
	"errors"
	"github.com/busgo/forest/internal/app/ectd"
	"github.com/busgo/forest/internal/app/global"
	"github.com/labstack/gommon/log"
	"strings"
)

//const (
//	JobConfPath = "/forest/server/conf/"
//)

type JobManager struct {
	node *JobNode
}

func NewJobManager(node *JobNode) (manager *JobManager) {

	manager = &JobManager{
		node: node,
	}

	go manager.watchJobConfPath()
	//go manager.loopLoadJobConf()

	return

}

func (manager *JobManager) watchJobConfPath() {

	keyChangeEventResponse := manager.node.Etcd.WatchWithPrefixKey(global.JobConfPath)

	for ch := range keyChangeEventResponse.Event { // 这里更新配置

		manager.handleJobConfChangeEvent(ch)
	}
}

func (manager *JobManager) LoopLoadJobConf() { // 从这里分配的 但是一开始的节点属于哪个集群是在哪里分配的呢，我还是没有找到

RETRY:
	var (
		keys   [][]byte
		values [][]byte
		err    error
	)
	if keys, values, err = manager.node.Etcd.GetWithPrefixKey(global.JobConfPath); err != nil {

		goto RETRY
	}

	if len(keys) == 0 {
		return
	}

	for i := 0; i < len(keys); i++ {
		jobConf, err := UParkJobConf(values[i])
		if err != nil {
			log.Warnf("upark the job conf error:%#v", err)
			continue
		}
		manager.node.Scheduler.pushJobChangeEvent(&JobChangeEvent{
			Type: global.JobCreateChangeEvent,
			Conf: jobConf,
		})

	}

}

func (manager *JobManager) handleJobConfChangeEvent(changeEvent *ectd.KeyChangeEvent) { // 处理工作创建

	switch changeEvent.Type {

	case global.KeyCreateChangeEvent:

		manager.handleJobCreateEvent(changeEvent.Value)
	case global.KeyUpdateChangeEvent:

		manager.handleJobUpdateEvent(changeEvent.Value)
	case global.KeyDeleteChangeEvent:

		manager.handleJobDeleteEvent(changeEvent.Key)

	}
}

func (manager *JobManager) handleJobCreateEvent(value []byte) {

	var (
		err     error
		jobConf *JobConf
	)
	if len(value) == 0 {

		return
	}

	if jobConf, err = UParkJobConf(value); err != nil {
		log.Printf("unpark the job conf err:%#v", err)
		return
	}

	manager.node.Scheduler.pushJobChangeEvent(&JobChangeEvent{
		Type: global.JobCreateChangeEvent,
		Conf: jobConf,
	})

}

func (manager *JobManager) handleJobUpdateEvent(value []byte) {

	var (
		err     error
		jobConf *JobConf
	)
	if len(value) == 0 {

		return
	}

	if jobConf, err = UParkJobConf(value); err != nil {
		log.Printf("unpark the job conf err:%#v", err)
		return
	}

	manager.node.Scheduler.pushJobChangeEvent(&JobChangeEvent{
		Type: global.JobUpdateChangeEvent,
		Conf: jobConf,
	})

}

// handle the job delete event
func (manager *JobManager) handleJobDeleteEvent(key string) {

	if key == "" {
		return
	}

	pos := strings.LastIndex(key, "/")
	if pos == -1 {
		return
	}

	id := key[pos+1:]

	jobConf := &JobConf{
		Id:      id,
		Version: -1,
	}

	manager.node.Scheduler.pushJobChangeEvent(&JobChangeEvent{
		Type: global.JobDeleteChangeEvent,
		Conf: jobConf,
	})

}

// add job conf
func (manager *JobManager) AddJob(jobConf *JobConf) (err error) {

	var (
		value   []byte
		v       []byte
		success bool
	)

	if value, err = manager.node.Etcd.Get(global.GroupConfPath + jobConf.Group); err != nil { // 这里只是为了检测集群的存在性
		return
	}

	if len(value) == 0 {

		err = errors.New("任务集群不存在")
		return
	}

	jobConf.Id = GenerateSerialNo() // 就是要发布的时候给一个id，然后后面的话引用这个id，给发到一个快照到专门的路径上，然后slave节点做完了就给返回一个结果，给到一个失败的路径上？？？？ // 所以还是发送快照过去，昨晚就删除这个任务，就和bus一样。如果失败了，就写入一个专门的路径，统计所有的坏节点？？？但是失败的概率还是很小的
	jobConf.Version = 1

	if v, err = ParkJobConf(jobConf); err != nil { //值都是json字符串
		return
	}
	if success, _, err = manager.node.Etcd.PutNotExist(global.JobConfPath+jobConf.Id, string(v)); err != nil { // 这里怎么没有看到事件变化啊 // 这里是存取任务
		return
	}

	if !success {
		err = errors.New("创建失败,请重试！")
		return
	}
	return
}

// edit job conf
func (manager *JobManager) EditJob(jobConf *JobConf) (err error) {

	var (
		value   []byte
		v       []byte
		success bool
		oldConf *JobConf
	)

	if value, err = manager.node.Etcd.Get(global.GroupConfPath + jobConf.Group); err != nil {
		return
	}

	if len(value) == 0 {

		err = errors.New("任务集群不存在")
		return
	}

	if jobConf.Id == "" {
		err = errors.New("此记录任务配置记录不存在")
		return
	}

	if value, err = manager.node.Etcd.Get(global.JobConfPath + jobConf.Id); err != nil {
		return
	}

	if len(value) == 0 {
		err = errors.New("此任务配置记录不存在")
		return
	}

	if oldConf, err = UParkJobConf([]byte(value)); err != nil {
		return
	}

	jobConf.Version = oldConf.Version + 1
	if v, err = ParkJobConf(jobConf); err != nil {
		return
	}

	if success, err = manager.node.Etcd.Update(global.JobConfPath+jobConf.Id, string(v), string(value)); err != nil {
		return
	}

	if !success {
		err = errors.New("修改失败,请重试！")
		return
	}
	return
}

// delete job conf
func (manager *JobManager) DeleteJob(jobConf *JobConf) (err error) {

	var (
		value []byte
	)

	if jobConf.Id == "" {
		err = errors.New("此记录任务配置记录不存在")
		return
	}

	if value, err = manager.node.Etcd.Get(global.JobConfPath + jobConf.Id); err != nil {
		return
	}

	if len(value) == 0 {
		err = errors.New("此任务配置记录不存在")
		return
	}
	err = manager.node.Etcd.Delete(global.JobConfPath + jobConf.Id)

	return
}

// job list
func (manager *JobManager) JobList() (jobConfs []*JobConf, err error) {

	var (
		keys   [][]byte
		values [][]byte
	)
	if keys, values, err = manager.node.Etcd.GetWithPrefixKey(global.JobConfPath); err != nil {
		return
	}

	if len(keys) == 0 {
		return
	}

	jobConfs = make([]*JobConf, 0)
	for i := 0; i < len(values); i++ {

		jobConf, err := UParkJobConf(values[i])
		if err != nil {
			log.Printf("upark the job conf errror:%#v", err)
			continue
		}

		jobConfs = append(jobConfs, jobConf)

	}

	return
}

// add group
func (manager *JobManager) AddGroup(groupConf *GroupConf) (err error) {

	var (
		value   []byte
		success bool
	)
	if value, err = ParkGroupConf(groupConf); err != nil {
		return
	}

	if success, _, err = manager.node.Etcd.PutNotExist(global.GroupConfPath+groupConf.Name, string(value)); err != nil { // 这里只管推上去，因为开了监控了，一旦有更新，就会同步过来
		return //在这里添加集群
	}

	if !success {

		err = errors.New("此任务集群已存在") //这个是符合逻辑的
	}

	return

}

// group list
func (manager *JobManager) GroupList() (groupConfs []*GroupConf, err error) {

	var (
		keys   [][]byte
		values [][]byte
	)
	if keys, values, err = manager.node.Etcd.GetWithPrefixKey(global.GroupConfPath); err != nil {
		return
	}

	if len(keys) == 0 {
		return
	}

	groupConfs = make([]*GroupConf, 0)
	for i := 0; i < len(values); i++ {

		groupConf, err := UParkGroupConf(values[i])
		if err != nil {
			log.Printf("upark the group conf errror:%#v", err)
			continue
		}

		groupConfs = append(groupConfs, groupConf)

	}

	return
}

// node list
func (manager *JobManager) NodeList() (nodes []string, err error) {

	var (
		keys   [][]byte
		values [][]byte
	)
	if keys, values, err = manager.node.Etcd.GetWithPrefixKey(global.JobNodePath); err != nil {
		return
	}

	if len(keys) == 0 {
		return
	}

	nodes = make([]string, 0)
	for i := 0; i < len(values); i++ {
		nodes = append(nodes, string(values[i]))
	}

	return
}