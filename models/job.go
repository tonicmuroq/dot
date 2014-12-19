package models

import (
	"time"
)

const (
	ADD    = 1
	REMOVE = 2
	BUILD  = 3
	INFO   = 4
	TEST   = 5

	RUNNING = 0
	DONE    = 1

	SUCC = 1
	FAIL = 0
)

type Job struct {
	ID         int       `orm:"column(id);auto;pk" json:"id"`
	AppName    string    `json:"app_name"`
	AppVersion string    `json:"app_version"`
	Status     int       `json:"status"` // 对应状态, Running/Done
	Succ       int       `json:"succ"`   // 成功/失败
	Kind       int       `json:"kind"`   // 类型, Add/Remove/Update/Build/Test
	Result     string    `json:"result"`
	Created    time.Time `orm:"auto_now_add;type(datetime)" json:"created"`
	Finished   time.Time `orm:"auto_now;type(datetime)" json:"finished"`
}

func GetJob(id int) *Job {
	var j Job
	if err := db.QueryTable(new(Job)).Filter("ID", id).One(&j); err != nil {
		return nil
	} else {
		return &j
	}
}

func NewJob(av *AppVersion, kind int) *Job {
	j := &Job{AppName: av.Name, AppVersion: av.Version, Status: RUNNING, Succ: FAIL, Kind: kind}
	_, err := db.Insert(j)
	if err != nil {
		return nil
	}
	return j
}

func GetJobByAppAndRet(av *AppVersion, ret string) *Job {
	var j Job
	err := db.QueryTable(new(Job)).Filter("AppName", av.Name).Filter("AppVersion", av.Version).Filter("Result", ret).One(&j)
	if err != nil {
		return nil
	}
	return &j
}

func GetJobs(name, version string, status, succ, start, limit int) []*Job {
	var jobs []*Job
	query := db.QueryTable(new(Job)).Filter("AppName", name)
	if version != "" {
		query = query.Filter("AppVersion", version)
	}
	if status != -1 {
		query = query.Filter("Status", status)
	}
	if succ != -1 {
		query = query.Filter("Succ", succ)
	}
	query.OrderBy("-id").Limit(limit, start).All(&jobs)
	return jobs
}

func (j *Job) Done(succ int, result string) {
	j.Status = DONE
	j.Succ = succ
	j.Result = result
	db.Update(j)
}

func (j *Job) SetResult(result string) {
	j.Result = result
	db.Update(j)
}
