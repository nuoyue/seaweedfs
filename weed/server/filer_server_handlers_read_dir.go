package weed_server

import (
	"net/http"
	"strconv"
	"strings"

	"github.com/chrislusf/seaweedfs/weed/filer2"
	"github.com/chrislusf/seaweedfs/weed/glog"
	ui "github.com/chrislusf/seaweedfs/weed/server/filer_ui"
)

// listDirectoryHandler lists directories and folers under a directory
// files are sorted by name and paginated via "lastFileName" and "limit".
// sub directories are listed on the first page, when "lastFileName"
// is empty.
func (fs *FilerServer) listDirectoryHandler(w http.ResponseWriter, r *http.Request) {
	path := r.URL.Path
	if strings.HasSuffix(path, "/") && len(path) > 1 {
		path = path[:len(path)-1]
	}

	limit, limit_err := strconv.Atoi(r.FormValue("limit"))
	if limit_err != nil {
		limit = 100
	}

	lastFileName := r.FormValue("lastFileName")

	entries, err := fs.filer.ListDirectoryEntries(filer2.FullPath(path), lastFileName, false, limit)

	if err != nil {
		glog.V(0).Infof("listDirectory %s %s $d: %s", path, lastFileName, limit, err)
		w.WriteHeader(http.StatusNotFound)
		return
	}

	shouldDisplayLoadMore := len(entries) == limit
	if path == "/" {
		path = ""
	}

	if len(entries) > 0 {
		lastFileName = entries[len(entries)-1].Name()
	}

	glog.V(4).Infof("listDirectory %s, last file %s, limit %d: %d items", path, lastFileName, limit, len(entries))

	args := struct {
		Path                  string
		Breadcrumbs           []ui.Breadcrumb
		Entries               interface{}
		Limit                 int
		LastFileName          string
		ShouldDisplayLoadMore bool
	}{
		path,
		ui.ToBreadcrumb(path),
		entries,
		limit,
		lastFileName,
		shouldDisplayLoadMore,
	}

	if r.Header.Get("Accept") == "application/json" {
		writeJsonQuiet(w, r, http.StatusOK, args)
	} else {
		ui.StatusTpl.Execute(w, args)
	}
}
