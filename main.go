package main

import (
	"fmt"
	"log"
	"os"
	"path"
	"time"

	"github.com/gotk3/gotk3/glib"
	"gotk3/gtk"
)

const (
	COLUMN_FILENAME = iota
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
	treeView, err := gtk.TreeViewNew()
	if err != nil {
		log.Fatal("Unable to create tree view:", err)
	}

	treeView.AppendColumn(createColumn("Filename", COLUMN_FILENAME))
	treeView.AppendColumn(createColumn("Size", COLUMN_SIZE))
	treeView.AppendColumn(createColumn("Modification Time", COLUMN_MODTIME))

	// Creating a list store. This is what holds the data that will be shown on our tree view.
	liststore, err := gtk.ListStoreNew(glib.TYPE_STRING, glib.TYPE_STRING, glib.TYPE_STRING)
	if err != nil {
		log.Fatal("Unable to create list store:", err)
	}
	liststore.SetSortColumnId(gtk.SORT_COLUMN_UNSORTED, gtk.SORT_DESCENDING)
	treeView.SetModel(liststore)
	treeView.SetFixedHeightMode(true)

	return treeView, liststore
}

func setupSearchBar() *gtk.SearchBar {
	searchbar, err := gtk.SearchBarNew()
	if err != nil {
		log.Fatal("Could not create search bar:", err)
	}

	return searchbar
}

func setupWindow(display ResultChannel, application *gtk.Application, treeview *gtk.TreeView, liststore *gtk.ListStore, searchbar *gtk.SearchBar, title string) {
	win, err := gtk.ApplicationWindowNew(application)
	if err != nil {
		log.Fatal("Unable to create window:", err)
	}

	header, err := gtk.HeaderBarNew()
	if err != nil {
		log.Fatal("Could not create header bar:", err)
	}
	header.SetShowCloseButton(true)
	header.SetTitle(title)
	header.SetSubtitle("subtitle")

	mbtn, err := gtk.MenuButtonNew()
	if err != nil {
		log.Fatal("Could not create menu button:", err)
	}

	menu := glib.MenuNew()
	if menu == nil {
		log.Fatal("Could not create menu (nil)")
	}
	menu.Append("Search", "app.search")
	aSearch := glib.SimpleActionNew("search", nil)
	aSearch.Connect("activate", func() {
		go func() {
			glib.IdleAdd(liststore.Clear)
			Search(display, nil)
		}()
	})
	application.AddAction(aSearch)

	menu.Append("Clear", "app.clear")
	aClear := glib.SimpleActionNew("clear", nil)
	aClear.Connect("activate", func() {
		go func() {
			glib.IdleAdd(liststore.Clear)
		}()
	})
	application.AddAction(aClear)

	menu.Append("Quit", "app.quit")
	aQuit := glib.SimpleActionNew("quit", nil)
	aQuit.Connect("activate", func() {
		application.Quit()
	})
	application.AddAction(aQuit)

	mbtn.SetMenuModel(&menu.MenuModel)
	header.PackStart(mbtn)

	win.SetTitle(title)
	win.SetTitlebar(header)
	win.SetPosition(gtk.WIN_POS_MOUSE)
	win.SetPosition(gtk.WIN_POS_CENTER)
	win.SetDefaultSize(800, 600)

	box, err := gtk.BoxNew(gtk.ORIENTATION_VERTICAL, 0)
	if err != nil {
		log.Fatal("Unable to create box:", err)
	}

	scrolledwindow, err := gtk.ScrolledWindowNew(nil, nil)
	if err != nil {
		log.Fatal("unable to create scrolled window:", err)
	}

	win.Add(box)
	box.PackStart(searchbar, false, false, 0)
	scrolledwindow.Add(treeview)
	box.PackStart(scrolledwindow, true, true, 5)
	win.ShowAll()
}

func addEntry(liststore *gtk.ListStore, entry FileEntry) gtk.TreeIter {
	namestring := path.Join(entry.path, entry.fileinfo.Name())
	sizestring := fmt.Sprintf("%d", entry.fileinfo.Size())

	modtime := entry.fileinfo.ModTime()
	modtimestring := modtime.Format("2006-01-02 15:04:05")

	var iter gtk.TreeIter
	err := liststore.InsertWithValues(&iter, -1,
		[]int{COLUMN_FILENAME, COLUMN_SIZE, COLUMN_MODTIME},
		[]interface{}{namestring, sizestring, modtimestring})

	if err != nil {
		log.Fatal("Unable to add row:", err)
	}

	return iter
}

func updateEntry(iter *gtk.TreeIter, liststore *gtk.ListStore, entry FileEntry) {
	namestring := path.Join(entry.path, entry.fileinfo.Name())
	sizestring := fmt.Sprintf("%d", entry.fileinfo.Size())

	modtime := entry.fileinfo.ModTime()
	modtimestring := modtime.Format("2006-01-02 15:04:05")

	err := liststore.Set(iter,
		[]int{COLUMN_FILENAME, COLUMN_SIZE, COLUMN_MODTIME},
		[]interface{}{namestring, sizestring, modtimestring})

	if err != nil {
		log.Fatal("Unable to update row:", err)
	}
}

func updateView(liststore *gtk.ListStore, display ResultChannel, sorttype chan int) {
	var byname, bymodtime, bysize []FileEntry
	currentsort := -1

	go func() {
		for {
			select {
			case <-time.After(1 * time.Second):
				var entries []FileEntry
				switch currentsort {
				case SORT_BY_NAME:
					entries = byname
				case SORT_BY_MODTIME:
					entries = bymodtime
				case SORT_BY_SIZE:
					entries = bysize
				}

				glib.IdleAdd(func() {
					n := 0
					iter, valid := liststore.GetIterFirst()
					for n < len(entries) && valid == true {
						updateEntry(iter, liststore, entries[n])
						valid = liststore.IterNext(iter)
						n += 1
					}
					if n < len(entries) && len(entries) > 10000 {
						for _, entry := range entries[n:10000] {
							addEntry(liststore, entry)
							n += 1
						}
					}
				})
			}
		}
	}()

	for {
		select {
		case currentsort = <-sorttype:
		case byname = <-display.byname:
		case bymodtime = <-display.bymodtime:
		case bysize = <-display.bysize:
		}
	}
}

func main() {
	const appID = "com.github.rakete.golocate"
	application, err := gtk.ApplicationNew(appID, glib.APPLICATION_FLAGS_NONE)
	if err != nil {
		log.Fatal("Could not create application:", err)
	}

	display := ResultChannel{make(chan ByName), make(chan ByModTime), make(chan BySize)}

	var liststore *gtk.ListStore
	var treeview *gtk.TreeView
	var searchbar *gtk.SearchBar
	sorttype := make(chan int)
	application.Connect("activate", func() {
		treeview, liststore = setupTreeView()
		searchbar = setupSearchBar()

		setupWindow(display, application, treeview, liststore, searchbar, "golocate")

		go updateView(liststore, display, sorttype)
		sorttype <- SORT_BY_SIZE
	})

	os.Exit(application.Run(os.Args))
}
