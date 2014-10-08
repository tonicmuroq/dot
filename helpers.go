package main

func DeployApplicationHelper(app *Application, hosts []*Host) error {
	var err error
	for _, host := range hosts {
		if host == nil {
			continue
		}
		cs := GetContainerByHostAndApp(host, app)
		if len(cs) == 0 {
			err = hub.Dispatch(host.IP, AddContainerTask(app, host))
		} else {
			for _, c := range cs {
				err = hub.Dispatch(host.IP, UpdateContainerTask(c, app))
			}
		}
	}
	return err
}

func RemoveApplicationFromHostHelper(app *Application, host *Host) error {
	var err error
	for _, c := range GetContainerByHostAndApp(host, app) {
		err = hub.Dispatch(host.IP, RemoveContainerTask(c))
	}
	return err
}

func UpdateApplicationHelper(fromApp, toApp *Application, hosts []*Host) error {
	var err error
	for _, host := range hosts {
		if host == nil {
			continue
		}
		oldContainers := GetContainerByHostAndApp(host, fromApp)
		if len(oldContainers) > 0 {
			for _, c := range oldContainers {
				err = hub.Dispatch(host.IP, UpdateContainerTask(c, toApp))
			}
		}
	}
	return err
}
