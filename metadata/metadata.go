package metadata

import (
	"encoding/xml"
	"fmt"
)

type Metadata struct {
	XMLName xml.Name `xml:"metadata"`
	Net     *string  `xml:"net"`
	Image   *string  `xml:"image"`
	Cluster *string  `xml:"cluster"`
	Subnet  *string  `xml:"subnet"`
}

func GetMetadata(metadata string) (*Metadata, error) {
	metadataString := fmt.Sprintf("<metadata>%s</metadata>", metadata)
	var m Metadata
	if err := xml.Unmarshal([]byte(metadataString), &m); err != nil {
		return nil, err
	}
	return &m, nil
}

func getXMLLine(in *string, t string) string {
	return fmt.Sprintf("<gokvm:%s xmlns:gokvm=\"http://gokvm\">%s</gokvm:%s>", t, *in, t)
}

func (m *Metadata) InstanceMetadata() string {
	var metadataString string
	if m.Subnet != nil {
		metadataString = metadataString + getXMLLine(m.Subnet, "subnet")
	}
	if m.Net != nil {
		metadataString = metadataString + getXMLLine(m.Net, "net")
	}
	if m.Cluster != nil {
		metadataString = metadataString + getXMLLine(m.Cluster, "cluster")
	}
	if m.Image != nil {
		metadataString = metadataString + getXMLLine(m.Image, "image")
	}
	return metadataString

}
