package dutil

import (
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"testing"
)

var (
	etlSuffixMatch = regexp.MustCompile(`(?i)\.etl$`)
)

type notEtlFile struct{}

func (*notEtlFile) IsFileIncluded(name string) bool {
	return !etlSuffixMatch.MatchString(name)
}

func TestElicitFiles(t *testing.T) {
	files := ElicitFilesFrom(".", StdIgnorer, AllFiles)
	t.Logf("%v", strings.Join(files, "\n"))

	t.Logf(". exist as a dir: %v\n", IsExistingPathDir("."))

	first, err := FindFirstFileWithSuffix(".", "go")
	if err != nil {
		t.Error(err)
	}
	t.Logf("locate for the first: %v\n", first)
}

var (
	goSuffix = regexp.MustCompile(`\.go$`)
)

type goFiles struct{}

func (*goFiles) IsFileIncluded(name string) bool {
	return goSuffix.MatchString(name)
}

func Test_ElicitingTest1(t *testing.T) {
	ElicitFilesFrom0(os.Getenv("HOME"), StdIgnorer, &goFiles{})
}

func Test_ElicitingTest2(t *testing.T) {
	ElicitFilesFrom(os.Getenv("HOME"), StdIgnorer, &goFiles{})
}

func makeMapFromStrArray(a []string) map[string]bool {
	rv := make(map[string]bool)
	for _, str := range a {
		rv[str] = true
	}
	return rv
}

//returns: arr's elements not in src
// in arr but not in src
func diffStringArray(src, arr []string) []string {
	var rv []string
	m0 := makeMapFromStrArray(src)
	for _, v := range arr {
		if _, ok := m0[v]; !ok {
			rv = append(rv, v)
		}
	}
	return rv
}

func Test_ElicitingAllGos(t *testing.T) {
	testCompare(os.Getenv("HOME"), &notEtlFile{}, t)
}

func Test_ElicitingBasic(t *testing.T) {
	t.Logf("testing start")
	startd := os.Getenv("GOPATH")
	t.Logf("starting with %v", startd)
	testCompare(startd, &allFiles{}, t)
	t.Logf("testing done")
}

func Test_ElicitingBasic2(t *testing.T) {
	ElicitFilesFrom0(os.Getenv("GOPATH"), StdIgnorer, &allFiles{})
	//log.Printf("\n%v", strings.Join(files, "\n"))
}

func testCompare(srcDir string, filter FileFilter, t *testing.T) {
	rvs0 := ElicitFilesFrom0(srcDir, StdIgnorer, filter)
	rvs1 := ElicitFilesFrom(srcDir, StdIgnorer, filter)

	printDiff := func() {
		d0 := diffStringArray(rvs1, rvs0)
		d1 := diffStringArray(rvs0, rvs1)
		t.Logf("missing rvs0 from rvs1: %v", strings.Join(d0, ", "))
		t.Logf("missing rvs1 from rvs0: %v", strings.Join(d1, ", "))
		t.Fatal("not the same")
	}
	sort.Strings(rvs0)
	sort.Strings(rvs1)
	if len(rvs0) != len(rvs1) {
		t.Logf("rvs0 len: %v", len(rvs0))
		t.Logf("rvs1 len: %v", len(rvs1))
		printDiff()
	}
	l0 := len(rvs0)
	for i := 0; i < l0; i++ {
		if filepath.ToSlash(rvs0[i]) != filepath.ToSlash(rvs1[i]) {
			printDiff()
			//t.Fatalf("Error %v differs from %v", rvs0[i], rvs1[i])
		}
	}
	t.Logf("%v has been test through", len(rvs0))
}
