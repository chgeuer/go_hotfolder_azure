package main

import (
	"context"
	"fmt"
	"net/url"
	"os"
	"time"

	a "github.com/Azure/azure-storage-blob-go/azblob"
)

const (
	maxRetries                                = 5
	retryDelay                                = 1 * time.Second
	timeout                                   = 10 * time.Second
	megaByte                                  = 1 << 20
	defaultBlockSize                          = 50 * megaByte
	environmentVariableNameStorageAccountName = "SAMPLE_STORAGE_ACCOUNT_NAME"
	environmentVariableNameStorageAccountKey  = "SAMPLE_STORAGE_ACCOUNT_KEY"
	containerName                             = "ocirocks3"
	blobName                                  = "20181007-110205-L1016848.jpg"
)

func main() {
	var (
		storageAccountName = os.Getenv(environmentVariableNameStorageAccountName)
		storageAccountKey  = os.Getenv(environmentVariableNameStorageAccountKey)
	)

	sharedKeyCredential, _ := a.NewSharedKeyCredential(storageAccountName, storageAccountKey)
	pipeline := a.NewPipeline(sharedKeyCredential, a.PipelineOptions{
		Retry: a.RetryOptions{
			Policy:     a.RetryPolicyExponential,
			MaxTries:   maxRetries,
			RetryDelay: retryDelay,
		}})

	url, _ := url.Parse(fmt.Sprintf("https://%s.blob.core.windows.net", storageAccountName))
	serviceURL := a.NewServiceURL(*url, pipeline)
	containerURL := serviceURL.NewContainerURL(containerName)
	blobURL := containerURL.NewBlockBlobURL(blobName)
	fmt.Printf("%s\n", blobURL)

	ctx := context.Background()
	if response, e := blobURL.Download(ctx, 0, a.CountToEnd, a.BlobAccessConditions{}, false); e != nil {
		fmt.Printf("Error %s", e)
		return
	} else {
		fmt.Printf("Downloaded %s", response.Body(a.RetryReaderOptions{}))
	}
}
