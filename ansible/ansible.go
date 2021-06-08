package ansible

import (
	"bufio"
	"bytes"
	"context"
	"fmt"
	"io"
	"os"
	"strings"
	"time"

	"github.com/apenella/go-ansible/pkg/execute"
	"github.com/apenella/go-ansible/pkg/playbook"
	"github.com/briandowns/spinner"

	log "github.com/sirupsen/logrus"
)

func Run(inventory string, playbookLocation string, clusterName string) error {
	log.Info("Running Kubespray")
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
	taskCounter := 0
	maxTaskNumber := 1555
	s := spinner.New(spinner.CharSets[9], 100*time.Millisecond)
	s.Start()
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
				/*
					submatchall := re.FindAllString(string(line), -1)
					var lineString string
					for _, element := range submatchall {
						element = strings.Trim(element, "[")
						element = strings.Trim(element, "]")
						lineString = element
					}
				*/
				taskCounter = taskCounter + 1
				perc := fmt.Sprintf("%0.2f %%", PercentageChange(taskCounter, maxTaskNumber))
				s.Prefix = fmt.Sprintf("%s ", perc)
				s.Restart()
				//fmt.Println(perc)
			}
			if string(line) != "" {
				if _, err := f.WriteString(string(line) + "\n"); err != nil {
					log.Println(err)
				}
			}

		}
	}()
	<-done

	return nil
}

func PercentageChange(taskCounter, maxTaskNumber int) (delta float64) {
	delta = 100 / float64(maxTaskNumber) * float64(taskCounter)
	return
}
