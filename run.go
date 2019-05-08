package main

import (
	"context"
	"fmt"
	"io"
	"log"
	"net/url"
	"os"
	"path/filepath"
	"time"

	a "github.com/Azure/azure-storage-blob-go/azblob"
	"github.com/fsnotify/fsnotify"
)

const (
	maxRetries                                = 5
	retryDelay                                = 1 * time.Second
	timeout                                   = 10 * time.Second
	megaByte                                  = 1 << 20
	defaultBlockSize                          = 50 * megaByte
	environmentVariableNameStorageAccountName = "SAMPLE_STORAGE_ACCOUNT_NAME"
	environmentVariableNameStorageAccountKey  = "SAMPLE_STORAGE_ACCOUNT_KEY"
)

func main() {
	watcher, err := fsnotify.NewWatcher()
	if err != nil {
		fmt.Println("ERROR", err)
	}
	defer watcher.Close()

	watch := func(path string, fi os.FileInfo, err error) error {
		if fi.Mode().IsDir() {
			return watcher.Add(path)
		}
		return nil
	}

	watchFolder := "."

	if err := filepath.Walk(watchFolder, watch); err != nil {
		fmt.Println("ERROR", err)
	}
	done := make(chan bool)

	go func() {
		for {
			select {
			// watch for events
			case event := <-watcher.Events:
				fmt.Printf("EVENT! %s %s\n", event.Name, event.Op)

				// watch for errors
			case err := <-watcher.Errors:
				fmt.Println("ERROR", err)
			}
		}
	}()

	var (
		storageAccountName = os.Getenv(environmentVariableNameStorageAccountName)
		storageAccountKey  = os.Getenv(environmentVariableNameStorageAccountKey)
		containerName      = "ocirocks3"
		blobName           = "20181007-110205-L1016848.jpg"
	)

	ctx := context.Background()
	if e := download(ctx, storageAccountName, storageAccountKey, containerName, blobName, blobName); e != nil {
		log.Fatal(e)
	}

	<-done
}

func download(ctx context.Context, accountName, accountKey, container, blobName, fileName string) error {
	sharedKeyCredential, e := a.NewSharedKeyCredential(accountName, accountKey)
	if e != nil {
		return e
	}
	pipeline := a.NewPipeline(sharedKeyCredential, a.PipelineOptions{
		Retry: a.RetryOptions{
			Policy:     a.RetryPolicyExponential,
			MaxTries:   maxRetries,
			RetryDelay: retryDelay,
		}})

	url, e := url.Parse(fmt.Sprintf("https://%s.blob.core.windows.net", accountName))

	serviceURL := a.NewServiceURL(*url, pipeline)
	containerURL := serviceURL.NewContainerURL(container)
	blobURL := containerURL.NewBlockBlobURL(blobName)
	if e != nil {
		return e
	}

	response, e := blobURL.Download(ctx, 0, a.CountToEnd, a.BlobAccessConditions{}, false)
	if e != nil {
		return e
	}

	body := response.Body(a.RetryReaderOptions{})
	file, e := os.OpenFile(fileName, os.O_RDWR|os.O_CREATE, 0755)
	if e != nil {
		return e
	}

	defer file.Close()
	io.Copy(file, body)

	return nil
}
