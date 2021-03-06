package main

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
)

type Node interface {
	fmt.Stringer
}

type Directory struct {
	name     string
	children []Node
}

type File struct {
	name string
	size int64
}

func (file File) String() string {
	if file.size == 0 {
		return file.name + " (empty)"
	}
	return file.name + " (" + strconv.FormatInt(file.size, 10) + "b)"
}

func (directory Directory) String() string {
	return directory.name
}

func readDir(path string, nodes []Node, withFiles bool) (error, []Node) {
	dir, err := os.Open(path)

	if dir == nil {
		return fmt.Errorf("no such dir"), nodes
	}

	files, err := dir.Readdir(0)
	_ = dir.Close()

	sort.Slice(files, func(i, j int) bool {
		return files[i].Name() < files[j].Name()
	})

	for _, info := range files {
		if !(info.IsDir() || withFiles) {
			continue
		}

		var newNode Node
		if info.IsDir() {
			_, children := readDir(filepath.Join(path, info.Name()), []Node{}, withFiles)
			newNode = Directory{info.Name(), children}
		} else {
			newNode = File{info.Name(), info.Size()}
		}

		nodes = append(nodes, newNode)
	}

	return err, nodes
}

func printDir(out io.Writer, nodes []Node, prefixes []string) {
	if len(nodes) == 0 {
		return
	}

	_, _ = fmt.Fprintf(out, "%s", strings.Join(prefixes, ""))

	node := nodes[0]

	if len(nodes) == 1 {
		_, _ = fmt.Fprintf(out, "%s%s\n", "└───", node)
		if directory, ok := node.(Directory); ok {
			printDir(out, directory.children, append(prefixes, "\t"))
		}
		return
	}

	_, _ = fmt.Fprintf(out, "%s%s\n", "├───", node)
	if directory, ok := node.(Directory); ok {
		printDir(out, directory.children, append(prefixes, "│\t"))
	}

	printDir(out, nodes[1:], prefixes)
}

func dirTree(out io.Writer, path string, f bool) error {
	err, nodes := readDir(path, []Node{}, f)
	printDir(out, nodes, []string{})
	return err
}


func main() {
	if len(os.Args) < 2 || len(os.Args) > 3 {
		panic("usage go run main.go .[-f]")
	}

	err := dirTree(
		os.Stdout,
		os.Args[1],
		len(os.Args) == 3 && os.Args[2] == "-f",
		)

	if err != nil {
		panic(err.Error())
	}
}
