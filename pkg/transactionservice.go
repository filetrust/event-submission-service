package transactionservice

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"log"
	"os"
	"time"

	"github.com/matryer/try"
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

	err := try.Do(func(attempt int) (bool, error) {
		var err error

		if jsonData.Events == nil {
			jsonData, err = CheckExistingFile(filePath)
			if err != nil {
				log.Printf("Failed to write event: Attempt %v, Error: %v", attempt, err)

				if attempt < 5 {
					time.Sleep(time.Duration(attempt) * time.Second) // 5 second wait if more attempts left
				}

				return attempt < 5, err
			}

			properties := PropertiesData{}
			properties.Properties = event

			jsonData.Events = append(jsonData.Events, properties)
		}

		err = WriteEventToFile(filePath, jsonData)
		if err != nil {
			log.Printf("Failed to write event: Attempt %v, Error: %v", attempt, err)

			if attempt < 5 {
				time.Sleep(time.Duration(attempt) * time.Second) // 5 second wait if more attempts left
			}

			return attempt < 5, err
		}

		return attempt < 5, nil
	})
	if err != nil {
		return fmt.Errorf("Unable to upload file: %v", err)
	}

	return nil
}

func WriteEventToFile(filePath string, jsonData MetadataJson) error {
	file, err := json.Marshal(jsonData)
	if err != nil {
		return fmt.Errorf("Unable to marshal json: %v", err)
	}

	err = ioutil.WriteFile(filePath, file, 0644)
	if err != nil {
		return fmt.Errorf("Unable to write file: %v", err)
	}

	return nil
}

func CheckExistingFile(filePath string) (MetadataJson, error) {
	jsonData := MetadataJson{}

	if _, err := os.Stat(filePath); !os.IsNotExist(err) {
		content, err := ioutil.ReadFile(filePath)
		if err != nil {
			return jsonData, fmt.Errorf("Unable to read file: %v", err)
		}

		err = json.Unmarshal(content, &jsonData)
		if err != nil {
			return jsonData, fmt.Errorf("Unable to parse file as json: %v", err)
		}
	}

	return jsonData, nil
}
