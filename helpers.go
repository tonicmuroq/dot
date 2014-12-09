package main

import (
	"./models"
	"errors"
)

func DeployApplicationHelper(av *models.AppVersion, hosts []*models.Host, daemon bool) ([]int, error) {
	var err error
	taskIds := []int{}
	for _, host := range hosts {
		if host == nil {
			continue
		}
		cs := models.GetContainerByHostAndAppVersion(host, av)
		if len(cs) == 0 {
			task := models.AddContainerTask(av, host, daemon)
			if task != nil {
				taskIds = append(taskIds, task.ID)
				err = hub.Dispatch(host.IP, task)
			} else {
				err = errors.New("task created error")
			}
		} else {
			for _, c := range cs {
				task := models.UpdateContainerTask(c, av)
				if task != nil {
					taskIds = append(taskIds, task.ID)
					err = hub.Dispatch(host.IP, task)
				} else {
					err = errors.New("task created error")
				}
			}
		}
	}
	return taskIds, err
}

func RemoveApplicationFromHostHelper(av *models.AppVersion, host *models.Host) ([]int, error) {
	var err error
	taskIds := []int{}
	for _, c := range models.GetContainerByHostAndAppVersion(host, av) {
		task := models.RemoveContainerTask(c)
		if task != nil {
			taskIds = append(taskIds, task.ID)
			err = hub.Dispatch(host.IP, task)
		} else {
			err = errors.New("task created error")
		}
	}
	return taskIds, err
}

func UpdateApplicationHelper(from, to *models.AppVersion, hosts []*models.Host) ([]int, error) {
	var err error
	taskIds := []int{}
	for _, host := range hosts {
		if host == nil {
			continue
		}
		oldContainers := models.GetContainerByHostAndAppVersion(host, from)
		if len(oldContainers) > 0 {
			for _, c := range oldContainers {
				task := models.UpdateContainerTask(c, to)
				if task != nil {
					taskIds = append(taskIds, task.ID)
					err = hub.Dispatch(host.IP, task)
				} else {
					err = errors.New("task created error")
				}
			}
		}
	}
	return taskIds, err
}
