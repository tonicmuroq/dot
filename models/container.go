package models

import . "../utils"

type Container struct {
	ID          int `orm:"column(id);auto;pk"`
	Port        int
	ContainerID string `orm:"column(container_id)"`
	IdentID     string `orm:"column(ident_id)"`
	HostID      int    `orm:"column(host_id)"`
	AppName     string
	Version     string
}

func (c *Container) Application() *Application {
	return GetApplication(c.AppName)
}

func (c *Container) AppVersion() *AppVersion {
	return GetVersion(c.AppName, c.Version)
}

func (c *Container) Host() *Host {
	return GetHostByID(c.HostID)
}

func (c *Container) Delete() bool {
	host := c.Host()
	if host != nil {
		host.RemovePort(c.Port)
	} else {
		Logger.Debug("Host not found when deleting container")
		return false
	}
	if _, err := db.Delete(&Container{ID: c.ID}); err == nil {
		return true
	}
	return false
}

func NewContainer(av *AppVersion, host *Host, port int, containerID, identID string) *Container {
	c := Container{
		Port:        port,
		ContainerID: containerID,
		IdentID:     identID,
		HostID:      host.ID,
		AppName:     av.Name,
		Version:     av.Version,
	}
	if _, err := db.Insert(&c); err == nil {
		return &c
	}
	return nil
}

func GetContainerByCid(cid string) *Container {
	var container Container
	err := db.QueryTable(new(Container)).Filter("ContainerID", cid).One(&container)
	if err != nil {
		return nil
	}
	return &container
}

func GetContainerByHostAndApp(host *Host, appname string) []*Container {
	var cs []*Container
	db.QueryTable(new(Container)).Filter("HostID", host.ID).Filter("AppName", appname).OrderBy("Port").All(&cs)
	return cs
}

func GetContainerByHostAndAppVersion(host *Host, av *AppVersion) []*Container {
	var cs []*Container
	db.QueryTable(new(Container)).Filter("HostID", host.ID).Filter("AppName", av.Name).Filter("Version", av.Version).OrderBy("Port").All(&cs)
	return cs
}

func GetContainerByHost(host *Host) []*Container {
	var cs []*Container
	db.QueryTable(new(Container)).Filter("HostID", host.ID).OrderBy("Port").All(&cs)
	return cs
}
