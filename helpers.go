package main

import (
	"./models"
	"errors"
)

func DeployApplicationHelper(app *models.Application, hosts []*models.Host) ([]int, error) {
	var err error
	taskIds := []int{}
	for _, host := range hosts {
		if host == nil {
			continue
		}
		cs := models.GetContainerByHostAndApp(host, app)
		if len(cs) == 0 {
			task := models.AddContainerTask(app, host)
			if task != nil {
				taskIds = append(taskIds, task.Id)
				err = hub.Dispatch(host.IP, task)
			} else {
				err = errors.New("task created error")
			}
		} else {
			for _, c := range cs {
				task := models.UpdateContainerTask(c, app)
				if task != nil {
					taskIds = append(taskIds, task.Id)
					err = hub.Dispatch(host.IP, task)
				} else {
					err = errors.New("task created error")
				}
			}
		}
	}
	return taskIds, err
}

func RemoveApplicationFromHostHelper(app *models.Application, host *models.Host) ([]int, error) {
	var err error
	taskIds := []int{}
	for _, c := range models.GetContainerByHostAndApp(host, app) {
		task := models.RemoveContainerTask(c)
		if task != nil {
			taskIds = append(taskIds, task.Id)
			err = hub.Dispatch(host.IP, task)
		} else {
			err = errors.New("task created error")
		}
	}
	return taskIds, err
}

func UpdateApplicationHelper(fromApp, toApp *models.Application, hosts []*models.Host) ([]int, error) {
	var err error
	taskIds := []int{}
	for _, host := range hosts {
		if host == nil {
			continue
		}
		oldContainers := models.GetContainerByHostAndApp(host, fromApp)
		if len(oldContainers) > 0 {
			for _, c := range oldContainers {
				task := models.UpdateContainerTask(c, toApp)
				if task != nil {
					taskIds = append(taskIds, task.Id)
					err = hub.Dispatch(host.IP, task)
				} else {
					err = errors.New("task created error")
				}
			}
		}
	}
	return taskIds, err
}
