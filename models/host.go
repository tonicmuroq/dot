package models

import "../config"

type Host struct {
	Id   int
	IP   string `orm:"column(ip)"`
	Name string
}

type HostPort struct {
	Id     int
	HostId int
	Port   int
}

func (self *Host) TableUnique() [][]string {
	return [][]string{
		[]string{"IP"},
	}
}

func NewHost(ip, name string) *Host {
	host := Host{IP: ip, Name: name}
	if _, id, err := db.ReadOrCreate(&host, "IP"); err == nil {
		host.Id = int(id)
		return &host
	}
	return nil
}

func GetHostById(hostId int) *Host {
	var host Host
	err := db.QueryTable(new(Host)).Filter("Id", hostId).One(&host)
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

// 注意里面可能有nil
func GetHostsByIPs(ips []string) []*Host {
	hosts := make([]*Host, len(ips))
	for i, ip := range ips {
		hosts[i] = GetHostByIP(ip)
	}
	return hosts
}

func (self *Host) Containers() []*Container {
	var cs []*Container
	db.QueryTable(new(Container)).Filter("HostId", self.Id).OrderBy("Port").All(&cs)
	return cs
}

func (self *Host) Ports() []int {
	var ports []*HostPort
	db.QueryTable(new(HostPort)).Filter("HostId", self.Id).OrderBy("Port").All(&ports)
	r := make([]int, len(ports))
	for i := 0; i < len(ports); i = i + 1 {
		r[i] = ports[i].Port
	}
	return r
}

func (self *Host) AddPort(port int) {
	hostPort := HostPort{HostId: self.Id, Port: port}
	db.Insert(&hostPort)
}

func (self *Host) RemovePort(port int) {
	db.Raw("DELETE FROM host_port WHERE host_id=? AND port=?", self.Id, port).Exec()
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
