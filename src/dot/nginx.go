package dot

import (
	"os"
	"text/template"
)

func UpstreamConf(name string, ups []string, tmplPath, targetPath string) error {
	f, err := os.Create(targetPath)
	defer f.Close()
	if err != nil {
		return err
	}
	data := struct {
		Name      string
		UpStreams []string
	}{
		Name:      name,
		UpStreams: ups,
	}
	t := template.Must(template.ParseFiles(tmplPath))
	err = t.Execute(f, data)
	if err != nil {
		return err
	}
	return nil
}

func ServerConf(name, podname, staticPath, staticDir, tmplPath, targetPath string) error {
	f, err := os.Create(targetPath)
	defer f.Close()
	if err != nil {
		return err
	}
	data := struct {
		Name    string
		PodName string
		Static  string
		Path    string
	}{
		Name:    name,
		PodName: podname,
		Static:  staticPath,
		Path:    staticDir,
	}
	t := template.Must(template.ParseFiles(tmplPath))
	err = t.Execute(f, data)
	if err != nil {
		return err
	}
	return nil
}
