package mod

import (
	"fmt"
	"go/build"
	"os"
	"path"
	"path/filepath"
	"strings"

	"gitlab.com/mnm/bud/internal/modcache"

	"gitlab.com/mnm/bud/go/is"
	"golang.org/x/mod/modfile"
)

type Require = modfile.Require

type File struct {
	file  *modfile.File
	cache *modcache.Cache
	dir   string
}

// Directory returns the module directory (e.g. /Users/$USER/...)
func (f *File) Directory(subpaths ...string) string {
	return filepath.Join(append([]string{f.dir}, subpaths...)...)
}

// ModulePath returns the module path (e.g. gitlab.com/mnm/bud)
func (f *File) ModulePath(subpaths ...string) string {
	return path.Join(append([]string{f.file.Module.Mod.Path}, subpaths...)...)
}

// ResolveDirectory resolves an import to an absolute path
func (f *File) ResolveDirectory(importPath string) (directory string, err error) {
	absdir, err := f.resolveDirectory(importPath)
	if err != nil {
		return "", err
	}
	// Ensure the resolved directory exists
	if _, err := os.Stat(absdir); err != nil {
		return "", fmt.Errorf("mod: unable to resolve directory for import path %q: %w", importPath, err)
	}
	return absdir, nil
}

// Load a new modfile from an import path
func (f *File) Load(importPath string) (*File, error) {
	absdir, err := f.resolveDirectory(importPath)
	if err != nil {
		return nil, err
	}
	// First search for go.mod
	return findModFile(f.cache, absdir)
}

// ResolveImport returns an import path from a local directory.
func (f *File) ResolveImport(directory string) (importPath string, err error) {
	if !strings.HasPrefix(directory, f.dir) {
		return "", fmt.Errorf("%q can't be outside the module directory %q", directory, f.dir)
	}
	importPath, err = resolveImport(f, directory)
	if err != nil {
		return "", err
	}
	return importPath, nil
}

func (f *File) AddRequire(importPath, version string) error {
	return f.file.AddRequire(importPath, version)
}

func (f *File) Replace(oldPath, newPath string) error {
	return f.AddReplace(oldPath, "", newPath, "")
}

func (f *File) AddReplace(oldPath, oldVers, newPath, newVers string) error {
	return f.file.AddReplace(oldPath, oldVers, newPath, newVers)
}

// Return a list of requires
func (f *File) Requires() (reqs []*Require) {
	reqs = make([]*Require, len(f.file.Require))
	for i, req := range f.file.Require {
		reqs[i] = req
	}
	return reqs
}

func (f *File) Format() []byte {
	return modfile.Format(f.file.Syntax)
}

// // Open implements fs.FS allowing you to read the contents of your dependencies.
// func (f *File) Open(name string) (fs.File, error) {
// 	dir, err := f.ResolveDirectory(name)
// 	if err != nil {
// 		return nil, err
// 	}
// 	return os.DirFS(dir).Open(".")
// }

// dir containing the standard libraries
var stdDir = filepath.Join(build.Default.GOROOT, "src")

func resolveImport(f *File, dir string) (importPath string, err error) {
	abspath, err := filepath.Abs(dir)
	if err != nil {
		return "", err
	}
	// Handle stdlib paths
	if strings.HasPrefix(dir, stdDir) {
		return filepath.Rel(stdDir, dir)
	}
	relPath, err := filepath.Rel(f.Directory(), abspath)
	if err != nil {
		return "", err
	}
	return filepath.Join(f.ModulePath(), relPath), nil
}

// ResolveDirectory resolves an import to an absolute path
func (f *File) resolveDirectory(importPath string) (directory string, err error) {
	if is.StdLib(importPath) {
		return filepath.Join(stdDir, importPath), nil
	}
	if contains(f.file.Module.Mod.Path, importPath) {
		directory = filepath.Join(f.dir, strings.TrimPrefix(importPath, f.file.Module.Mod.Path))
		return directory, nil
	}
	// loop over replaces
	for _, rep := range f.file.Replace {
		if contains(rep.Old.Path, importPath) {
			relPath := strings.TrimPrefix(importPath, rep.Old.Path)
			newPath := filepath.Join(rep.New.Path, relPath)
			resolved := resolvePath(f.dir, newPath)
			return resolved, nil
		}
	}
	// loop over requires
	for _, req := range f.file.Require {
		if contains(req.Mod.Path, importPath) {
			relPath := strings.TrimPrefix(importPath, req.Mod.Path)
			dir, err := f.cache.ResolveDirectory(req.Mod.Path, req.Mod.Version)
			if err != nil {
				return "", err
			}
			return filepath.Join(dir, relPath), nil
		}
	}
	return "", fmt.Errorf("mod: unable to resolve directory for import path %q", importPath)
}

func resolvePath(path string, rest ...string) (result string) {
	result = path
	for _, p := range rest {
		if filepath.IsAbs(p) {
			result = p
			continue
		}
		result = filepath.Join(result, p)
	}
	return result
}

func contains(basePath, importPath string) bool {
	return basePath == importPath || strings.HasPrefix(importPath, basePath+"/")
}
