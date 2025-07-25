package snapshot_test

import (
	"fmt"
	"io"
	"os"
	"strings"
	"testing"

	"github.com/PlakarKorp/kloset/objects"
	"github.com/PlakarKorp/kloset/snapshot/importer"
	ptesting "github.com/PlakarKorp/kloset/testing"
	"github.com/stretchr/testify/require"
)

func TestSimpleBackup(t *testing.T) {
	repo := ptesting.GenerateRepository(t, nil, nil, nil)

	files := []ptesting.MockFile{
		ptesting.NewMockFile("hello.txt", 0644, "hello world!\n"),
		ptesting.NewMockFile("unreadable", 0, "wooo\n"),
	}
	snap := ptesting.GenerateSnapshot(t, repo, files)

	summary := snap.Header.GetSource(0).Summary
	require.Equal(t, summary.Directory.Errors+summary.Below.Errors, uint64(1))

	fs, err := snap.Filesystem()
	require.NoError(t, err)

	fs, err = fs.Chroot(snap.Header.GetSource(0).Importer.Directory)
	require.NoError(t, err)

	fp, err := fs.Open("hello.txt")
	require.NoError(t, err, "can't open expected file")
	require.NotNil(t, fp)

	fp, err = fs.Open("unreadable")
	require.NotNil(t, err, "can open file unexpectedly")
	require.Nil(t, fp)
}

func TestBackupWithExcludes(t *testing.T) {
	repo := ptesting.GenerateRepository(t, nil, nil, nil)

	files := []ptesting.MockFile{
		ptesting.NewMockFile("hello0", 0644, "hello world!\n"),
		ptesting.NewMockFile("hello1", 0644, "hello world!\n"),
		ptesting.NewMockFile("hello2", 0644, "hello world!\n"),
		ptesting.NewMockFile("hello3", 0644, "hello world!\n"),
		ptesting.NewMockFile("hello4", 0644, "hello world!\n"),
		ptesting.NewMockFile("hello5", 0644, "hello world!\n"),
		ptesting.NewMockFile("hello6", 0644, "hello world!\n"),
		ptesting.NewMockFile("hello7", 0644, "hello world!\n"),
		ptesting.NewMockFile("hello8", 0644, "hello world!\n"),
		ptesting.NewMockFile("hello9", 0644, "hello world!\n"),
	}

	snap := ptesting.GenerateSnapshot(t, repo, files, ptesting.WithExcludes([]string{
		"/hello0", "/hello2", "/hello4", "/hello8",
	}))

	summary := &snap.Header.GetSource(0).Summary
	require.Equal(t, summary.Directory.Files, uint64(6))
}

func errorGenerator(ch chan<- *importer.ScanResult) {
	ch <- &importer.ScanResult{
		Record: &importer.ScanRecord{
			Pathname: "/",
			FileInfo: objects.FileInfo{
				Lname:      "/",
				Lnlink:     1,
				Lmode:      os.ModeDir,
				Lusername:  "flan",
				Lgroupname: "hacker",
			},
		},
	}

	for i := 'a'; i < 'g'; i++ {
		ch <- &importer.ScanResult{
			Record: &importer.ScanRecord{
				Pathname: fmt.Sprintf("/%v", i),
				FileInfo: objects.FileInfo{
					Lname:      fmt.Sprint(i),
					Lnlink:     1,
					Lmode:      os.ModeDir,
					Lusername:  "flan",
					Lgroupname: "hacker",
				},
			},
		}
		for j := 'a'; j < 'g'; j++ {
			ch <- &importer.ScanResult{
				Record: &importer.ScanRecord{
					Pathname: fmt.Sprintf("/%v/%v", i, j),
					FileInfo: objects.FileInfo{
						Lname:      fmt.Sprint(j),
						Lnlink:     1,
						Lmode:      os.ModeDir,
						Lusername:  "flan",
						Lgroupname: "hacker",
					},
				},
			}

			for k := range 10 {
				if k%2 == 0 {
					ch <- &importer.ScanResult{
						Record: &importer.ScanRecord{
							Pathname: fmt.Sprintf("/%v/%v/%v", i, j, k),
							FileInfo: objects.FileInfo{
								Lname:      fmt.Sprint(k),
								Lsize:      int64(len("hello world")),
								Lnlink:     1,
								Lusername:  "flan",
								Lgroupname: "hacker",
							},
							Reader: importer.NewLazyReader(func() (io.ReadCloser, error) {
								return io.NopCloser(strings.NewReader("hello world")), nil
							}),
						},
					}
				} else {
					ch <- &importer.ScanResult{
						Error: &importer.ScanError{
							Pathname: fmt.Sprintf("/%v/%v/%v", i, j, k),
							Err:      os.ErrPermission,
						},
					}
				}
			}
		}
	}
	close(ch)
}

func TestBackupManyError(t *testing.T) {
	repo := ptesting.GenerateRepository(t, nil, nil, nil)
	snap := ptesting.GenerateSnapshot(t, repo, nil, ptesting.WithGenerator(errorGenerator))

	summary := snap.Header.GetSource(0).Summary
	require.Equal(t, summary.Below.Files, uint64(180))
	require.Equal(t, summary.Below.Directories, uint64(36))
	require.Equal(t, summary.Below.Errors, uint64(180))
}
