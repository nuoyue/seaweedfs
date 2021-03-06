package filesys

import (
	"bazil.org/fuse"
	"bazil.org/fuse/fs"
	"context"
	"github.com/chrislusf/seaweedfs/weed/filer2"
	"github.com/chrislusf/seaweedfs/weed/glog"
	"github.com/chrislusf/seaweedfs/weed/pb/filer_pb"
	"os"
	"path/filepath"
	"time"
)

var _ = fs.Node(&File{})
var _ = fs.NodeOpener(&File{})
var _ = fs.NodeFsyncer(&File{})
var _ = fs.NodeSetattrer(&File{})

type File struct {
	Chunks     []*filer_pb.FileChunk
	Name       string
	dir        *Dir
	wfs        *WFS
	attributes *filer_pb.FuseAttributes
	isOpen     bool
}

func (file *File) Attr(context context.Context, attr *fuse.Attr) error {

	fullPath := filepath.Join(file.dir.Path, file.Name)

	if file.attributes == nil || !file.isOpen {
		item := file.wfs.listDirectoryEntriesCache.Get(fullPath)
		if item != nil {
			entry := item.Value().(*filer_pb.Entry)
			file.Chunks = entry.Chunks
			file.attributes = entry.Attributes
			glog.V(1).Infof("file attr read cached %v attributes", file.Name)
		} else {
			err := file.wfs.withFilerClient(func(client filer_pb.SeaweedFilerClient) error {

				request := &filer_pb.GetEntryAttributesRequest{
					Name:      file.Name,
					ParentDir: file.dir.Path,
				}

				resp, err := client.GetEntryAttributes(context, request)
				if err != nil {
					glog.V(0).Infof("file attr read file %v: %v", request, err)
					return err
				}

				file.attributes = resp.Attributes
				file.Chunks = resp.Chunks

				glog.V(1).Infof("file attr %v %+v: %d", fullPath, file.attributes, filer2.TotalSize(file.Chunks))

				return nil
			})

			if err != nil {
				return err
			}
		}
	}

	attr.Mode = os.FileMode(file.attributes.FileMode)
	attr.Size = filer2.TotalSize(file.Chunks)
	attr.Mtime = time.Unix(file.attributes.Mtime, 0)
	attr.Gid = file.attributes.Gid
	attr.Uid = file.attributes.Uid

	return nil

}

func (file *File) Open(ctx context.Context, req *fuse.OpenRequest, resp *fuse.OpenResponse) (fs.Handle, error) {

	fullPath := filepath.Join(file.dir.Path, file.Name)

	glog.V(3).Infof("%v file open %+v", fullPath, req)

	file.isOpen = true

	return &FileHandle{
		f:          file,
		dirtyPages: newDirtyPages(file),
		RequestId:  req.Header.ID,
		NodeId:     req.Header.Node,
		Uid:        req.Uid,
		Gid:        req.Gid,
	}, nil

}

func (file *File) Setattr(ctx context.Context, req *fuse.SetattrRequest, resp *fuse.SetattrResponse) error {
	fullPath := filepath.Join(file.dir.Path, file.Name)

	glog.V(3).Infof("%v file setattr %+v", fullPath, req)
	if req.Valid.Size() {

		glog.V(3).Infof("%v file setattr set size=%v", fullPath, req.Size)
		if req.Size == 0 {
			// fmt.Printf("truncate %v \n", fullPath)
			file.Chunks = nil
		}
		file.attributes.FileSize = req.Size
	}
	if req.Valid.Mode() {
		file.attributes.FileMode = uint32(req.Mode)
	}

	if req.Valid.Uid() {
		file.attributes.Uid = req.Uid
	}

	if req.Valid.Gid() {
		file.attributes.Gid = req.Gid
	}

	if req.Valid.Mtime() {
		file.attributes.Mtime = req.Mtime.Unix()
	}

	return nil

}

func (file *File) Fsync(ctx context.Context, req *fuse.FsyncRequest) error {
	// fsync works at OS level
	// write the file chunks to the filer
	glog.V(3).Infof("%s/%s fsync file %+v", file.dir.Path, file.Name, req)

	return nil
}
