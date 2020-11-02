package uploader

import (
	"context"
	"encoding/json"
	"fmt"
	"net/url"

	"github.com/Azure/azure-pipeline-go/pipeline"
	"github.com/Azure/azure-storage-file-go/azfile"
)

type UploaderArgs struct {
	AccountName string
	AccountKey  string
	BaseURL     *url.URL
	Path        string
	Pipeline    pipeline.Pipeline
}

type MetadataJson struct {
	Events []interface{}
}

func (ua *UploaderArgs) GetPipeline() error {
	credential, err := azfile.NewSharedKeyCredential(ua.AccountName, ua.AccountKey)
	if err != nil {
		return fmt.Errorf("Unable to create Share Key Credential: %v", err)
	}

	p := azfile.NewPipeline(credential, azfile.PipelineOptions{})

	ua.Pipeline = p

	return nil
}

func (ua *UploaderArgs) GetPaths(timestamp, fileID string) error {
	baseURL, err := url.Parse(fmt.Sprintf("https://%s.file.core.windows.net/transactions", ua.AccountName))
	if err != nil {
		return fmt.Errorf("Unable to parse base url: %v", err)
	}

	path, err := getPathFromTimestamp(timestamp, fileID)
	if err != nil {
		return fmt.Errorf("Unable to generate path from timestamp: %v", err)
	}

	ua.BaseURL = baseURL
	ua.Path = path

	return nil
}

func (ua UploaderArgs) UploadAnalysisReport(analysisReport string) error {
	url, err := url.Parse(fmt.Sprintf("%s/%s/report.json", ua.BaseURL.String(), ua.Path))
	if err != nil {
		return fmt.Errorf("Unable to parse file url: %v", err)
	}

	fileURL := azfile.NewFileURL(*url, ua.Pipeline)

	if !fileExists(fileURL, ua.Pipeline) {
		err := createPath(ua.Path, ua.BaseURL, ua.Pipeline)
		if err != nil {
			return fmt.Errorf("Unable to create path: %v", err)
		}

		err = azfile.UploadBufferToAzureFile(context.TODO(), []byte(analysisReport), fileURL, azfile.UploadToAzureFileOptions{})
		if err != nil {
			return fmt.Errorf("Unable to upload file: %v", err)
		}
	}

	return nil
}

func (ua UploaderArgs) UploadTransactionEvent(event map[string]interface{}) error {
	url, err := url.Parse(fmt.Sprintf("%s/%s/metadata.json", ua.BaseURL.String(), ua.Path))
	if err != nil {
		return fmt.Errorf("Unable to parse file url: %v", err)
	}

	fileURL := azfile.NewFileURL(*url, ua.Pipeline)

	jsonData := MetadataJson{}

	if !fileExists(fileURL, ua.Pipeline) {
		err := createPath(ua.Path, ua.BaseURL, ua.Pipeline)
		if err != nil {
			return fmt.Errorf("Unable to create path: %v", err)
		}
	} else {
		fileData, err := downloadFile(fileURL)
		if err != nil {
			return fmt.Errorf("Unable to download file: %v", err)
		}

		err = json.Unmarshal(fileData, &jsonData)
		if err != nil {
			return fmt.Errorf("Unable to parse file as json: %v", err)
		}
	}

	jsonData.Events = append(jsonData.Events, event)

	file, err := json.Marshal(jsonData)

	err = azfile.UploadBufferToAzureFile(context.TODO(), file, fileURL, azfile.UploadToAzureFileOptions{})
	if err != nil {
		return fmt.Errorf("Unable to upload file: %v", err)
	}

	return nil
}
