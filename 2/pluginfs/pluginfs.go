package pluginfs

import (
	"io/fs"
	"path"
	"strings"

	mergefs "github.com/yalue/merged_fs"
	"gitlab.com/mnm/bud/2/mod"
	"gitlab.com/mnm/bud/2/virtual"
	"golang.org/x/sync/errgroup"
)

type Option = func(o *option)

type option struct {
	fileCache *virtual.Map // can be nil
}

// WithFileCache uses a custom mod cache instead of the default
func WithFileCache(cache *virtual.Map) func(o *option) {
	return func(opt *option) {
		opt.fileCache = cache
	}
}

func Load(module *mod.Module, options ...Option) (fs.FS, error) {
	opt := &option{
		fileCache: nil,
	}
	plugins, err := loadPlugins(module)
	if err != nil {
		return nil, err
	}
	merged := merge(module, plugins)
	return &FS{
		opt:    opt,
		merged: merged,
	}, nil
}

// Load plugins
func loadPlugins(module *mod.Module) (plugins []*mod.Module, err error) {
	modfile := module.File()
	var importPaths []string
	for _, req := range modfile.Requires() {
		// The last path in the module path needs to start with "bud-"
		if !strings.HasPrefix(path.Base(req.Mod.Path), "bud-") {
			continue
		}
		importPaths = append(importPaths, req.Mod.Path)
	}
	// Concurrently resolve directories
	plugins = make([]*mod.Module, len(importPaths))
	eg := new(errgroup.Group)
	for i, importPath := range importPaths {
		i, importPath := i, importPath
		eg.Go(func() error {
			module, err := module.Find(importPath)
			if err != nil {
				return err
			}
			plugins[i] = module
			return nil
		})
	}
	// Wait for modules to finish resolving
	if err := eg.Wait(); err != nil {
		return nil, err
	}
	return plugins, nil
}

type FS struct {
	opt    *option
	merged fs.FS
}

func (f *FS) Open(name string) (fs.File, error) {
	if f.opt.fileCache == nil {
		return f.merged.Open(name)
	}
	return f.cachedOpen(f.opt.fileCache, name)
}

func (f *FS) cachedOpen(fmap *virtual.Map, name string) (fs.File, error) {
	if fmap.Has(name) {
		return fmap.Open(name)
	}
	file, err := f.merged.Open(name)
	if err != nil {
		return nil, err
	}
	defer file.Close()
	vfile, err := virtual.From(file)
	if err != nil {
		return nil, err
	}
	fmap.Set(name, vfile)
	return fmap.Open(name)
}

// Merge the filesystems into one
func merge(app fs.FS, plugins []*mod.Module) fs.FS {
	if len(plugins) == 0 {
		return app
	}
	var next = app
	for _, plugin := range plugins {
		next = mergefs.NewMergedFS(next, plugin)
	}
	return next
}
