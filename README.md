
# `chgeuer/go_hotfolder_azure` - A hotfolder synchronizer

Purpose of this utility is to run on a VM, monitor a local (hot) folder, and synchronize files between that folder and an Azure storage container. 

## Research pieces

### File system watchers

- [`fsnotify` utilizes golang.org/x/sys rather than syscall from the standard library](https://github.com/fsnotify/fsnotify)
- [`watcher` is a Go package for watching for files or directory changes without using filesystem events](https://github.com/radovskyb/watcher)
- [`filewatcher` is meant to provide abstraction for watching a single file](https://godoc.org/github.com/johnsiilver/golib/filewatcher), and recommends `fsnotify` for folders. 
- [How to detect file changes in Golang (2015)](https://medium.com/@skdomino/watch-this-file-watching-in-go-5b5a247cf71f)
