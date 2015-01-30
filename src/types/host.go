package types

import "config"

type Host struct {
	ID     int    `orm:"column(id);auto;pk" json:"id"`
	IP     string `orm:"column(ip)" json:"ip"`
	Name   string `json:"name"`
	Status int    `json:"status"`
}

type Port struct {
	ID     int `orm:"column(id);auto;pk"`
	HostID int `orm:"column(host_id)"`
	Port   int
}

func NewHost(ip, name string) *Host {
	host := &Host{IP: ip, Name: name, Status: 0}
	if _, id, err := db.ReadOrCreate(host, "IP"); err == nil {
		host.ID = int(id)
		if host.Status != 0 {
			host.Online()
		}
		return host
	}
	return nil
}

func GetHostByID(hostID int) *Host {
	var host Host
	err := db.QueryTable(new(Host)).Filter("ID", hostID).One(&host)
	if err != nil {
		return nil
	}
	return &host
}

func GetHostByIP(ip string) *Host {
	var host Host
	err := db.QueryTable(new(Host)).Filter("IP", ip).One(&host)
	if err != nil {
		return nil
	}
	return &host
}

func GetAllHosts(start, limit int) []*Host {
	var hosts []*Host
	db.QueryTable(new(Host)).Limit(limit, start).All(&hosts)
	return hosts
}

func (h *Host) Online() {
	h.Status = 0
	db.Update(h)
}

func (h *Host) Offline() {
	h.Status = 1
	db.Update(h)
}

// 注意里面可能有nil
func GetHostsByIPs(ips []string) []*Host {
	hosts := make([]*Host, len(ips))
	for i, ip := range ips {
		hosts[i] = GetHostByIP(ip)
	}
	return hosts
}

func (h *Host) Containers() []*Container {
	return GetContainerByHost(h)
}

func (h *Host) Ports() []int {
	var ports []*Port
	db.QueryTable(new(Port)).Filter("HostID", h.ID).OrderBy("Port").All(&ports)
	r := make([]int, len(ports))
	for i := 0; i < len(ports); i = i + 1 {
		r[i] = ports[i].Port
	}
	return r
}

func (h *Host) AddPort(port int) {
	p := Port{HostID: h.ID, Port: port}
	db.Insert(&p)
}

func (h *Host) RemovePort(port int) {
	db.Raw("DELETE FROM port WHERE host_id=? AND port=?", h.ID, port).Exec()
}

// 获取一个host上的可用的一个端口
// 如果超出范围就返回0
// 只允许一个访问
func GetPortFromHost(host *Host) int {
	lowerBound, upperBound := config.Config.Minport, config.Config.Maxport+1
	portMutex.Lock()
	defer portMutex.Unlock()
	portList := make([]int, upperBound-lowerBound)
	newPort := lowerBound

	for _, port := range host.Ports() {
		index := port - lowerBound
		if index >= len(portList) || index < 0 {
			continue
		}
		portList[index] = port
	}
	for index, hold := range portList {
		if hold == 0 {
			newPort = lowerBound + index
			break
		}
	}
	if newPort >= upperBound {
		return 0
	} else {
		host.AddPort(newPort)
	}
	return newPort
}
