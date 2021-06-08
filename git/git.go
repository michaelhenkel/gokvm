package git

import (
	"os"

	gogit "github.com/go-git/go-git/v5"
)

func Clone(gitLocation string) error {
	_, err := gogit.PlainClone(gitLocation, false, &gogit.CloneOptions{
		URL:      "https://github.com/kubernetes-sigs/kubespray",
		Progress: os.Stdout,
	})
	if err != nil {
		return err
	}
	return nil
}
