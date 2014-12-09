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
	ID         int `orm:"column(id);auto;pk"`
	AppName    string
	AppVersion string
	Status     int // 对应状态, Running/Done
	Succ       int // 成功/失败
	Kind       int // 类型, Add/Remove/Update/Build/Test
	Result     string
	Created    time.Time `orm:"auto_now_add;type(datetime)"`
	Finished   time.Time `orm:"auto_now;type(datetime)"`
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
