package metadata

import "encoding/json"

type Metadata struct {
	ChannelId    string
	RotaName     string
	StartOfShift string
	EndOfShift   string
}

func GenerateCommandMetadata(channelId string, rotaName string, startOfShiftTime string, endOfShiftTime string) (string, error) {
	metadata := Metadata{
		ChannelId:    channelId,
		RotaName:     rotaName,
		StartOfShift: startOfShiftTime,
		EndOfShift:   endOfShiftTime,
	}
	b, err := json.Marshal(metadata)
	if err != nil {
		return "", err
	}

	return string(b), nil
}

func UnpackCommandMetadata(metadataBlob string) (*Metadata, error) {
	var metadata Metadata
	err := json.Unmarshal([]byte(metadataBlob), &metadata)
	if err != nil {
		return nil, err
	}

	return &metadata, nil
}
