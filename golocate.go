package main

import (
	"fmt"
	"log"
	"os"
	"os/exec"
	"strings"
	"sync"

	"github.com/go-cmd/cmd"
	"github.com/gotk3/gotk3/glib"
	"gotk3/gtk"
)

func DirToDbName(dir string) string {
	name := strings.Replace(strings.Trim(dir, "/"), "/", "-", -1)
	if len(name) > 0 {
		name = "-" + name
	}
	name = "golocate" + name
	return name
}

func UpdateDbAndLocate(updatedb string, mlocate string, dir string) []string {
	dbname := DirToDbName(dir)

	updatedbCmd := cmd.NewCmd(updatedb, "--require-visibility", "0", "-o", "/home/rakete/mlocate/"+dbname+".db", "-U", dir)
	updatedbStatusChan := updatedbCmd.Start()
	<-updatedbStatusChan

	mlocateCmd := cmd.NewCmd(mlocate, "-d", "/home/rakete/mlocate/"+dbname+".db", "-r", ".*")
	mlocateStatusChan := mlocateCmd.Start()
	<-mlocateStatusChan

	status := mlocateCmd.Status()
	return status.Stdout
}

func Search(liststore *gtk.ListStore) {
	updatedb, updatedbErr := exec.LookPath("updatedb.mlocate")
	if updatedbErr != nil {
		panic(updatedbErr)
	}

	mlocate, mlocateErr := exec.LookPath("mlocate")
	if mlocateErr != nil {
		panic(mlocateErr)
	}

	glib.IdleAdd(liststore.Clear)

	directories := []string{"/home/rakete", "/usr", "/var", "/bin", "/lib", "/sys", "/proc"}

	var wg sync.WaitGroup
	for _, dir := range directories {
		wg.Add(1)
		go func(dir string) {
			stdout := UpdateDbAndLocate(updatedb, mlocate, dir)
			stdoutlength := fmt.Sprintf("%d", len(stdout))
			glib.IdleAdd(AddRow, liststore, dir, stdoutlength)
			defer wg.Done()
		}(dir)
	}

	wg.Wait()

	glib.IdleAdd(func() {
		for i := 0; i < 10000; i++ {
			var iter gtk.TreeIter
			liststore.InsertWithValues(&iter, -1, []int{COLUMN_FILENAME, COLUMN_COUNT}, []interface{}{"InsertWithValues", i})
		}
	})
}

const (
	COLUMN_FILENAME = iota
	COLUMN_COUNT
)

func CreateColumn(title string, id int) *gtk.TreeViewColumn {
	cellRenderer, err := gtk.CellRendererTextNew()
	if err != nil {
		log.Fatal("Unable to create text cell renderer:", err)
	}

	column, err := gtk.TreeViewColumnNewWithAttribute(title, cellRenderer, "text", id)
	if err != nil {
		log.Fatal("Unable to create cell column:", err)
	}

	return column
}

func AddRow(liststore *gtk.ListStore, filename, count string) {
	iter := liststore.Append()

	err := liststore.Set(iter,
		[]int{COLUMN_FILENAME, COLUMN_COUNT},
		[]interface{}{filename, count})

	if err != nil {
		log.Fatal("Unable to add row:", err)
	}
}

func SetupTreeView() (*gtk.TreeView, *gtk.ListStore) {
	treeView, err := gtk.TreeViewNew()
	if err != nil {
		log.Fatal("Unable to create tree view:", err)
	}

	treeView.AppendColumn(CreateColumn("Filename", COLUMN_FILENAME))
	treeView.AppendColumn(CreateColumn("Count", COLUMN_COUNT))

	// Creating a list store. This is what holds the data that will be shown on our tree view.
	liststore, err := gtk.ListStoreNew(glib.TYPE_STRING, glib.TYPE_STRING)
	if err != nil {
		log.Fatal("Unable to create list store:", err)
	}
	liststore.SetSortColumnId(gtk.SORT_COLUMN_UNSORTED, gtk.SORT_DESCENDING)
	treeView.SetModel(liststore)
	treeView.SetFixedHeightMode(true)

	return treeView, liststore
}

func SetupSearchBar() *gtk.SearchBar {
	searchbar, err := gtk.SearchBarNew()
	if err != nil {
		log.Fatal("Could not create search bar:", err)
	}

	return searchbar
}

func SetupWindow(application *gtk.Application, treeview *gtk.TreeView, liststore *gtk.ListStore, searchbar *gtk.SearchBar, title string) {
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
			Search(liststore)
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

func main() {

	const appID = "com.github.rakete.golocate"
	application, err := gtk.ApplicationNew(appID, glib.APPLICATION_FLAGS_NONE)
	if err != nil {
		log.Fatal("Could not create application:", err)
	}

	application.Connect("activate", func() {
		treeview, liststore := SetupTreeView()
		searchbar := SetupSearchBar()

		SetupWindow(application, treeview, liststore, searchbar, "golocate")
	})

	os.Exit(application.Run(os.Args))
}
