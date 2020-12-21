package transactionservice

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"os"
)

type Args struct {
	Path string
}

type MetadataJson struct {
	Events []PropertiesData
}

type PropertiesData struct {
	Properties map[string]interface{}
}

func (ua Args) WriteAnalysisReport(analysisReport string) error {
	if _, err := os.Stat(ua.Path); os.IsNotExist(err) {
		os.MkdirAll(ua.Path, os.ModePerm)
	}

	filePath := fmt.Sprintf("%s/report.xml", ua.Path)

	if _, err := os.Stat(filePath); os.IsNotExist(err) {
		f, err := os.Create(filePath)
		if err != nil {
			return fmt.Errorf("Unable to create file: %v", err)
		}
		l, err := f.WriteString(analysisReport)
		if err != nil {
			f.Close()
			return fmt.Errorf("Unable to create file: %v", err)
		}
		fmt.Println(l, "bytes written successfully")
		err = f.Close()
		if err != nil {
			fmt.Println(err)
			return fmt.Errorf("Unable to create file: %v", err)
		}
	}

	return nil
}

func (ua Args) WriteTransactionEvent(event map[string]interface{}) error {
	if _, err := os.Stat(ua.Path); os.IsNotExist(err) {
		os.MkdirAll(ua.Path, os.ModePerm)
	}

	filePath := fmt.Sprintf("%s/metadata.json", ua.Path)

	jsonData := MetadataJson{}

	if _, err := os.Stat(filePath); !os.IsNotExist(err) {
		content, err := ioutil.ReadFile(filePath)
		if err != nil {
			return fmt.Errorf("Unable to parse file as json: %v", err)
		}

		err = json.Unmarshal(content, &jsonData)
		if err != nil {
			return fmt.Errorf("Unable to parse file as json: %v", err)
		}
	}

	properties := PropertiesData{}
	properties.Properties = event

	jsonData.Events = append(jsonData.Events, properties)

	file, err := json.Marshal(jsonData)

	err = ioutil.WriteFile(filePath, file, 0644)
	if err != nil {
		return fmt.Errorf("Unable to upload file: %v", err)
	}

	return nil
}
