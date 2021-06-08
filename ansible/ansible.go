package ansible

import (
	"context"

	"github.com/apenella/go-ansible/pkg/playbook"
)

func Run(inventory string, playbookLocation string) error {
	pb := &playbook.AnsiblePlaybookCmd{
		Playbooks: []string{playbookLocation},
		Options: &playbook.AnsiblePlaybookOptions{
			Inventory: inventory,
		},
	}

	err := pb.Run(context.TODO())
	if err != nil {
		return err
	}
	return nil
}
