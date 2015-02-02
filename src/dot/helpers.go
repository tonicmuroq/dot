package dot

import (
	"errors"

	"types"
)

func DeployApplicationHelper(av *types.AppVersion, hosts []*types.Host, appyaml *types.AppYaml, daemon bool) ([]int, error) {
	var err error
	taskIds := []int{}
	for _, host := range hosts {
		if host == nil {
			continue
		}
		cs := types.GetContainerByHostAndAppVersion(host, av)
		if len(cs) == 0 {
			task := types.AddContainerTask(av, host, appyaml, daemon)
			if task != nil {
				taskIds = append(taskIds, task.ID)
				err = LeviHub.Dispatch(host.IP, task)
			} else {
				err = errors.New("task created error")
			}
		} else {
			for _, c := range cs {
				task := types.UpdateContainerTask(c, av)
				if task != nil {
					taskIds = append(taskIds, task.ID)
					err = LeviHub.Dispatch(host.IP, task)
				} else {
					err = errors.New("task created error")
				}
			}
		}
	}
	return taskIds, err
}

func RemoveApplicationFromHostHelper(av *types.AppVersion, host *types.Host) ([]int, error) {
	var err error
	taskIds := []int{}
	for _, c := range types.GetContainerByHostAndAppVersion(host, av) {
		task := types.RemoveContainerTask(c)
		if task != nil {
			taskIds = append(taskIds, task.ID)
			err = LeviHub.Dispatch(host.IP, task)
		} else {
			err = errors.New("task created error")
		}
	}
	return taskIds, err
}

func UpdateApplicationHelper(from, to *types.AppVersion, hosts []*types.Host) ([]int, error) {
	var err error
	taskIds := []int{}
	for _, host := range hosts {
		if host == nil {
			continue
		}
		oldContainers := types.GetContainerByHostAndAppVersion(host, from)
		if len(oldContainers) > 0 {
			for _, c := range oldContainers {
				task := types.UpdateContainerTask(c, to)
				if task != nil {
					taskIds = append(taskIds, task.ID)
					err = LeviHub.Dispatch(host.IP, task)
				} else {
					err = errors.New("task created error")
				}
			}
		}
	}
	return taskIds, err
}
