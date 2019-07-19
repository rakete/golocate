package main

import (
	//"fmt"
	"log"
	"os"
	"regexp"
	"runtime"
	"sync"
	//"syscall"
	"io/ioutil"
	"sort"
	"strconv"
	"strings"
	"time"

	glib "github.com/gotk3/gotk3/glib"
	gtk "github.com/gotk3/gotk3/gtk"
)

const (
	DEFAULT_DIRECTION  gtk.SortType = gtk.SORT_ASCENDING
	OPPOSITE_DIRECTION gtk.SortType = gtk.SORT_DESCENDING
	DEFAULT_SORT       SortColumn   = SORT_BY_MODTIME
)

func createColumn(title string, id SortColumn) *gtk.TreeViewColumn {
	cellrenderer, err := gtk.CellRendererTextNew()
	if err != nil {
		log.Fatal("Unable to create text cell renderer:", err)
	}

	column, err := gtk.TreeViewColumnNewWithAttribute(title, cellrenderer, "text", int(id))
	if err != nil {
		log.Fatal("Unable to create cell column:", err)
	}

	column.SetSizing(gtk.TREE_VIEW_COLUMN_FIXED)
	column.SetResizable(true)
	column.SetClickable(true)
	column.SetReorderable(true)

	if id == DEFAULT_SORT {
		column.SetSortIndicator(true)
		column.SetSortOrder(DEFAULT_DIRECTION)
	}

	switch id {
	case SORT_BY_NAME:
		column.SetFixedWidth(500)
		column.SetMinWidth(60)
	case SORT_BY_DIR:
		column.SetFixedWidth(800)
		column.SetMinWidth(60)
	case SORT_BY_MODTIME:
		column.SetFixedWidth(200)
		column.SetMinWidth(60)
	case SORT_BY_SIZE:
		column.SetFixedWidth(120)
		column.SetMinWidth(60)
	}

	return column
}

func setupTreeView() (*gtk.TreeView, *gtk.ListStore) {
	treeview, err := gtk.TreeViewNew()
	if err != nil {
		log.Fatal("Unable to create tree view:", err)
	}

	treeview.AppendColumn(createColumn("Name", SORT_BY_NAME))
	treeview.AppendColumn(createColumn("Dir", SORT_BY_DIR))
	treeview.AppendColumn(createColumn("Size", SORT_BY_SIZE))
	treeview.AppendColumn(createColumn("Modification Time", SORT_BY_MODTIME))

	// Creating a list store. This is what holds the data that will be shown on our tree view.
	liststore, err := gtk.ListStoreNew(glib.TYPE_STRING, glib.TYPE_STRING, glib.TYPE_STRING, glib.TYPE_STRING)
	if err != nil {
		log.Fatal("Unable to create list store:", err)
	}
	liststore.SetSortColumnId(gtk.SORT_COLUMN_UNSORTED, gtk.SORT_DESCENDING)
	treeview.SetModel(liststore)
	treeview.SetFixedHeightMode(true)
	treeview.SetHeadersVisible(true)
	treeview.SetHeadersClickable(true)

	return treeview, liststore
}

func setupWindow(application *gtk.Application, treeview *gtk.TreeView, title string) (*gtk.ApplicationWindow, *gtk.ScrolledWindow, *gtk.SearchEntry) {

	header, err := gtk.HeaderBarNew()
	if err != nil {
		log.Fatal("Could not create header bar:", err)
	}
	header.SetShowCloseButton(true)

	mbtn, err := gtk.MenuButtonNew()
	if err != nil {
		log.Fatal("Could not create menu button:", err)
	}

	menu := glib.MenuNew()
	if menu == nil {
		log.Fatal("Could not create menu (nil)")
	}
	menu.Append("Crawl", "app.crawl")
	menu.Append("Quit", "app.quit")

	mbtn.SetMenuModel(&menu.MenuModel)
	header.PackStart(mbtn)

	searchentry, err := gtk.SearchEntryNew()
	if err != nil {
		log.Fatal("Could not create search entry:", err)
	}

	searchentry.SetHAlign(gtk.ALIGN_FILL)
	searchentry.SetHExpand(true)
	searchentry.SetWidthChars(40)

	header.SetCustomTitle(searchentry)

	appwin, err := gtk.ApplicationWindowNew(application)
	if err != nil {
		log.Fatal("Unable to create window:", err)
	}

	appwin.SetTitle(title)
	appwin.SetTitlebar(header)
	appwin.SetPosition(gtk.WIN_POS_CENTER)
	appwin.SetDefaultSize(1700, 1000)

	verticalbox, err := gtk.BoxNew(gtk.ORIENTATION_VERTICAL, 0)
	if err != nil {
		log.Fatal("Unable to create box:", err)
	}

	scrollwin, err := gtk.ScrolledWindowNew(nil, nil)
	if err != nil {
		log.Fatal("unable to create scrolled window:", err)
	}

	appwin.Add(verticalbox)
	scrollwin.Add(treeview)
	verticalbox.PackStart(scrollwin, true, true, 5)
	appwin.ShowAll()

	return appwin, scrollwin, searchentry
}

func addEntry(liststore *gtk.ListStore, entry *FileEntry) gtk.TreeIter {
	sizestring := SizeThreshold(entry.size).String()

	modtime := entry.modtime
	modtimestring := modtime.Format("2006-01-02 15:04:05")

	var iter gtk.TreeIter
	err := liststore.InsertWithValues(&iter, -1,
		[]int{int(SORT_BY_NAME), int(SORT_BY_DIR), int(SORT_BY_SIZE), int(SORT_BY_MODTIME)},
		[]interface{}{entry.name, entry.dir, sizestring, modtimestring})

	if err != nil {
		log.Fatal("Unable to add row:", err)
	}

	return iter
}

func updateEntry(iter *gtk.TreeIter, liststore *gtk.ListStore, entry *FileEntry) {
	sizestring := SizeThreshold(entry.size).String()

	modtime := entry.modtime
	modtimestring := modtime.Format("2006-01-02 15:04:05")

	err := liststore.Set(iter,
		[]int{int(SORT_BY_NAME), int(SORT_BY_DIR), int(SORT_BY_SIZE), int(SORT_BY_MODTIME)},
		[]interface{}{entry.name, entry.dir, sizestring, modtimestring})

	if err != nil {
		log.Fatal("Unable to update row:", err)
	}
}

func instantSort(list *ViewList, oldsort SortColumn, olddirection gtk.SortType, newsort SortColumn, newdirection gtk.SortType, n int) bool {
	ret := false

	var wg sync.WaitGroup
	wg.Add(1)
	glib.IdleAdd(func() {

		listlength := list.store.IterNChildren(nil)
		if listlength < n {

			list.mutex.Lock()

			if oldsort != newsort {
				var directories []string
				direntries := make(map[string][]*FileEntry)
				for _, entry := range list.entries {
					entries, ok := direntries[entry.dir]
					direntries[entry.dir] = append(entries, entry)
					if !ok {
						directories = append(directories, entry.dir)
					}
				}
				list.entries = list.entries[:0]
				sort.Stable(sort.Reverse(sort.StringSlice(directories)))
				for _, dir := range directories {
					entries, _ := direntries[dir]

					sort.Stable(SortedByName(entries))
					switch newsort {
					case SORT_BY_MODTIME:
						sort.Stable(SortedByModTime(entries))
					case SORT_BY_SIZE:
						sort.Stable(SortedBySize(entries))
					}
					list.entries = sortMerge(newsort, list.entries, entries)
				}
			} else if olddirection != newdirection {
				for i := len(list.entries)/2 - 1; i >= 0; i-- {
					opp := len(list.entries) - 1 - i
					list.entries[i], list.entries[opp] = list.entries[opp], list.entries[i]
				}
			}

			i := 0
			iter, valid := list.store.GetIterFirst()
			for valid == true && i < len(list.entries) {
				updateEntry(iter, list.store, list.entries[i])
				valid = list.store.IterNext(iter)
				i += 1
			}

			ret = true

			list.mutex.Unlock()
		}

		wg.Done()
	})
	wg.Wait()

	return ret
}

func instantSearch(list *ViewList, query *regexp.Regexp) int {
	ret := 0
	var wg sync.WaitGroup
	wg.Add(1)
	glib.IdleAdd(func() {
		list.mutex.Lock()

		i := 0
		var newentries []*FileEntry
		var removeindices []int
		iter, valid := list.store.GetIterFirst()
		for iter != nil && valid == true {

			if query == nil || query.MatchString(list.entries[i].name) || query.MatchString(list.entries[i].dir) {
				newentries = append(newentries, list.entries[i])
			} else {
				removeindices = append(removeindices, i)
			}
			valid = list.store.IterNext(iter)

			i += 1
		}

		ret = i

		if len(newentries) > 0 {
			iter := new(gtk.TreeIter)
			for nremoved, index := range removeindices {
				list.store.IterNthChild(iter, nil, index-nremoved)
				list.store.Remove(iter)
			}

			list.entries = make([]*FileEntry, len(newentries))
			copy(list.entries, newentries)

			ret = len(newentries)
		}

		list.mutex.Unlock()

		wg.Done()
	})
	wg.Wait()

	return ret
}

func updateView(cache MatchCaches, bucket Bucket, list *ViewList, sortcolumn SortColumn, direction gtk.SortType, query *regexp.Regexp, n int, abort chan struct{}) {
	if bucket == nil {
		return
	}

	var wg sync.WaitGroup
	wg.Add(1)

	display := func(newentries []*FileEntry) {
		wg.Add(1)
		glib.IdleAdd(func() {
			list.mutex.Lock()
			log.Println("displaying", len(newentries), "entries")

			i := 0
			iter, valid := list.store.GetIterFirst()
			for i < len(newentries) && valid == true {
				updateEntry(iter, list.store, newentries[i])
				valid = list.store.IterNext(iter)
				i += 1
			}

			if i < len(newentries) {
				for _, newentry := range newentries[i:] {
					addEntry(list.store, newentry)
				}
			} else {
				for valid == true {
					list.store.Remove(iter)
					valid = list.store.IterIsValid(iter)
				}
			}

			list.entries = make([]*FileEntry, len(newentries))
			copy(list.entries, newentries)

			list.query <- query
			list.mutex.Unlock()

			wg.Done()
		})
	}

	taken := make(chan *FileEntry)
	var batch []*FileEntry
	aborttake := make(chan struct{})

	go func() {
		for {
			select {
			case <-abort:
				close(aborttake)
				return
			case entry := <-taken:
				if entry == nil {
					close(aborttake)
					display(batch)
					return
				}
				batch = append(batch, entry)
			case <-time.After(1000 * time.Millisecond):
				if (n > 10 && len(batch) > 10) || len(batch) > 0 {
					display(batch)
				}
			}
		}
	}()
	bucket.Node().Take(cache, sortcolumn, direction, query, n, aborttake, taken)
	wg.Done()

	wg.Wait()
}

type ViewList struct {
	store   *gtk.ListStore
	entries []*FileEntry
	mutex   *sync.Mutex
	query   chan *regexp.Regexp
}

type ViewSort struct {
	column    SortColumn
	direction gtk.SortType
}

type ViewControls struct {
	sort       chan ViewSort
	more       chan struct{}
	reset      chan struct{}
	searchterm chan string
}

func Controller(mem ResultMemory, viewcontrols ViewControls, list *ViewList) {
	currentsort := DEFAULT_SORT
	currentdirection := DEFAULT_DIRECTION
	var currentquery *regexp.Regexp
	lastpoll := time.Unix(0, 0)
	inc := 1000
	n := inc
	abort := make(chan struct{})
	maxproc := make(chan struct{}, 1)
	matchcaches := MatchCaches{NewSimpleCache(), NewSimpleCache()}

	for {
		select {
		case <-viewcontrols.more:
			listlength := list.store.IterNChildren(nil)
			if listlength >= n {
				n += inc
				lastpoll = time.Unix(0, 0)
			} else {
				n = inc
			}
		case <-viewcontrols.reset:
			n = inc
		case searchterm := <-viewcontrols.searchterm:
			var query *regexp.Regexp
			var err error
			if len(searchterm) > 0 {
				query, err = regexp.Compile(searchterm)
			}

			if err == nil {
				currentquery = query

				close(abort)
				<-abort
				abort = make(chan struct{})

				listlength := instantSearch(list, currentquery)
				for n > listlength && listlength > inc {
					n -= inc
				}
				matchcaches = MatchCaches{NewSimpleCache(), NewSimpleCache()}
				lastpoll = time.Unix(0, 0)
			}
		case newsort := <-viewcontrols.sort:
			if newsort.column != currentsort || newsort.direction != currentdirection {
				oldsort := currentsort
				olddirection := currentdirection
				currentsort = newsort.column
				currentdirection = newsort.direction

				close(abort)
				<-abort
				abort = make(chan struct{})

				if !instantSort(list, oldsort, olddirection, currentsort, currentdirection, n) {
					lastpoll = time.Unix(0, 0)
				}
			}
		case <-time.After(1000 * time.Millisecond):
		}

		var currentbucket Bucket
		switch currentsort {
		case SORT_BY_NAME:
			currentbucket = mem.byname.(*Node)
		case SORT_BY_DIR:
			currentbucket = mem.bydir.(*Node)
		case SORT_BY_SIZE:
			currentbucket = mem.bysize.(*Node)
		case SORT_BY_MODTIME:
			currentbucket = mem.bymodtime.(*Node)
		}

		if currentbucket.Node().lastchange.After(lastpoll) {
			if len(maxproc) == 0 {
				maxproc <- struct{}{}
				lastpoll = time.Now()
				go func() {
					updateView(matchcaches, currentbucket, list, currentsort, currentdirection, currentquery, n, abort)
					<-maxproc
				}()
			} else {
				lastpoll = time.Unix(0, 0)
			}
		}
	}
}

func createColumnSortToggle(treeview *gtk.TreeView, clickedcolumn int, viewsortchan chan ViewSort, sortcolumn SortColumn) func() {
	return func() {
		for i := 0; i < int(treeview.GetNColumns()); i++ {
			column := treeview.GetColumn(i)

			if i == clickedcolumn {
				firstclick := !column.GetSortIndicator()

				column.SetSortIndicator(true)
				sortdirection := DEFAULT_DIRECTION
				if !firstclick {
					currentdirection := column.GetSortOrder()
					if currentdirection == DEFAULT_DIRECTION {
						sortdirection = OPPOSITE_DIRECTION
					}
				}
				column.SetSortOrder(sortdirection)

				viewsortchan <- ViewSort{sortcolumn, sortdirection}
			} else {
				column.SetSortOrder(DEFAULT_DIRECTION)
				column.SetSortIndicator(false)
			}
		}
	}
}

type Configuration struct {
	cores       int
	directories []string
	maxinotify  int
}

func main() {
	runtime.LockOSThread()

	gtk.Init(nil)

	const appID = "com.github.rakete.golocate"
	application, err := gtk.ApplicationNew(appID, glib.APPLICATION_FLAGS_NONE)
	if err != nil {
		log.Fatal("Could not create application:", err)
	}

	mem := ResultMemory{
		NewNameBucket(),
		NewDirBucket(),
		NewModTimeBucket(),
		NewSizeBucket(),
	}
	config := Configuration{
		cores:       runtime.NumCPU(),
		directories: []string{os.Getenv("HOME"), "/usr", "/var", "/sys", "/opt", "/etc", "/bin", "/sbin"},
		maxinotify:  1024,
	}

	maxinotifybytes, readerr := ioutil.ReadFile("/proc/sys/fs/inotify/max_user_watches")
	if readerr == nil {
		maxinotifystring := strings.TrimSpace(string(maxinotifybytes))

		maxinotifyint64, converr := strconv.ParseUint(maxinotifystring, 10, 32)
		if converr == nil {
			config.maxinotify = int(maxinotifyint64) / 2
		}
	}

	config.maxinotify = 10000

	var wg sync.WaitGroup
	application.Connect("activate", func() {
		treeview, liststore := setupTreeView()
		applicationwin, scrollwin, searchentry := setupWindow(application, treeview, "golocate")
		searchentry.GrabFocus()

		applicationwin.Connect("focus-in-event", func() {
			log.Println("focus-in-event")
		})

		applicationwin.Connect("focus-out-event", func() {
			log.Println("focus-out-event")
		})

		viewcontrols := ViewControls{
			sort:       make(chan ViewSort),
			more:       make(chan struct{}),
			reset:      make(chan struct{}),
			searchterm: make(chan string),
		}

		viewlist := ViewList{
			store:   liststore,
			entries: nil,
			mutex:   new(sync.Mutex),
			query:   make(chan *regexp.Regexp, 1),
		}

		go Controller(mem, viewcontrols, &viewlist)

		crawlernewdirs := make(chan string)
		crawlerfinish := make(chan struct{})
		log.Println("starting Crawl on", config.cores, "cores")
		wg.Add(1)
		go Crawler(&wg, mem, config, crawlernewdirs, viewlist.query, crawlerfinish)

		for i := 0; i < int(treeview.GetNColumns()); i++ {
			column := treeview.GetColumn(i)
			title := column.GetTitle()
			switch title {
			case "Name":
				column.Connect("clicked", createColumnSortToggle(treeview, i, viewcontrols.sort, SORT_BY_NAME))
			case "Dir":
				column.Connect("clicked", createColumnSortToggle(treeview, i, viewcontrols.sort, SORT_BY_DIR))
			case "Size":
				column.Connect("clicked", createColumnSortToggle(treeview, i, viewcontrols.sort, SORT_BY_SIZE))
			case "Modification Time":
				column.Connect("clicked", createColumnSortToggle(treeview, i, viewcontrols.sort, SORT_BY_MODTIME))
			default:
				column.Connect("clicked", func() {
					log.Println("can not sort by", title)
				})
			}
		}

		searchentry.Connect("search-changed", func(search *gtk.SearchEntry) {
			buffer, err := search.GetBuffer()
			if err == nil {
				text, err := buffer.GetText()
				if err == nil {
					viewcontrols.searchterm <- text
				}
			}
		})

		lastupper := -1.0
		adjustment := scrollwin.GetVAdjustment()
		// adjustment.Connect("value-changed", func() {
		// 	upper := adjustment.GetUpper()
		// 	if upper < lastupper {
		// 		lastupper = -1.0
		// 		view.reset <- struct{}{}
		// 	}
		// 	if upper > lastupper && (adjustment.GetValue()+adjustment.GetPageSize()) > (adjustment.GetUpper()/4)*3 {
		// 		lastupper = upper
		// 		view.more <- struct{}{}
		// 	}
		// })

		scrollwin.Connect("edge-reached", func(win *gtk.ScrolledWindow, pos gtk.PositionType) {
			upper := adjustment.GetUpper()
			if upper < lastupper {
				lastupper = -1.0
				viewcontrols.reset <- struct{}{}
			}
			if upper > lastupper && (adjustment.GetValue()+adjustment.GetPageSize()) > (adjustment.GetUpper()/4)*3 {
				lastupper = upper
				viewcontrols.more <- struct{}{}
			}
		})

		aCrawl := glib.SimpleActionNew("crawl", nil)
		aCrawl.Connect("activate", func() {
		})
		application.AddAction(aCrawl)

		aQuit := glib.SimpleActionNew("quit", nil)
		aQuit.Connect("activate", func() {
			close(crawlerfinish)
			<-crawlerfinish
			log.Println("Crawl terminated")
			application.Quit()
		})
		application.AddAction(aQuit)
	})

	os.Exit(application.Run(os.Args))
}
