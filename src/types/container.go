package types

import . "utils"

type Container struct {
	ID          int    `orm:"column(id);auto;pk" json:"id"`
	Port        int    `json:"port"`
	ContainerID string `orm:"column(container_id)" json:"container_id"`
	IdentID     string `orm:"column(ident_id)" json:"ident_id"`
	HostID      int    `orm:"column(host_id)" json:"host_id"`
	AppName     string `json:"app_name"`
	Version     string `json:"version"`
	SubApp      string `orm:"column(sub_app)" json:"sub_app"`
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

func NewContainer(av *AppVersion, host *Host, port int, containerID, identID, subApp string) *Container {
	c := Container{
		Port:        port,
		ContainerID: containerID,
		IdentID:     identID,
		HostID:      host.ID,
		AppName:     av.Name,
		Version:     av.Version,
		SubApp:      subApp,
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

func GetContainers(hostID int, appName, version string, start, limit int) []*Container {
	var cs []*Container
	query := db.QueryTable(new(Container))
	if hostID != -1 {
		query = query.Filter("HostID", hostID)
	}
	if appName != "" {
		query = query.Filter("AppName", appName)
	}
	if version != "" {
		query = query.Filter("Version", version)
	}
	query.OrderBy("AppName").Limit(limit, start).All(&cs)
	return cs
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
