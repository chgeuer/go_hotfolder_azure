package main

import (
	"context"
	"crypto/md5"
	"encoding/base64"
	"fmt"
	"io"
	"sync"

	// "log"
	// "net/url"
	"os"
	"os/exec"

	"path/filepath"
	"strconv"
	"strings"
	"time"

	a "github.com/Azure/azure-storage-blob-go/azblob"
	"github.com/fsnotify/fsnotify"
	"github.com/mattetti/filebuffer"
	"github.com/pkg/errors"
	"github.com/reactivex/rxgo"
	"github.com/reactivex/rxgo/handlers"
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

type fileSystemEvent struct {
	path      string
	operation fsnotify.Op
	isDir     bool
	exists    bool
	time      time.Time
}

func (fse fileSystemEvent) DirectoryCreated() bool {
	return fse.exists && fse.isDir && fse.operation == fsnotify.Create
}

func (fse fileSystemEvent) Removed() bool {
	return !fse.exists && fse.operation == fsnotify.Remove
}

func existsIsDir(path string) (exists bool, isDir bool, err error) {
	fileInfo, err := os.Stat(path)
	if err == nil {
		exists = true
		isDir = fileInfo.Mode().IsDir()
		return
	}

	exists = os.IsNotExist(err)
	isDir = false
	return
}

func getObervableFsNotifyWatcher(watchFolder string) (rxgo.Observable, error) {
	basePath, _ := filepath.Abs(watchFolder)

	fsWatcher, e := fsnotify.NewWatcher()
	if e != nil {
		return nil, e
	}

	var wg sync.WaitGroup
	wg.Add(1)
	go func() {
		wg.Wait()
		fsWatcher.Close()
	}()

	watch := func(path string, fi os.FileInfo, err error) error {
		if fi.Mode().IsDir() {
			path, _ = filepath.Abs(path)
			return fsWatcher.Add(path)
		}
		return nil
	}

	if e := filepath.Walk(basePath, watch); e != nil {
		return nil, e
	}

	fileSystemEventsChannel := make(chan interface{})
	source := rxgo.FromEventSource(fileSystemEventsChannel)
	source.Subscribe(rxgo.NewObserver(
		handlers.NextFunc(func(item interface{}) {
			fse := item.(fileSystemEvent)
			if fse.DirectoryCreated() {
				fsWatcher.Add(fse.path)
			}
			if fse.Removed() {
				fsWatcher.Remove(fse.path)
			}
		}),
		handlers.ErrFunc(func(err error) {}),
		handlers.DoneFunc(func() { wg.Done() }),
	))

	go func() {
		for {
			select {
			case event := <-fsWatcher.Events:
				absPath, _ := filepath.Abs(event.Name)
				relPath, _ := filepath.Rel(basePath, absPath)
				exists, isDir, _ := existsIsDir(absPath)

				fileSystemEventsChannel <- fileSystemEvent{
					path:      relPath,
					operation: event.Op,
					isDir:     isDir,
					exists:    exists,
					time:      time.Now(),
				}
			case err := <-fsWatcher.Errors:
				fileSystemEventsChannel <- err
			}
		}
	}()

	return source, nil
}

func main() {
	fmt.Println("Running")

	fileSystemChangeObservable, e := getObervableFsNotifyWatcher(".")
	if e != nil {
		fmt.Println(e)
		return
	}

	fileSystemChangeObservable.Filter(func(item interface{}) bool {
		fse := item.(fileSystemEvent)
		return fse.operation == fsnotify.Create || fse.operation == fsnotify.Remove
	}).Map(func(item interface{}) interface{} {
		fse := item.(fileSystemEvent)
		return fmt.Sprintf("Next %s %v\n", fse.path, fse.operation.String())
	}).Subscribe(rxgo.NewObserver(
		handlers.NextFunc(func(item interface{}) {
			fmt.Printf(item.(string))
		}),
		handlers.ErrFunc(func(err error) {
			fmt.Printf("Error %v\n", err)
		}),
		handlers.DoneFunc(func() {
			fmt.Println("Done")
		}),
	))

	<-make(chan interface{})

	// for {
	// 	filename := "1.mkv"
	// 	if isLocked, pid := fileIsLockedByOtherProcess(filename); isLocked {
	// 		fmt.Printf("%s locked by %d\n", filename, pid)
	// 	} else {
	// 		fmt.Printf("%s not locked\n", filename)
	// 	}
	// }

	// var (
	// 	storageAccountName = os.Getenv(environmentVariableNameStorageAccountName)
	// 	storageAccountKey  = os.Getenv(environmentVariableNameStorageAccountKey)
	// 	containerName      = "ocirocks3"
	// 	blobName           = "20181007-110205-L1016848.jpg"
	// )

	// sharedKeyCredential, e := a.NewSharedKeyCredential(storageAccountName, storageAccountKey)
	// if e != nil {
	// 	log.Fatal(e)
	// 	return
	// }
	// pipeline := a.NewPipeline(sharedKeyCredential, a.PipelineOptions{
	// 	Retry: a.RetryOptions{
	// 		Policy:     a.RetryPolicyExponential,
	// 		MaxTries:   maxRetries,
	// 		RetryDelay: retryDelay,
	// 	}})

	// url, e := url.Parse(fmt.Sprintf("https://%s.blob.core.windows.net", storageAccountName))
	// if e != nil {
	// 	log.Fatal(e)
	// 	return
	// }

	// serviceURL := a.NewServiceURL(*url, pipeline)
	// containerURL := serviceURL.NewContainerURL(containerName)
	// blobURL := containerURL.NewBlockBlobURL(blobName)

	// ctx := context.Background()

	// fmt.Println("Start Download")
	// if e := download(ctx, blobURL, blobName); e != nil {
	// 	log.Fatal(e)
	// 	return
	// }

	// fmt.Println("Start Upload")
	// if e := upload(ctx, blobURL, blobName); e != nil {
	// 	log.Fatal(e)
	// }

	// fmt.Println("Done")
}

func download(ctx context.Context, blobURL a.BlockBlobURL, fileName string) error {
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

func upload(ctx context.Context, blobURL a.BlockBlobURL, fileName string) error {
	file, e := os.OpenFile(fileName, os.O_RDWR|os.O_CREATE, 0755)
	if e != nil {
		return e
	}
	defer file.Close()

	fileInfo, e := file.Stat()
	if e != nil {
		return e
	}
	sourceContentLength := fileInfo.Size()

	overallMD5 := md5.New()
	numberOfBlocks, e := getNumberOfBlocks(sourceContentLength, defaultBlockSize)
	if e != nil {
		return e
	}

	uncommittedBlocksList := make([]string, numberOfBlocks)
	for i := 0; i < numberOfBlocks; i++ {
		uncommittedBlocksList[i] = blockIDfromIndex(i)
	}
	data := make([]byte, defaultBlockSize)

	for i := 0; i <= numberOfBlocks; i++ {
		expectedByteCount := func() int {
			if i < numberOfBlocks-1 {
				return defaultBlockSize
			}
			return int(sourceContentLength % int64(defaultBlockSize))
		}()

		numBytesRead, e := io.ReadAtLeast(file, data, expectedByteCount)

		if i == numberOfBlocks && numBytesRead == 0 && e == io.EOF {
			// now we have reached EOF
			break
		} else if e != nil {
			return e
		}

		bytesToUpload := data[:numBytesRead]
		cloned := append(bytesToUpload[:0:0], bytesToUpload...)
		overallMD5.Write(cloned)
		if putBlockErr := uploadSingleBlock(ctx, blobURL, cloned, i); putBlockErr != nil {
			return putBlockErr
		}
	}

	if _, putBlockListErr := blobURL.CommitBlockList(ctx, uncommittedBlocksList, a.BlobHTTPHeaders{ContentMD5: overallMD5.Sum(nil)}, a.Metadata{}, a.BlobAccessConditions{}); putBlockListErr != nil {
		return putBlockListErr
	}

	return nil
}

func delete(ctx context.Context, blobURL a.BlockBlobURL) error {
	_, e := blobURL.Delete(ctx, a.DeleteSnapshotsOptionInclude, a.BlobAccessConditions{})
	if e != nil {
		if se, ok := e.(a.StorageError); ok && se.ServiceCode() == a.ServiceCodeBlobNotFound {
			// Blob was already deleted
			return nil
		}
		return e
	}
	return nil
}

func blockIDfromIndex(i int) string {
	return base64.StdEncoding.EncodeToString([]byte(fmt.Sprintf("%05d", i)))
}

func uploadSingleBlock(ctx context.Context, temporaryBlob a.BlockBlobURL, bytesToUpload []byte, i int) error {
	blockID := blockIDfromIndex(i)

	checkSumMD5 := md5.New()
	checkSumMD5.Write(bytesToUpload)
	blockMD5bytes := checkSumMD5.Sum(nil)
	body := filebuffer.New(bytesToUpload)
	_, err := temporaryBlob.StageBlock(ctx, blockID, body, a.LeaseAccessConditions{}, blockMD5bytes)
	return err
}

func getNumberOfBlocks(inputLength int64, blockSize int) (int, error) {
	fullyFilledBlocks := int(inputLength / int64(blockSize))
	hasPartiallyFilledBlock := (inputLength % int64(blockSize)) > 0
	var numberOfBlocks int
	if hasPartiallyFilledBlock {
		numberOfBlocks = fullyFilledBlocks + 1
	} else {
		numberOfBlocks = fullyFilledBlocks
	}
	if numberOfBlocks > a.BlockBlobMaxBlocks {
		return -1, errors.Errorf("BlockBlob cannot have more than %d blocks. File size %v bytes, block size %d", a.BlockBlobMaxBlocks, inputLength, blockSize)
	}
	return numberOfBlocks, nil
}

func fileIsLockedByOtherProcess(filename string) (bool, int64) {
	if _, e := os.Stat(filename); e != nil {
		return false, -1
	}

	const command = "lsof"
	args := []string{"-t", filename}
	output, e := exec.Command(command, args...).Output()
	if e != nil {
		return false, -1
	}

	o := strings.TrimSpace(string(output))
	pid, e := strconv.ParseInt(o, 10, 64)
	if e != nil {
		return false, -1
	}

	return true, pid
}
