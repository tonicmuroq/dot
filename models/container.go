package models

import . "../utils"

type Container struct {
	Id          int
	Port        int
	ContainerId string
	IdentId     string
	HostId      int
	AppId       int
}

// Container
func (self *Container) TableIndex() [][]string {
	return [][]string{
		[]string{"AppId"},
		[]string{"ContainerId"},
		[]string{"host_id"}, /* TODO 有点tricky */
	}
}

func (self *Container) Application() *Application {
	return GetApplicationById(self.AppId)
}

func (self *Container) Host() *Host {
	return GetHostById(self.HostId)
}

func (self *Container) Delete() bool {
	host := self.Host()
	if host != nil {
		host.RemovePort(self.Port)
	} else {
		Logger.Debug("Host not found when deleting container")
		return false
	}
	if _, err := db.Delete(&Container{Id: self.Id}); err == nil {
		return true
	}
	return false
}

func NewContainer(app *Application, host *Host, port int, containerId, daemonId string) *Container {
	c := Container{Port: port, ContainerId: containerId, IdentId: daemonId, AppId: app.Id, HostId: host.Id}
	if _, err := db.Insert(&c); err == nil {
		return &c
	}
	return nil
}

func GetContainerByCid(cid string) *Container {
	var container Container
	err := db.QueryTable(new(Container)).Filter("ContainerId", cid).One(&container)
	if err != nil {
		return nil
	}
	return &container
}

func GetContainerByHostAndApp(host *Host, app *Application) []*Container {
	var cs []*Container
	db.QueryTable(new(Container)).Filter("HostId", host.Id).Filter("AppId", app.Id).OrderBy("Port").All(&cs)
	return cs
}
