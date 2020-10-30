package uploader

import (
	"bytes"
	"context"
	"fmt"
	"net/url"
	"strings"
	"time"

	"github.com/Azure/azure-pipeline-go/pipeline"
	"github.com/Azure/azure-storage-file-go/azfile"
)

func urlExists(url *url.URL, p pipeline.Pipeline) bool {
	directoryURL := azfile.NewDirectoryURL(*url, p)
	_, err := directoryURL.GetProperties(context.TODO())
	if err != nil {
		return false
	}
	return true
}

func fileExists(fileURL azfile.FileURL, p pipeline.Pipeline) bool {
	_, err := fileURL.GetProperties(context.TODO())
	if err != nil {
		return false
	}
	return true
}

func getPathFromTimestamp(timestamp, fileID string) (string, error) {
	t, err := time.Parse(time.RFC3339, timestamp)
	if err != nil {
		return "", err
	}

	path := fmt.Sprintf("%d/%d/%d/%d/%s", t.Year(), t.Month(), t.Day(), t.Hour(), fileID)

	return path, nil
}

func createPath(path string, baseURL *url.URL, p pipeline.Pipeline) error {
	subDirs := strings.Split(path, "/")
	var buildPath string

	for _, subdir := range subDirs {
		buildPath += subdir
		url, err := url.Parse(fmt.Sprintf("%s/%s", baseURL.String(), buildPath))
		if err != nil {
			return err
		}

		if !urlExists(url, p) {
			dirrectoryURL := azfile.NewDirectoryURL(*url, p)
			_, err := dirrectoryURL.Create(context.TODO(), azfile.Metadata{}, azfile.SMBProperties{})
			if err != nil {
				return err
			}
		}
		buildPath += "/"
	}
	return nil
}

func downloadFile(fileURL azfile.FileURL) ([]byte, error) {
	get, err := fileURL.Download(context.TODO(), 0, azfile.CountToEnd, false)
	if err != nil {
		return nil, err
	}

	fileData := &bytes.Buffer{}
	retryReader := get.Body(azfile.RetryReaderOptions{})
	defer retryReader.Close()

	_, err = fileData.ReadFrom(retryReader)
	if err != nil {
		return nil, err
	}

	return fileData.Bytes(), nil
}
