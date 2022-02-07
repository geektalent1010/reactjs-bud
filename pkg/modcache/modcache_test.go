package modcache_test

import (
	"bytes"
	"context"
	"errors"
	"go/build"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
	"testing/fstest"

	"gitlab.com/mnm/bud/internal/dsync"
	"gitlab.com/mnm/bud/pkg/vfs"

	"github.com/matryer/is"
	"gitlab.com/mnm/bud/pkg/modcache"
)

// Run calls `go run -mod=mod main.go ...`
func goRun(cacheDir, appDir string) (string, string, error) {
	ctx := context.Background()
	cmd := exec.CommandContext(ctx, "go", "run", "-mod=mod", "main.go")
	cmd.Env = append(os.Environ(), "GOMODCACHE="+cacheDir, "GOPRIVATE=*")
	stdout := new(bytes.Buffer)
	cmd.Stdout = stdout
	stderr := new(bytes.Buffer)
	cmd.Stderr = stderr
	cmd.Stdin = os.Stdin
	cmd.Dir = appDir
	err := cmd.Run()
	if stderr.Len() > 0 {
		return "", stderr.String(), nil
	}
	if err != nil {
		return "", "", err
	}
	return stdout.String(), "", nil
}

func TestDirectory(t *testing.T) {
	is := is.New(t)
	dir := modcache.Default().Directory()
	if env := os.Getenv("GOMODCACHE"); env != "" {
		is.Equal(dir, env)
	} else {
		is.Equal(dir, filepath.Join(build.Default.GOPATH, "pkg", "mod"))
	}
}

func TestWriteModule(t *testing.T) {
	is := is.New(t)
	cacheDir := t.TempDir()
	modCache := modcache.New(cacheDir)
	err := modCache.Write(modcache.Modules{
		"mod.test/one@v0.0.1": modcache.Files{
			"const.go": "package one\n\nconst Answer = 42",
		},
		"mod.test/one@v0.0.2": modcache.Files{
			"const.go": "package one\n\nconst Answer = 43",
		},
	})
	is.NoErr(err)
	dir, err := modCache.ResolveDirectory("mod.test/one", "v0.0.2")
	is.NoErr(err)
	is.Equal(dir, modCache.Directory("mod.test", "one@v0.0.2"))
	// Now verify that the go commands don't try downloading mod.test/one
	appDir := t.TempDir()
	err = vfs.Write(appDir, vfs.Map{
		"go.mod": []byte(`
			module app.com

			require (
				mod.test/one v0.0.2
			)
		`),
		"main.go": []byte(`
			package main

			import (
				"fmt"
				"mod.test/one"
			)

			func main() {
				fmt.Print(one.Answer)
			}
		`),
	})
	is.NoErr(err)
	stdout, stderr, err := goRun(cacheDir, appDir)
	is.NoErr(err)
	is.Equal(stderr, "")
	is.Equal(stdout, "43")
}

func TestWriteModuleFS(t *testing.T) {
	is := is.New(t)
	cacheDir := t.TempDir()
	modCache := modcache.New(cacheDir)
	fsys, err := modCache.WriteFS(modcache.Modules{
		"mod.test/one@v0.0.1": modcache.Files{
			"const.go": "package one\n\nconst Answer = 42",
		},
		"mod.test/one@v0.0.2": modcache.Files{
			"const.go": "package one\n\nconst Answer = 43",
		},
	})
	is.NoErr(err)
	err = fstest.TestFS(fsys,
		"cache/download/mod.test/one/@v/v0.0.1.mod",
		"cache/download/mod.test/one/@v/v0.0.1.ziphash",
		"cache/download/mod.test/one/@v/v0.0.1.ziphash",
		"cache/download/mod.test/one/@v/v0.0.2.mod",
		"cache/download/mod.test/one/@v/v0.0.2.ziphash",
		"mod.test/one@v0.0.1/const.go",
		"mod.test/one@v0.0.1/go.mod",
		"mod.test/one@v0.0.2/const.go",
		"mod.test/one@v0.0.2/go.mod",
	)
	is.NoErr(err)
	err = dsync.Dir(fsys, ".", vfs.OS(cacheDir), ".")
	is.NoErr(err)
	dir, err := modCache.ResolveDirectory("mod.test/one", "v0.0.2")
	is.NoErr(err)
	is.Equal(dir, modCache.Directory("mod.test", "one@v0.0.2"))
	// Now verify that the go commands don't try downloading mod.test/one
	appDir := t.TempDir()
	err = vfs.Write(appDir, vfs.Map{
		"go.mod": []byte(`
			module app.com

			require (
				mod.test/one v0.0.2
			)
		`),
		"main.go": []byte(`
			package main

			import (
				"fmt"
				"mod.test/one"
			)

			func main() {
				fmt.Print(one.Answer)
			}
		`),
	})
	is.NoErr(err)
	stdout, stderr, err := goRun(cacheDir, appDir)
	is.NoErr(err)
	is.Equal(stderr, "")
	is.Equal(stdout, "43")
}

func TestResolveDirectoryFromCache(t *testing.T) {
	is := is.New(t)
	modCache := modcache.Default()
	is.True(modCache.Directory() != "")
	dir, err := modCache.ResolveDirectory("github.com/matryer/is", "v1.4.0")
	is.NoErr(err)
	is.Equal(dir, modCache.Directory("github.com", "matryer", "is@v1.4.0"))
}

func TestExportImport(t *testing.T) {
	is := is.New(t)
	cacheDir := t.TempDir()
	modCache := modcache.New(cacheDir)
	err := modCache.Write(map[string]modcache.Files{
		"gitlab.com/mnm/bud-tailwind@v0.0.1": modcache.Files{
			"public/tailwind/preflight.css": `/* tailwind */`,
		},
	})
	is.NoErr(err)
	dir, err := modCache.ResolveDirectory("gitlab.com/mnm/bud-tailwind", "v0.0.1")
	is.NoErr(err)
	is.Equal(dir, modCache.Directory("gitlab.com/mnm/bud-tailwind@v0.0.1"))
	cacheDir2 := t.TempDir()
	modCache2 := modcache.New(cacheDir2)
	// Verify modcache2 doesn't have the module
	dir, err = modCache2.ResolveDirectory("gitlab.com/mnm/bud-tailwind", "v0.0.1")
	is.Equal(dir, "")
	is.True(errors.Is(err, fs.ErrNotExist))
	// Export to a new location
	tmpDir := t.TempDir()
	err = modCache.Export(tmpDir)
	is.NoErr(err)
	// Import from new location
	err = modCache2.Import(tmpDir)
	is.NoErr(err)
	// Try again
	dir, err = modCache2.ResolveDirectory("gitlab.com/mnm/bud-tailwind", "v0.0.1")
	is.NoErr(err)
	is.Equal(dir, modCache2.Directory("gitlab.com/mnm/bud-tailwind@v0.0.1"))
}
