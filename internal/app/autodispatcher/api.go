package autodispatcher

import (
	"fmt"
	"github.com/busgo/forest/internal/app/global"
	"github.com/go-xorm/xorm"
	"github.com/labstack/echo"
	"github.com/labstack/echo/middleware"
	"github.com/labstack/gommon/log"
	"github.com/robfig/cron"
	"net/http"
	"time"
)

type JobAPi struct {
	node *JobNode
	echo *echo.Echo
}

func NewJobAPi(node *JobNode) (api *JobAPi) {

	api = &JobAPi{
		node: node,
	}
	e := echo.New()
	e.Use(middleware.CORSWithConfig(middleware.CORSConfig{
		AllowOrigins: []string{"*", "*"},
		AllowHeaders: []string{echo.HeaderOrigin, echo.HeaderContentType, echo.HeaderAccept, echo.HeaderAccessControlAllowOrigin},
	}))
	e.POST("/job/add", api.AddJob) //这个可以进行
	e.POST("/job/edit", api.editJob)
	e.POST("/job/delete", api.deleteJob)
	e.POST("/job/list", api.jobList)
	e.POST("/job/execute", api.manualExecute)
	e.POST("/group/add", api.addGroup) // 节点是注册的，集群是手动添加的，这样看
	e.POST("/group/list", api.groupList)
	e.POST("/node/list", api.nodeList)
	e.POST("/plan/list", api.planList)
	e.POST("/client/list", api.clientList)
	e.POST("/snapshot/list", api.snapshotList)
	e.POST("/snapshot/delete", api.snapshotDelete)
	e.POST("/execute/snapshot/list", api.executeSnapshotList)
	go func() {
		e.Logger.Fatal(e.Start(node.ApiAddress)) // 这里就开始无限执行了 // 如果这个东西放到最后就不需要这样了，可以直接运行，自己就会阻塞无限循环，如果放到中间，就必须进行协程运行。
	}()
	api.echo = e
	return
}

// add a new job
func (api *JobAPi) AddJob(context echo.Context) (err error) {

	var (
		message string
	)
	jobConf := new(JobConf)
	if err = context.Bind(jobConf); err != nil { // 这里是前端传过来直接制定了

		message = "请求参数不能为空"
		goto ERROR
	}

	if jobConf.Name == "" {
		message = "任务名称不能为空"
		goto ERROR
	}
	if jobConf.Group == "" {
		message = "任务分组不能为空"
		goto ERROR
	}

	if jobConf.Cron == "" {
		message = "任务Cron表达式不能为空"
		goto ERROR
	}

	if _, err = cron.Parse(jobConf.Cron); err != nil {
		message = "非法的Cron表达式"
		goto ERROR
	}

	if jobConf.Target == "" {
		message = "任务Target不能为空"
		goto ERROR
	}

	if jobConf.Status == 0 {
		message = "任务状态不能为空"
		goto ERROR
	}

	if err = api.node.Manager.AddJob(jobConf); err != nil {
		message = err.Error()
		goto ERROR
	}

	return context.JSON(http.StatusOK, Result{Code: 0, Data: jobConf, Message: "创建成功"})

ERROR:
	return context.JSON(http.StatusOK, Result{Code: -1, Message: message})
}

// edit a job
func (api *JobAPi) editJob(context echo.Context) (err error) {

	var (
		message string
	)
	jobConf := new(JobConf)
	if err = context.Bind(jobConf); err != nil {

		message = "请求参数不能为空"
		goto ERROR
	}

	if jobConf.Id == "" {
		message = "此任务记录不存在"
		goto ERROR
	}
	if jobConf.Name == "" {
		message = "任务名称不能为空"
		goto ERROR
	}
	if jobConf.Group == "" {
		message = "任务分组不能为空"
		goto ERROR
	}

	if jobConf.Cron == "" {
		message = "任务Cron表达式不能为空"
		goto ERROR
	}

	if _, err = cron.Parse(jobConf.Cron); err != nil {
		message = "非法的Cron表达式"
		goto ERROR
	}

	if jobConf.Target == "" {
		message = "任务Target不能为空"
		goto ERROR
	}

	if jobConf.Status == 0 {
		message = "任务状态不能为空"
		goto ERROR
	}

	if err = api.node.Manager.EditJob(jobConf); err != nil {
		message = err.Error()
		goto ERROR
	}

	return context.JSON(http.StatusOK, Result{Code: 0, Data: jobConf, Message: "修改成功"})

ERROR:
	return context.JSON(http.StatusOK, Result{Code: -1, Message: message})
}

// job  list
func (api *JobAPi) jobList(context echo.Context) (err error) {

	var (
		jobConfs []*JobConf
	)

	if jobConfs, err = api.node.Manager.JobList(); err != nil {
		return context.JSON(http.StatusOK, Result{Code: -1, Message: err.Error()})
	}
	return context.JSON(http.StatusOK, Result{Code: 0, Data: jobConfs, Message: "查询成成功"})

}

// delete a job
func (api *JobAPi) deleteJob(context echo.Context) (err error) {

	var (
		message string
	)
	jobConf := new(JobConf)
	if err = context.Bind(jobConf); err != nil {

		message = "请求参数不能为空"
		goto ERROR
	}

	if jobConf.Id == "" {
		message = "此任务记录不存在"
		goto ERROR
	}

	if err = api.node.Manager.DeleteJob(jobConf); err != nil {
		message = err.Error()
		goto ERROR
	}

	return context.JSON(http.StatusOK, Result{Code: 0, Data: jobConf, Message: "删除成功"})

ERROR:
	return context.JSON(http.StatusOK, Result{Code: -1, Message: message})
}

// add a job group
func (api *JobAPi) addGroup(context echo.Context) (err error) {

	var (
		message string
	)
	//fmt.Printf("%+v\n",context)
	//fmt.Println(context.QueryParam("a"))
	groupConf := new(GroupConf) // 所以添加一个json就行了，在header中
	if err = context.Bind(groupConf); err != nil {

		message = "请求参数不能为空"
		goto ERROR
	}

	if groupConf.Name == "" {
		message = "任务集群名称不能为空"
		goto ERROR
	}

	if groupConf.Remark == "" {
		message = "任务集群描述"
		goto ERROR
	}

	if err = api.node.Manager.AddGroup(groupConf); err != nil {
		message = err.Error()
		goto ERROR
	}

	return context.JSON(http.StatusOK, Result{Code: 0, Data: groupConf, Message: "添加成功"})

ERROR:
	return context.JSON(http.StatusOK, Result{Code: -1, Message: message})
}

// job group list
func (api *JobAPi) groupList(context echo.Context) (err error) {

	var (
		groupConfs []*GroupConf
	)

	if groupConfs, err = api.node.Manager.GroupList(); err != nil {
		return context.JSON(http.StatusOK, Result{Code: -1, Message: err.Error()})
	}
	return context.JSON(http.StatusOK, Result{Code: 0, Data: groupConfs, Message: "查询成成功"})

}

// job node list
func (api *JobAPi) nodeList(context echo.Context) (err error) {

	var (
		nodes     []*Node
		leader    []byte
		nodeNames []string
	)

	if nodeNames, err = api.node.Manager.NodeList(); err != nil {
		return context.JSON(http.StatusOK, Result{Code: -1, Message: err.Error()})
	}

	if leader, err = api.node.Etcd.Get(global.JobNodeElectPath); err != nil {
		return context.JSON(http.StatusOK, Result{Code: -1, Message: err.Error()})
	}

	if len(nodeNames) == 0 {
		return context.JSON(http.StatusOK, Result{Code: 0, Data: nodes, Message: "查询成成功"})
	}

	nodes = make([]*Node, 0)

	for _, name := range nodeNames {

		if name == string(leader) {
			nodes = append(nodes, &Node{Name: name, State: global.NodeLeaderState})
		} else {
			nodes = append(nodes, &Node{Name: name, State: global.NodeFollowerState}) // follower node
		}

	}

	return context.JSON(http.StatusOK, Result{Code: 0, Data: nodes, Message: "查询成成功"})

}

func (api *JobAPi) planList(context echo.Context) (err error) {

	var (
		plans []*SchedulePlan
	)
	schedulePlans := api.node.Scheduler.SchedulePlans
	if len(schedulePlans) == 0 {

		return context.JSON(http.StatusOK, Result{Code: 0, Data: plans})

	}

	plans = make([]*SchedulePlan, 0)

	for _, p := range schedulePlans {

		plans = append(plans, p)

	}

	return context.JSON(http.StatusOK, Result{Code: 0, Data: plans})
}

func (api *JobAPi) clientList(context echo.Context) (err error) {

	var (
		query     *QueryClientParam
		message   string
		group     *Group
		clients   []*JobClient
		groupPath string
	)

	query = new(QueryClientParam)
	if err = context.Bind(query); err != nil {
		message = "请选择任务集群"
		goto ERROR
	}

	if query.Group == "" {
		message = "请选择任务集群"
		goto ERROR
	}
	groupPath = fmt.Sprintf("%s%s", global.GroupConfPath, query.Group)
	if group = api.node.GroupManager.Groups[groupPath]; group == nil {
		message = "此任务集群不存在"
		goto ERROR
	}

	clients = make([]*JobClient, 0)

	for _, c := range group.Clients {

		clients = append(clients, &JobClient{Name: c.Name, Path: c.Path, Group: query.Group})
	}

	return context.JSON(http.StatusOK, Result{Code: 0, Data: clients, Message: "查询成成功"})

ERROR:
	return context.JSON(http.StatusOK, Result{Code: -1, Message: message})
}

// 任务快照
func (api *JobAPi) snapshotList(context echo.Context) (err error) {

	// 这是为了拿下面所有的快照
	var (
		query     *QuerySnapshotParam
		message   string
		keys      [][]byte
		values    [][]byte
		snapshots []*JobSnapshot
		prefix    string
	)

	query = new(QuerySnapshotParam)
	if err = context.Bind(query); err != nil {
		message = "非法的请求参数"
		goto ERROR
	}

	prefix = global.JobSnapshotPath
	if query.Group != "" && query.Id != "" && query.Ip != "" {
		prefix = fmt.Sprintf(global.JobClientSnapshotPath, query.Group, query.Ip)
		prefix = fmt.Sprintf("%s/%s", prefix, query.Id)
	} else if query.Group != "" && query.Ip != "" {
		prefix = fmt.Sprintf(global.JobClientSnapshotPath, query.Group, query.Ip)
	} else if query.Group != "" && query.Ip == "" {
		prefix = fmt.Sprintf(global.JobSnapshotGroupPath, query.Group)
	} // 这里是为了添加筛选条件,前端传过来的

	if keys, values, err = api.node.Etcd.GetWithPrefixKeyLimit(prefix, 500); err != nil { // 这里限制500
		message = err.Error()
		goto ERROR
	}

	snapshots = make([]*JobSnapshot, 0)
	if len(keys) == 0 {
		return context.JSON(http.StatusOK, Result{Code: 0, Data: snapshots, Message: "查询成成功"})
	}

	for _, value := range values {

		if len(value) == 0 {
			continue
		}
		var snapshot *JobSnapshot

		if snapshot, err = UParkJobSnapshot(value); err != nil {
			continue
		}

		snapshots = append(snapshots, snapshot)

	}

	return context.JSON(http.StatusOK, Result{Code: 0, Data: snapshots, Message: "查询成成功"})

ERROR:
	return context.JSON(http.StatusOK, Result{Code: -1, Message: message})
}

// 任务删除任务快照
func (api *JobAPi) snapshotDelete(context echo.Context) (err error) {

	var (
		query   *QuerySnapshotParam
		message string
		key     string
	)

	query = new(QuerySnapshotParam)
	if err = context.Bind(query); err != nil {
		message = "非法的请求参数"
		goto ERROR
	}

	if query.Group == "" || query.Id == "" || query.Ip == "" {
		message = "非法的请求参数"
		goto ERROR
	}

	key = fmt.Sprintf(global.JobClientSnapshotPath, query.Group, query.Ip)
	key = fmt.Sprintf("%s/%s", key, query.Id)
	if err = api.node.Etcd.Delete(key); err != nil {
		message = err.Error()
		goto ERROR
	}
	return context.JSON(http.StatusOK, Result{Code: 0, Message: "删除成功"})

ERROR:
	return context.JSON(http.StatusOK, Result{Code: -1, Message: message})
}

func (api *JobAPi) executeSnapshotList(context echo.Context) (err error) {

	var (
		query      *QueryExecuteSnapshotParam
		message    string
		count      int64
		snapshots  []*JobExecuteSnapshot
		totalPage  int64
		where      *xorm.Session
		queryWhere *xorm.Session
	)
	query = new(QueryExecuteSnapshotParam)
	if err = context.Bind(query); err != nil {
		message = "非法的请求参数"
		goto ERROR
	}

	if query.PageSize <= 0 {
		query.PageSize = 10
	}

	if query.PageNo <= 0 {
		query.PageNo = 1
	}

	snapshots = make([]*JobExecuteSnapshot, 0)
	where = api.node.Engine.Where("1=1")
	queryWhere = api.node.Engine.Where("1=1")
	if query.Id != "" {
		where.And("id=?", query.Id)
		queryWhere.And("id=?", query.Id)
	}
	if query.Group != "" {

		where.And("`group`=?", query.Group)
		queryWhere.And("`group`=?", query.Group)
	}

	if query.Ip != "" {

		where.And("ip=?", query.Ip)
		queryWhere.And("ip=?", query.Ip)
	}
	if query.Name != "" {
		where.And("name=?", query.Name)
		queryWhere.And("name=?", query.Name)
	}
	if query.Status != 0 {
		where.And("`status`=?", query.Status)
		queryWhere.And("`status`=?", query.Status)
	}
	if count, err = where.Count(&JobExecuteSnapshot{}); err != nil {
		log.Errorf("err:%#v", err)
		message = "查询失败"
		goto ERROR
	}

	if count > 0 {
		err = queryWhere.Desc("create_time").Limit(query.PageSize, (query.PageNo-1)*query.PageSize).Find(&snapshots)
		if err != nil {
			log.Errorf("err:%#v", err)
			message = "查询失败"
			goto ERROR
		}

		if count%int64(query.PageSize) == 0 {
			totalPage = count / int64(query.PageSize)
		} else {
			totalPage = count/int64(query.PageSize) + 1
		}

	}

	return context.JSON(http.StatusOK, Result{Code: 0, Data: &PageResult{TotalCount: int(count), TotalPage: int(totalPage), List: &snapshots}, Message: "查询成成功"})

ERROR:
	return context.JSON(http.StatusOK, Result{Code: -1, Message: message})

}

// manual execute
func (api *JobAPi) manualExecute(context echo.Context) (err error) {

	var (
		conf         *JobConf
		value        []byte
		client       *Client
		snapshotPath string
		snapshot     *JobSnapshot
		success      bool
	)

	conf = new(JobConf)
	if err = context.Bind(conf); err != nil {
		return context.JSON(http.StatusOK, Result{Code: -1, Message: "非法的参数"})

	}

	// 检查任务配置id是否存在
	if conf.Id == "" {

		return context.JSON(http.StatusOK, Result{Code: -1, Message: "此任务配置不存在"})
	}

	// 查询任务配置
	if value, err = api.node.Etcd.Get(global.JobConfPath + conf.Id); err != nil {
		return context.JSON(http.StatusOK, Result{Code: -1, Message: "查询任务配置出现异常:" + err.Error()})
	}

	// 任务配置是否为空
	if len(value) == 0 {

		return context.JSON(http.StatusOK, Result{Code: -1, Message: "此任务配置内容为空"})
	}

	if conf, err = UParkJobConf(value); err != nil {
		return context.JSON(http.StatusOK, Result{Code: -1, Message: "非法的任务配置内容"})
	}

	// select a execute  job client for group
	if client, err = api.node.GroupManager.SelectClient(conf.Group); err != nil {
		return context.JSON(http.StatusOK, Result{Code: -1, Message: "没有找到此要执行的任务作业节点"})
	}

	// build the job snapshot path
	snapshotPath = fmt.Sprintf(global.JobClientSnapshotPath, conf.Group, client.Name)

	// build job snapshot
	snapshotId := GenerateSerialNo() + conf.Id
	snapshot = &JobSnapshot{
		Id:         snapshotId,
		JobId:      conf.Id,
		Name:       conf.Name,
		Ip:         client.Name,
		Group:      conf.Group,
		Cron:       conf.Cron,
		Target:     conf.Target,
		Params:     conf.Params,
		Mobile:     conf.Mobile,
		Remark:     conf.Remark,
		CreateTime: ToDateString(time.Now()),
	}

	// park the job snapshot
	if value, err = ParkJobSnapshot(snapshot); err != nil {

		return context.JSON(http.StatusOK, Result{Code: -1, Message: err.Error()})
	}

	// dispatch the job snapshot the client
	if success, _, err = api.node.Etcd.PutNotExist(snapshotPath, string(value)); err != nil { // 这里一定会执行,仔细找下
		return context.JSON(http.StatusOK, Result{Code: -1, Message: err.Error()})
	}

	if !success {

		return context.JSON(http.StatusOK, Result{Code: -1, Message: "手动执行任务失败,请重试"})
	}

	return context.JSON(http.StatusOK, Result{Code: 0, Message: "手动执行任务请求已提交"})

}