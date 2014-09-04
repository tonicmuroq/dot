package main

import (
	"encoding/json"
	"os"
	"os/user"
)

func JSONDecode(data string, v interface{}) error {
	return json.Unmarshal([]byte(data), v)
}

func JSONEncode(v interface{}) (string, error) {
	r, err := json.Marshal(v)
	return string(r), err
}

func GetUid(username string) (string, error) {
	user, err := user.Lookup(username)
	if err != nil {
		return "", err
	}
	return user.Uid, nil
}

func GetGid(username string) (string, error) {
	user, err := user.Lookup(username)
	if err != nil {
		return "", err
	}
	return user.Gid, nil
}

func EnsureDir(path string, owner, group int) error {
	err := os.Mkdir(path, 0755)
	if err != nil {
		return err
	}
	return os.Chown(path, owner, group)
}

func EnsureFile(path string, owner, group int, content []byte) error {
	file, err := os.Create(path)
	if err != nil {
		return nil
	}
	file.Write(content)
	os.Chmod(path, 0755)
	os.Chown(path, owner, group)
	return nil
}

func EnsureDirAbsent(path string) error {
	return os.RemoveAll(path)
}

func EnsureFileAbsent(path string) error {
	return os.Remove(path)
}
