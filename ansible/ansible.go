package ansible

import (
	"bufio"
	"bytes"
	"context"
	"fmt"
	"io"
	"os"
	"strings"
	"sync"

	"github.com/apenella/go-ansible/pkg/execute"
	"github.com/apenella/go-ansible/pkg/playbook"
	"github.com/vbauerster/mpb/v7"
	"github.com/vbauerster/mpb/v7/decor"

	log "github.com/sirupsen/logrus"
)

func Run2(inventory string, playbookLocation string, clusterName string) error {
	log.Info("Running Kubespray")

	pb := &playbook.AnsiblePlaybookCmd{
		Playbooks: []string{playbookLocation},
		Options: &playbook.AnsiblePlaybookOptions{
			Inventory: inventory,
		},
	}

	if err := pb.Run(context.TODO()); err != nil {
		fmt.Println(err)
	}

	return nil
}

func Run(inventory string, playbookLocation string, clusterName string) error {
	buff := new(bytes.Buffer)
	errBuff := new(bytes.Buffer)
	execute := execute.NewDefaultExecute(
		execute.WithWrite(io.Writer(buff)),
		execute.WithWriteError(io.Writer(errBuff)),
	)
	pb := &playbook.AnsiblePlaybookCmd{
		Playbooks: []string{playbookLocation},
		Options: &playbook.AnsiblePlaybookOptions{
			Inventory: inventory,
		},
		Exec: execute,
	}
	maxTaskNumber := 1555
	var wg sync.WaitGroup
	p := mpb.New(mpb.WithWaitGroup(&wg), mpb.WithWidth(32))
	bar := p.AddBar(int64(maxTaskNumber),
		mpb.PrependDecorators(
			// simple name decorator
			decor.Name("deploying kubespray"),
			decor.OnComplete(
				// spinner decorator with default style
				decor.Spinner(nil, decor.WCSyncSpace), "done",
			),
		),
		mpb.AppendDecorators(
			// decor.DSyncWidth bit enables column width synchronization
			decor.Percentage(decor.WCSyncWidth),
		),
	)
	var done = make(chan bool)
	go func() {
		if err := pb.Run(context.TODO()); err != nil {
			fmt.Println(err)
		}
		done <- true
	}()
	f, err := os.OpenFile("/var/log/ans.log",
		os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		return err
	}
	defer f.Close()
	go func() {
		for {
			r := bufio.NewReader(buff)
			line, _, _ := r.ReadLine()
			if string(line) != "" && strings.HasPrefix(string(line), "TASK") {
				wg.Add(1)
				increment(bar, &wg)
			}
			if string(line) != "" {
				if _, err := f.WriteString(string(line) + "\n"); err != nil {
					log.Println(err)
				}
			}

		}
	}()
	defer wg.Done()
	wg.Wait()
	<-done
	return nil
}

func PercentageChange(taskCounter, maxTaskNumber int) (delta float64) {
	delta = 100 / float64(maxTaskNumber) * float64(taskCounter)
	return
}

func increment(bar *mpb.Bar, wg *sync.WaitGroup) {

	bar.Increment()
}
