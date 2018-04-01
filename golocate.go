package main

import (
	"fmt"
	"log"
	"os"
	//"path"
	"regexp"
	"runtime"
	"sync"
	"time"

	"github.com/gotk3/gotk3/gdk"
	"github.com/gotk3/gotk3/glib"
	"github.com/gotk3/gotk3/gtk"
)

const (
	COLUMN_NAME = iota
	COLUMN_PATH
	COLUMN_SIZE
	COLUMN_MODTIME
)

func createColumn(title string, id int) *gtk.TreeViewColumn {
	cellRenderer, err := gtk.CellRendererTextNew()
	if err != nil {
		log.Fatal("Unable to create text cell renderer:", err)
	}

	column, err := gtk.TreeViewColumnNewWithAttribute(title, cellRenderer, "text", id)
	if err != nil {
		log.Fatal("Unable to create cell column:", err)
	}

	column.SetSizing(gtk.TREE_VIEW_COLUMN_FIXED)
	column.SetResizable(true)
	column.SetClickable(true)
	column.SetReorderable(true)
	column.SetSortIndicator(true)

	return column
}

func setupTreeView() (*gtk.TreeView, *gtk.ListStore) {
	treeview, err := gtk.TreeViewNew()
	if err != nil {
		log.Fatal("Unable to create tree view:", err)
	}

	treeview.AppendColumn(createColumn("Name", COLUMN_NAME))
	treeview.AppendColumn(createColumn("Path", COLUMN_PATH))
	treeview.AppendColumn(createColumn("Size", COLUMN_SIZE))
	treeview.AppendColumn(createColumn("Modification Time", COLUMN_MODTIME))

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

func setupSearchBar() (*gtk.SearchBar, *gtk.SearchEntry) {
	searchbar, err := gtk.SearchBarNew()
	if err != nil {
		log.Fatal("Could not create search bar:", err)
	}
	searchbar.SetSearchMode(true)

	horizontalbox, err := gtk.BoxNew(gtk.ORIENTATION_HORIZONTAL, 0)
	if err != nil {
		log.Fatal("Unable to create box:", err)
	}

	searchentry, err := gtk.SearchEntryNew()
	if err != nil {
		log.Fatal("Could not create search entry:", err)
	}
	searchentry.SetSizeRequest(400, 0)

	searchbar.Add(horizontalbox)
	horizontalbox.PackStart(searchentry, true, true, 0)
	searchbar.ConnectEntry(searchentry)

	return searchbar, searchentry
}

func setupWindow(application *gtk.Application, treeview *gtk.TreeView, searchbar *gtk.SearchBar, title string) *gtk.ApplicationWindow {
	win, err := gtk.ApplicationWindowNew(application)
	if err != nil {
		log.Fatal("Unable to create window:", err)
	}

	// win, err := gtk.WindowNew(gtk.WINDOW_TOPLEVEL)
	// if err != nil {
	// 	log.Fatal("Unable to create window:", err)
	// }
	// win.Connect("destroy", func() {
	// 	gtk.MainQuit()
	// })
	// application.AddWindow(win)

	header, err := gtk.HeaderBarNew()
	if err != nil {
		log.Fatal("Could not create header bar:", err)
	}
	header.SetShowCloseButton(true)
	header.SetTitle(title)
	header.SetSubtitle("finding files with fearless concurrency")

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

	win.SetTitle(title)
	win.SetTitlebar(header)
	win.SetPosition(gtk.WIN_POS_MOUSE)
	win.SetPosition(gtk.WIN_POS_CENTER)
	win.SetDefaultSize(800, 600)

	verticalbox, err := gtk.BoxNew(gtk.ORIENTATION_VERTICAL, 0)
	if err != nil {
		log.Fatal("Unable to create box:", err)
	}

	scrolledwindow, err := gtk.ScrolledWindowNew(nil, nil)
	if err != nil {
		log.Fatal("unable to create scrolled window:", err)
	}

	win.Add(verticalbox)
	verticalbox.PackStart(searchbar, false, false, 0)
	scrolledwindow.Add(treeview)
	verticalbox.PackStart(scrolledwindow, true, true, 5)
	win.ShowAll()

	return win
}

func addEntry(liststore *gtk.ListStore, entry *FileEntry) gtk.TreeIter {
	sizestring := fmt.Sprintf("%d", entry.size)

	modtime := entry.modtime
	modtimestring := modtime.Format("2006-01-02 15:04:05")

	var iter gtk.TreeIter
	err := liststore.InsertWithValues(&iter, -1,
		[]int{COLUMN_NAME, COLUMN_PATH, COLUMN_SIZE, COLUMN_MODTIME},
		[]interface{}{entry.name, entry.path, sizestring, modtimestring})

	if err != nil {
		log.Fatal("Unable to add row:", err)
	}

	return iter
}

func updateEntry(iter *gtk.TreeIter, liststore *gtk.ListStore, entry *FileEntry) {
	sizestring := fmt.Sprintf("%d", entry.size)

	modtime := entry.modtime
	modtimestring := modtime.Format("2006-01-02 15:04:05")

	err := liststore.Set(iter,
		[]int{COLUMN_NAME, COLUMN_PATH, COLUMN_SIZE, COLUMN_MODTIME},
		[]interface{}{entry.name, entry.path, sizestring, modtimestring})

	if err != nil {
		log.Fatal("Unable to update row:", err)
	}
}

func updateList(bucket Bucket, liststore *gtk.ListStore, sorttype, direction int, query *regexp.Regexp, n int) {
	if bucket == nil {
		return
	}

	var entries []*FileEntry
	entries = bucket.Node().Take(sorttype, direction, nil, n)

	glib.IdleAdd(func() {
		i := 0
		iter, valid := liststore.GetIterFirst()
		for i < len(entries) && valid == true {
			updateEntry(iter, liststore, entries[i])
			valid = liststore.IterNext(iter)
			i += 1
		}
		if i < len(entries) {
			for _, entry := range entries {
				addEntry(liststore, entry)
				i += 1
			}
		}
	})
}

func Controller(liststore *gtk.ListStore, display DisplayChannel, sorttype chan int) {
	var byname, bysize, bymodtime Bucket
	currentsort := -1
	currentdirection := DIRECTION_DESCENDING

	go func() {
		for {
			select {
			case newsorttype := <-sorttype:
				currentsort = newsorttype
				if currentsort == newsorttype && currentdirection == DIRECTION_ASCENDING {
					currentdirection = DIRECTION_DESCENDING
				} else {
					currentdirection = DIRECTION_ASCENDING
				}
			case <-time.After(1 * time.Second):
			}

			var currentbucket Bucket
			switch currentsort {
			case SORT_BY_NAME:
				currentbucket = byname
			case SORT_BY_SIZE:
				currentbucket = bysize
			case SORT_BY_MODTIME:
				currentbucket = bymodtime
			}
			updateList(currentbucket, liststore, currentsort, currentdirection, nil, 100)
		}
	}()

	for {
		select {
		case bucket := <-display.byname:
			byname = bucket.(*Node)
		case bucket := <-display.bymodtime:
			bymodtime = bucket.(*Node)
		case bucket := <-display.bysize:
			bysize = bucket.(*Node)
		}
	}
}

func makeColumnSortToggle(treeview *gtk.TreeView, clickedcolumn int, sorttypechan chan int, sorttype int) func() {
	return func() {
		sorttypechan <- sorttype

		for i := 0; i < int(treeview.GetNColumns()); i++ {
			column := treeview.GetColumn(i)

			if i == clickedcolumn {
				direction := column.GetSortOrder()
				if direction == gtk.GTK_SORT_ASCENDING {
					column.SetSortOrder(gtk.GTK_SORT_DESCENDING)
				} else {
					column.SetSortOrder(gtk.GTK_SORT_ASCENDING)
				}
			} else {
				column.SetSortOrder(gtk.GTK_SORT_ASCENDING)
			}

		}

	}
}

func main() {
	gtk.Init(nil)

	const appID = "com.github.rakete.golocate"
	application, err := gtk.ApplicationNew(appID, glib.APPLICATION_FLAGS_NONE)
	if err != nil {
		log.Fatal("Could not create application:", err)
	}

	directories := []string{os.Getenv("HOME"), "/usr", "/var", "/sys", "/opt", "/etc", "/bin", "/sbin"}

	mem := ResultMemory{
		NewNameBucket(),
		NewModTimeBucket(),
		NewSizeBucket(),
	}
	display := DisplayChannel{make(chan CrawlResult), make(chan CrawlResult), make(chan CrawlResult)}
	newdirs := make(chan string)
	finish := make(chan struct{})
	cores := runtime.NumCPU()

	var liststore *gtk.ListStore
	var treeview *gtk.TreeView
	var searchbar *gtk.SearchBar
	var searchentry *gtk.SearchEntry
	sorttypechan := make(chan int)

	var wg sync.WaitGroup
	application.Connect("activate", func() {
		treeview, liststore = setupTreeView()
		searchbar, searchentry = setupSearchBar()

		go Controller(liststore, display, sorttypechan)
		sorttypechan <- SORT_BY_SIZE

		for i := 0; i < int(treeview.GetNColumns()); i++ {
			column := treeview.GetColumn(i)
			title := column.GetTitle()
			switch title {
			case "Name":
				column.Connect("clicked", makeColumnSortToggle(treeview, i, sorttypechan, SORT_BY_NAME))
			case "Size":
				column.Connect("clicked", makeColumnSortToggle(treeview, i, sorttypechan, SORT_BY_SIZE))
			case "Modification Time":
				column.Connect("clicked", makeColumnSortToggle(treeview, i, sorttypechan, SORT_BY_MODTIME))
			default:
				column.Connect("clicked", func() {
					log.Println("can not sort by", title)
				})
			}
		}

		searchentry.Connect("search-changed", func() {
			log.Println("ping")
		})

		wg.Add(1)
		go Crawler(&wg, cores, mem, display, newdirs, finish)
		log.Println("starting Crawl on", cores, "cores")
		for _, dir := range directories {
			newdirs <- dir
		}

		win := setupWindow(application, treeview, searchbar, "golocate")
		win.Window.Connect("key-press-event", func(win *gtk.ApplicationWindow, ev *gdk.Event) {
			searchbar.HandleEvent(ev)
		})

		aCrawl := glib.SimpleActionNew("crawl", nil)
		aCrawl.Connect("activate", func() {
			go func() {
				for _, dir := range directories {
					newdirs <- dir
				}
			}()
		})
		application.AddAction(aCrawl)

		aQuit := glib.SimpleActionNew("quit", nil)
		aQuit.Connect("activate", func() {
			close(finish)
			<-finish
			log.Println("Crawl terminated")
			application.Quit()
		})
		application.AddAction(aQuit)
	})

	os.Exit(application.Run(os.Args))
}
