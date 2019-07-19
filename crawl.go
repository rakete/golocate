package main

import (
	//"fmt"
	"io/ioutil"
	"log"
	"os"
	"path"
	"regexp"
	"sort"
	"sync"
	"time"

	fsnotify "github.com/fsnotify/fsnotify"
	gtk "github.com/gotk3/gotk3/gtk"
	sys "golang.org/x/sys/unix"
)

type FileEntry struct {
	dir     string
	name    string
	modtime time.Time
	size    int64
}

type FilesChannel struct {
	byname    chan SortedByName
	bydir     chan SortedByDir
	bymodtime chan SortedByModTime
	bysize    chan SortedBySize
}

type ResultMemory struct {
	byname    CrawlResult
	bydir     CrawlResult
	bymodtime CrawlResult
	bysize    CrawlResult
}

type Cache interface {
	Test(k string) (bool, bool)
	Put(k string, v bool)
}

type SyncCache sync.Map

func NewSyncCache() *SyncCache {
	return new(SyncCache)
}

func (c *SyncCache) Test(k string) (bool, bool) {
	v, ok := (*sync.Map)(c).Load(k)
	ret, _ := v.(bool)
	return ret, ok
}

func (c *SyncCache) Put(k string, v bool) {
	(*sync.Map)(c).Store(k, v)
}

type SimpleCache map[string]bool

func NewSimpleCache() *SimpleCache {
	empty := SimpleCache(map[string]bool{})
	return &empty
}

func (c *SimpleCache) Test(k string) (bool, bool) {
	v, ok := (*c)[k]
	return v, ok
}

func (c *SimpleCache) Put(k string, v bool) {
	(*c)[k] = v
}

type MatchCaches struct {
	dirs  Cache
	names Cache
}

func testMatchCaches(dircache Cache, namecache Cache, entry *FileEntry, query *regexp.Regexp) (bool, bool) {
	matchedname, knownname, matcheddir, knowndir := true, true, false, false
	if query != nil {
		matchedname, knownname = namecache.Test(entry.name)
		matcheddir, knowndir = dircache.Test(entry.dir)

		if !matchedname && !matcheddir {
			if !knownname {
				matchedname = query.MatchString(entry.name)
				namecache.Put(entry.name, matchedname)
			}
			if !knowndir && !matchedname {
				matcheddir = query.MatchString(entry.dir)
				dircache.Put(entry.dir, matcheddir)
			}
		}
	}

	return matchedname, matcheddir
}

type CrawlResult interface {
	Merge(sortcolumn SortColumn, files []*FileEntry)
	Take(cache MatchCaches, sortcolumn SortColumn, direction gtk.SortType, query *regexp.Regexp, n int, abort chan struct{}, results chan *FileEntry)
	Remove(sortcolumn SortColumn, files []*FileEntry)
	NumFiles() int
}

type FileEntries struct {
	queue  []*FileEntry
	sorted []*FileEntry
}

func (entries *FileEntries) Merge(_ SortColumn, files []*FileEntry) {
	entries.queue = append(entries.queue, files...)
}

func (entries *FileEntries) Commit(sortcolumn SortColumn) {
	switch sortcolumn {
	case SORT_BY_NAME:
		sort.Stable(SortedByName(entries.queue))
	case SORT_BY_MODTIME:
		sort.Stable(SortedByModTime(entries.queue))
	case SORT_BY_SIZE:
		sort.Stable(SortedBySize(entries.queue))
	case SORT_BY_DIR:
		sort.Stable(SortedByDir(entries.queue))
	}
	entries.sorted = sortMerge(sortcolumn, entries.sorted, entries.queue)
	entries.queue = nil
}

func (entries *FileEntries) Take(cache MatchCaches, sortcolumn SortColumn, direction gtk.SortType, query *regexp.Regexp, n int, abort chan struct{}, results chan *FileEntry) {
	var indexfunc func(int, int) int
	switch direction {
	case gtk.SORT_ASCENDING:
		indexfunc = func(l, i int) int { return i }
	case gtk.SORT_DESCENDING:
		indexfunc = func(l, j int) int { return l - 1 - j }
	}

	entries.Commit(sortcolumn)

	l := len(entries.sorted)
	if n > l {
		n = l
	}

	numresults := 0
	aborted := false
	var namecache, dircache Cache
	if cache.names != nil {
		namecache = cache.names
	} else {
		empty := SimpleCache(map[string]bool{})
		namecache = &empty
	}

	if cache.dirs != nil {
		dircache = cache.dirs
	} else {
		empty := SimpleCache(map[string]bool{})
		dircache = &empty
	}

sortedloop:
	for i := 0; i < len(entries.sorted); i++ {
		select {
		case <-abort:
			aborted = true
			break sortedloop
		default:

			index := indexfunc(l, i)
			entry := entries.sorted[index]
			matchedname, matcheddir := testMatchCaches(dircache, namecache, entry, query)

			if query == nil || matchedname || matcheddir {
				results <- entry
				numresults += 1
			}

			if numresults >= n {
				break sortedloop
			}
		}

	}

	if !aborted {
		results <- nil
	}
}

func (entries *FileEntries) Remove(dirs []string, files []*FileEntry) {
	entries.Commit(SORT_BY_DIR)
}

func (entries *FileEntries) NumFiles() int {
	return len(entries.queue) + len(entries.sorted)
}

func visit(wg *sync.WaitGroup, config Configuration, watcher *fsnotify.Watcher, maxwatch chan struct{}, maxproc chan struct{}, newdirs chan string, collect FilesChannel, direntry *DirEntry, dir string) {
	relevantage := time.Now().Add(-time.Hour * 24 * 31)

	maxwatch <- struct{}{}
	watcherr := watcher.Add(dir)

	if watcherr != nil {
		<-maxwatch
	} else {

		dirinfo := new(sys.Stat_t)
		staterr := sys.Lstat(dir, dirinfo)

		var infos []os.FileInfo
		var readerr error
		if staterr == nil {
			infos, readerr = ioutil.ReadDir(dir)
		}
		<-maxproc

		if staterr != nil || readerr != nil {
			watcher.Remove(dir)
			<-maxwatch
		} else {
			modtime := time.Unix(dirinfo.Mtim.Sec, dirinfo.Mtim.Nsec)

			addedinotify := true
			if len(maxwatch)+1 >= config.maxinotify || modtime.Before(relevantage) || len(infos) < 1 {
				addedinotify = false
				watcher.Remove(dir)
				<-maxwatch
			}

			wg.Add(4)

			fileentries := make([]*FileEntry, len(infos))
			numfiles := 0

			for _, fileinfo := range infos {
				entrypath := path.Join(dir, fileinfo.Name())
				if fileinfo.IsDir() {
					newdirs <- entrypath
				} else {
					entry := &FileEntry{
						dir:     dir,
						name:    fileinfo.Name(),
						modtime: fileinfo.ModTime(),
						size:    fileinfo.Size(),
					}
					fileentries[numfiles] = entry
					numfiles += 1
				}
			}

			direntry.path = dir
			direntry.modtime = modtime
			direntry.files = fileentries[:numfiles]
			direntry.inotify = addedinotify

			if numfiles > 0 {
				collect.byname <- fileentries[:numfiles]
				collect.bydir <- fileentries[:numfiles]
				collect.bymodtime <- fileentries[:numfiles]
				collect.bysize <- fileentries[:numfiles]
			} else {
				defer func() {
					wg.Done()
					wg.Done()
					wg.Done()
					wg.Done()
				}()
			}
		}
	}

	defer wg.Done()
}

func collectByName(wg *sync.WaitGroup, mem ResultMemory, collect FilesChannel, finish chan struct{}) {
	for {
		select {
		case files := <-collect.byname:
			newbyname := make([]*FileEntry, len(files))
			copy(newbyname, files)

			sort.Stable(SortedByName(newbyname))
			mem.byname.Merge(SORT_BY_NAME, newbyname)

			wg.Done()
		case <-finish:
			return
		}
	}
}

func collectByDir(wg *sync.WaitGroup, mem ResultMemory, collect FilesChannel, finish chan struct{}) {
	for {
		select {
		case files := <-collect.bydir:
			newbydir := make([]*FileEntry, len(files))
			copy(newbydir, files)

			// - should already be sorted by dir
			//sort.Stable(SortedByDir(newbydir))
			mem.bydir.Merge(SORT_BY_DIR, newbydir)

			wg.Done()
		case <-finish:
			return
		}
	}
}

func collectByModTime(wg *sync.WaitGroup, mem ResultMemory, collect FilesChannel, finish chan struct{}) {
	for {
		select {
		case files := <-collect.bymodtime:
			newbymodtime := make([]*FileEntry, len(files))
			copy(newbymodtime, files)

			sort.Stable(SortedByModTime(newbymodtime))
			mem.bymodtime.Merge(SORT_BY_MODTIME, newbymodtime)

			wg.Done()
		case <-finish:
			return
		}
	}
}

func collectBySize(wg *sync.WaitGroup, mem ResultMemory, collect FilesChannel, finish chan struct{}) {
	for {
		select {
		case files := <-collect.bysize:
			newbysize := make([]*FileEntry, len(files))
			copy(newbysize, files)

			sort.Stable(SortedBySize(newbysize))
			mem.bysize.Merge(SORT_BY_SIZE, newbysize)

			wg.Done()
		case <-finish:
			return
		}
	}
}

type DirEntry struct {
	path    string
	modtime time.Time
	files   []*FileEntry
	inotify bool
}

type Events struct {
	name       string
	info       *sys.Stat_t
	timestamps []time.Time
	ops        []fsnotify.Op
}

func queueEvent(eventqueue *sync.Map, name string, info *sys.Stat_t, op fsnotify.Op) {
	timestamp := time.Now()

	events, loaded := eventqueue.LoadOrStore(name, &Events{
		name:       name,
		info:       info,
		timestamps: []time.Time{timestamp},
		ops:        []fsnotify.Op{op},
	})

	if loaded {
		events.(*Events).timestamps = append(events.(*Events).timestamps, timestamp)
		events.(*Events).ops = append(events.(*Events).ops, op)
	}

	// if info == nil {
	// 	fmt.Println("inotify:", name, op)
	// } else {
	// 	fmt.Println("polling:", name, op)
	// }
}

func Crawler(wg *sync.WaitGroup, mem ResultMemory, config Configuration, newdirs chan string, query chan *regexp.Regexp, finish chan struct{}) {
	wg.Add(len(config.directories))

	watcher, err := fsnotify.NewWatcher()
	if err != nil {
		log.Fatal("could not create fsnotify watcher", err)
	}
	defer watcher.Close()

	eventqueue := new(sync.Map)
	go func() {
		for {
			select {
			case <-finish:
				return
			case event := <-watcher.Events:
				queueEvent(eventqueue, event.Name, nil, event.Op)
			case err := <-watcher.Errors:
				log.Println("error:", err)
			}
		}
	}()

	collect := FilesChannel{
		make(chan SortedByName),
		make(chan SortedByDir),
		make(chan SortedByModTime),
		make(chan SortedBySize),
	}

	// inotify:
	// - need to start watching before first visit
	// - after visiting first time I need to decide if I keep watching or remove watcher and poll instead
	// - to make that decision I need to maintain a list of directories sorted by their modtime

	// - inotify events:
	// -- write file, chmod file -> lstat file, remove file, add file
	// -- write dir -> remove dir, remove files, visit dir, (add dir)
	// -- chmod dir -> [lstat dir, remove dir], (add dir) *

	// -- create file -> lstat file, add file *
	//                -> [lstat parent, remove parent], (add parent)
	// -- create dir -> visit dir, (add dir) *
	//               -> [lstat parent, remove parent], (add parent) +
	// -- rename, remove file -> remove file
	//                        -> [lstat parent, remove parent], (add parent) +
	// -- rename, remove dir -> remove dir
	//                       -> [lstat parent, remove parent], (add parent) +

	// - would be easier to just map all ops to files/dirs to just one updates
	// to a dir, so:
	// -- create, write, remove, chmod, rename -> remove and crawl containing dir

	// polling:
	// - occasionally poll all directories that are not watched with inotify
	// - when directory modtime after stored modtime need to remove dir from polling and start watching it with inotify
	// - polling needs to mimic events generated by inotfy:
	// -- if directory modtime different read directory contents
	// --- remove all previous fileentries and visit non-recursively
	// --- check all contained directories, when a directory is not watched by inotify and not polled then it is new, and needs to be visited
	// -- else nothing new was created inside directory
	// --- loop over known files, stat, when changed modtime remove from mem and add as new

	maxwatch := make(chan struct{}, config.maxinotify)
	maxproc := make(chan struct{}, config.cores)

	direntries := make([]*DirEntry, 0, 100000)

	var currentquery *regexp.Regexp
	go func() {
		for {
			select {
			case dir := <-newdirs:
				watcherror := watcher.Add(dir)
				if watcherror != nil {
					//log.Println("could not start watching directory for changes", dir, watcherror)
				} else {
					wg.Add(1)
					maxproc <- struct{}{}

					direntry := &DirEntry{
						inotify: false,
					}

					go visit(wg, config, watcher, maxwatch, maxproc, newdirs, collect, direntry, dir)
					direntries = append(direntries, direntry)
				}

			case currentquery = <-query:
			case <-finish:
				return
			}
		}
	}()

	go collectByName(wg, mem, collect, finish)
	go collectByDir(wg, mem, collect, finish)
	go collectByModTime(wg, mem, collect, finish)
	go collectBySize(wg, mem, collect, finish)

	for _, dir := range config.directories {
		newdirs <- dir
		wg.Done()
	}

	wg.Done()
	wg.Wait()
	log.Println(len(maxwatch))

	const (
		pollparts        = 6
		polldirsoncount  = 5
		pollfilesoncount = 20
	)
	polldirscounter := 0
	pollfilescounter := 0
	offset := 0
	for {
		select {
		case <-finish:
			return
		case <-time.After(1000 * time.Millisecond):
			now := time.Now()

			polldirscounter += 1
			pollfilescounter += 1
			if polldirscounter >= polldirsoncount {
				pollfiles := false
				if pollfilescounter > pollfilesoncount {
					pollfiles = true
				}

				for _, direntry := range direntries {
					if !direntry.inotify {
						dirinfo := new(sys.Stat_t)
						statdirerr := sys.Lstat(direntry.path, dirinfo)

						if statdirerr != nil {
							//fmt.Println("poll found removed dir", direntry.path)
							queueEvent(eventqueue, direntry.path, dirinfo, fsnotify.Remove)
						} else {
							dirmodtime := time.Unix(dirinfo.Mtim.Sec, dirinfo.Mtim.Nsec)
							if dirmodtime.After(direntry.modtime) {
								//fmt.Println("poll found updated dir", direntry.path)
								queueEvent(eventqueue, direntry.path, dirinfo, fsnotify.Write)
							} else if pollfiles && len(direntry.files) > 0 {

								start := offset
								inc := pollparts
								if len(direntry.files) < pollparts {
									start = 0
									inc = 1
								}

							statfilesloop:
								for i := start; i < len(direntry.files); i += inc {
									fileentry := direntry.files[i]

									filepath := path.Join(fileentry.dir, fileentry.name)
									fileinfo := new(sys.Stat_t)
									statfileerr := sys.Lstat(filepath, fileinfo)

									if statfileerr != nil {
										//fmt.Println("poll found removed file:", filepath)
										queueEvent(eventqueue, filepath, fileinfo, fsnotify.Remove)
									} else {
										filemodtime := time.Unix(fileinfo.Mtim.Sec, fileinfo.Mtim.Nsec)
										if filemodtime.After(fileentry.modtime) {
											//fmt.Println("poll found updated file:", filepath, direntry.path)
											queueEvent(eventqueue, direntry.path, dirinfo, fsnotify.Write)
											break statfilesloop
										}
									}
								}
							}
						}
					}
				}

				offset += 1
				if offset >= pollparts {
					offset = 0
				}

				polldirscounter = 0
				if pollfiles {
					pollfilescounter = 0
				}
			}

			currentevents := make([]Events, 0, 100)
			eventqueue.Range(func(name, events interface{}) bool {
				ops := events.(*Events).ops

				if len(ops) > 0 {
					timestamps := events.(*Events).timestamps
					lastchange := events.(*Events).timestamps[len(timestamps)-1]

					if lastchange.Before(now.Add(-time.Millisecond * 500)) {
						currentevents = append(currentevents, *events.(*Events))
						eventqueue.Delete(name)
					}
				}
				return true
			})

			// Create -> UPDATE
			// Write -> UPDATE
			// Chmod -> UPDATE

			// Remove -> REMOVE
			// Rename -> REMOVE
			const (
				UPDATE int = iota
				NEWDIR
				REMOVE
			)

			if len(currentevents) > 0 {
				//fmt.Println(now)
				for _, events := range currentevents {
					//fmt.Print(events.name, ": ", len(events.ops))

					action := UPDATE
					for _, op := range events.ops {
						//fmt.Print(", ", op.String())
						if op == fsnotify.Remove || op == fsnotify.Rename {
							action = REMOVE
						} else {
							action = UPDATE
						}
					}

					switch action {
					case UPDATE:
						//fmt.Println(" -> UPDATE")
					case NEWDIR:
						//fmt.Println(" -> NEWDIR")
					case REMOVE:
						//fmt.Println(" -> REMOVE")
					}
				}
				currentevents = currentevents[:0]
			}
		}
	}
}
