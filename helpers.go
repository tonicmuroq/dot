package main

import "./models"

func DeployApplicationHelper(app *models.Application, hosts []*models.Host) error {
	var err error
	for _, host := range hosts {
		if host == nil {
			continue
		}
		cs := models.GetContainerByHostAndApp(host, app)
		if len(cs) == 0 {
			err = hub.Dispatch(host.IP, models.AddContainerTask(app, host))
		} else {
			for _, c := range cs {
				err = hub.Dispatch(host.IP, models.UpdateContainerTask(c, app))
			}
		}
	}
	return err
}

func RemoveApplicationFromHostHelper(app *models.Application, host *models.Host) error {
	var err error
	for _, c := range models.GetContainerByHostAndApp(host, app) {
		err = hub.Dispatch(host.IP, models.RemoveContainerTask(c))
	}
	return err
}

func UpdateApplicationHelper(fromApp, toApp *models.Application, hosts []*models.Host) error {
	var err error
	for _, host := range hosts {
		if host == nil {
			continue
		}
		oldContainers := models.GetContainerByHostAndApp(host, fromApp)
		if len(oldContainers) > 0 {
			for _, c := range oldContainers {
				err = hub.Dispatch(host.IP, models.UpdateContainerTask(c, toApp))
			}
		}
	}
	return err
}
