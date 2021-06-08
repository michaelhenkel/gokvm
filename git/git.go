package git

import (
	"os"
	"path/filepath"

	gogit "github.com/go-git/go-git/v5"
)

func Clone(gitLocation string) error {
	fname := filepath.Join(os.TempDir(), "gokvmgitclone.log")
	old := os.Stdout            // keep backup of the real stdout
	temp, _ := os.Create(fname) // create temp file
	os.Stdout = temp

	_, err := gogit.PlainClone(gitLocation, false, &gogit.CloneOptions{
		URL:      "https://github.com/kubernetes-sigs/kubespray",
		Progress: os.Stdout,
	})
	if err != nil {
		if err != gogit.ErrRepositoryAlreadyExists {
			return err
		}
	}
	temp.Close()
	os.Stdout = old
	return nil
}
