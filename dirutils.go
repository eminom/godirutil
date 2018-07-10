package dutil

import (
	"fmt"
	"io"
	"io/ioutil"
	"os"
	"path"
	"path/filepath"
	"regexp"
	"runtime"
	"strings"
	"sync"
)

var (
	concurrent = runtime.GOMAXPROCS(runtime.NumCPU())
)

func MustToAbsPath(p string) string {
	ret, err := filepath.Abs(p)
	if err != nil {
		panic(err)
	}
	return ret
}

//
// refer to https://stackoverflow.com/questions/30697324/how-to-check-if-directory-on-path-is-empty
//
func IsDirEmpty(filepath string) (bool, error) {
	f, err := os.Open(filepath)
	if err != nil {
		return false, err
	}
	defer f.Close()

	_, err = f.Readdirnames(1)
	if err == io.EOF {
		return true, nil
	}
	return false, err
}

// Either a file or a directory.
func IsPathExist(filepath string) bool {
	if _, err := os.Stat(filepath); nil == err {
		return true
	} else {
		// no file info.
		return false
	}
}

func IsDirExist(filepath string) bool {
	if info, err := os.Stat(filepath); err != nil {
		return false
	} else {
		return info.IsDir()
	}
}

// may throw exception on this one
func IsExistingDirEmpty(filepath string) bool {
	isEmpty, err := IsDirEmpty(filepath)
	if err != nil {
		panic(err)
	}
	return isEmpty
}

// if not exists, panic
func IsExistingPathDir(filepath string) bool {
	info, err := os.Stat(filepath)
	if err != nil {
		panic(err)
	}
	return info.IsDir()
}

func IsExistingPathFile(filepath string) bool {
	return !IsExistingPathDir(filepath)
}

// without exception
// false: either a folder or simply non-exist
func IsFileForPath(filepath string) bool {
	if info, err := os.Stat(filepath); nil == err {
		return !info.IsDir()
	}
	return false
}

// pay attention:  IsFileForPath != invert(IsDirForPath)
func IsDirForPath(filepath string) bool {
	if info, err := os.Stat(filepath); nil == err {
		return info.IsDir()
	}
	return false
}

type DirFilter interface {
	IsDirIgnored(name string) bool
}

type stdIgnorer struct{}

func (s *stdIgnorer) IsDirIgnored(name string) bool {
	switch name {
	case ".svn":
	case ".git":
		return true
	}
	return false
}

type noIgnorer struct{}

func (*noIgnorer) IsDirIgnored(string) bool {
	return false
}

type FileFilter interface {
	IsFileIncluded(name string) bool
}

type allFiles struct{}

func (*allFiles) IsFileIncluded(string) bool {
	return true
}

var (
	StdIgnorer = &stdIgnorer{}
	NoIgnorer  = &noIgnorer{}

	AllFiles = &allFiles{}
)

// paths returned are all in absolute forms
func ElicitFilesFrom0(dir string, ignorer DirFilter, fileFilter FileFilter) []string {
	var e error
	dir, e = filepath.Abs(dir) // eliminate the case that dir is "." or ".."
	if e != nil {
		panic(e)
	}
	dir = filepath.ToSlash(dir)
	var rv []string
	// Walk: the first parameter in callback is in platform's separator ('\' for windows, etc)
	filepath.Walk(dir, func(now string, info os.FileInfo, err error) error {
		now = filepath.ToSlash(now)
		if err != nil {
			return err
		}
		if !info.IsDir() {
			// now is already in abs form.(the dir passed in has been adjust to abs in the beginning)
			if fileFilter.IsFileIncluded(now) {
				rv = append(rv, now) // now contains the dir(except that dir is dot/dot-dot)
			}
		} else {
			raw := strings.TrimPrefix(now, dir)
			raw = strings.TrimPrefix(raw, "/")
			rls := strings.Split(raw, "/")
			if len(rls) > 0 && ignorer.IsDirIgnored(rls[len(rls)-1]) {
				//println("skip:", now, " for ", raw)
				return filepath.SkipDir
			}
		}
		return nil // continue
	})
	return rv
}

//~ A faster way
func ElicitFilesFrom(dir string, ignorer DirFilter, fileFilter FileFilter) []string {
	dir, e := filepath.Abs(dir)
	if e != nil {
		panic(e)
	}
	dir = filepath.ToSlash(dir) //fixed.

	fileReadCh := make(chan string, 1024)
	readDone := make(chan struct{})

	var wRead sync.WaitGroup
	wRead.Add(1)
	var rvs []string
	go func() {
		defer wRead.Done()
		for {
			select {
			case <-readDone:
				return
			case fpath := <-fileReadCh:
				rvs = append(rvs, fpath)
			}
		}
	}()

	semaphoreChan := make(chan struct{}, concurrent)
	var wg sync.WaitGroup
	wg.Add(1)
	go walkSub(dir, ignorer, fileFilter, &wg, semaphoreChan, fileReadCh)

	wg.Wait()
	close(readDone)
	wRead.Wait() // read done

	//now fixed: fileReadCh is bufferred.
A100:
	for {
		select {
		case nuevo := <-fileReadCh:
			rvs = append(rvs, nuevo)
		default:
			break A100
		}
	}
	return rvs
}

// dir is expected to be in absolute form.
func walkSub(dir string, ignorer DirFilter, fileFilter FileFilter, wg *sync.WaitGroup, semaphoreChan chan struct{}, fileChan chan<- string) {
	semaphoreChan <- struct{}{}
	go func() {
		defer func() {
			<-semaphoreChan // release one
			wg.Done()
		}()
		fdir, err := os.Open(dir)
		if err != nil {
			fmt.Printf("error opening directory:%v\n", err)
			return
		}
		defer fdir.Close()
		files, err := fdir.Readdir(-1)
		if err != nil {
			fmt.Printf("reading dir error:%v\n", err)
			return
		}
		for _, f := range files {
			if f.IsDir() {
				if !ignorer.IsDirIgnored(f.Name()) {
					wg.Add(1)
					go walkSub(path.Join(dir, f.Name()), ignorer, fileFilter, wg, semaphoreChan, fileChan)
				}
			} else {
				if fileFilter.IsFileIncluded(f.Name()) {
					fileChan <- path.Join(dir, f.Name())
				}
			}
		}
	}()
}

// refer to following link:
// https://stackoverflow.com/questions/21060945/simple-way-to-copy-a-file-in-golang
func MustCopyFromTo(src string, dst string) {
	// folder => folder
	if IsDirForPath(src) {
		err := os.MkdirAll(dst, os.ModePerm)
		if err != nil {
			panic(err)
		}
	} else if IsFileForPath(src) {
		err := CopyFileFromTo(src, dst, false)
		if err != nil {
			panic(err)
		}
	}
}

// src is ensured to be a file.
func CopyFileFromTo(src string, dst string, check bool) (err error) {
	if check && !IsFileForPath(src) {
		err = fmt.Errorf("%v is not a file", src)
		return
	}
	err = os.MkdirAll(path.Dir(dst), os.ModePerm)
	if err != nil {
		return
	}

	// a much faster copy.(rather than reading-writing raw content by hand)
	// if err = os.Link(src, dst); nil == err {
	// 	return
	// }

	var fin *os.File
	fin, err = os.Open(src)
	if err != nil {
		return
	}
	defer fin.Close()
	var fout *os.File
	fout, err = os.Create(dst)
	if err != nil {
		return
	}
	defer func() {
		cerr := fout.Close()
		if cerr != nil {
			err = cerr
		}
	}()
	if _, err = io.Copy(fout, fin); err != nil {
		return
	}
	err = fout.Sync()
	return
}

// if not, exception will be raisen.
func EnsureDir(dirpath string) {
	err := os.MkdirAll(dirpath, os.ModePerm)
	if err != nil {
		//panic(errs.Newf("failed to create <%v>: %v", sub, err.Error()))
		panic(err)
	}
}

func FindFirstFileWithSuffix(dir string, suffix string) (string, error) {
	sp := regexp.MustCompile(fmt.Sprintf("\\.%v$", suffix))
	infos, err := ioutil.ReadDir(dir)
	if err != nil {
		return "", err
	}

	for _, info := range infos {
		if sp.MatchString(info.Name()) {
			return path.Join(dir, info.Name()), nil
		}
	}
	return "", fmt.Errorf("not found for .%v", suffix)
}
