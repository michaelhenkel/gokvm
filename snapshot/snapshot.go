package snapshot

import (
	"os"

	"github.com/jedib0t/go-pretty/v6/table"
)

type Snapshot struct {
	Instance  string
	Name      string
	IsCurrent bool
}

func Render(snapshots []*Snapshot) {
	t := table.NewWriter()
	t.SetOutputMirror(os.Stdout)
	t.AppendHeader(table.Row{"Instance", "Snapshot Name", "Current"})
	for _, ss := range snapshots {
		t.AppendRow(table.Row{ss.Instance, ss.Name, ss.IsCurrent})
	}
	t.SetColumnConfigs([]table.ColumnConfig{
		{Number: 1, AutoMerge: true},
	})
	t.SetStyle(table.StyleLight)
	t.Render()

}
