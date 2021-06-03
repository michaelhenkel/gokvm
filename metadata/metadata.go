package metadata

import (
	"encoding/xml"
	"fmt"
)

type Metadata struct {
	XMLName xml.Name `xml:"metadata"`
	Network string   `xml:"net"`
	Image   string   `xml:"image"`
	Cluster string   `xml:"cluster"`
}

func GetMetadata(metadata string) (*Metadata, error) {
	metadataString := fmt.Sprintf("<metadata>%s</metadata>", metadata)
	xmlByte := []byte(metadataString)
	var m Metadata
	if err := xml.Unmarshal(xmlByte, &m); err != nil {
		return nil, err
	}
	return &m, nil
}
