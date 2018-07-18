package main

import (
	"bytes"
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/cockroachdb/cockroach/pkg/sql/parser"
	"github.com/cockroachdb/cockroach/pkg/sql/sem/tree"

	// Initialize the builtins.
	_ "github.com/cockroachdb/cockroach/pkg/sql/sem/builtins"
)

func main() {
	sqlRE := regexp.MustCompile(`(?is)(~~~.?sql\s*)(.*?)(\s*~~~)`)
	exprRE := regexp.MustCompile(`^(?s)(\s*)(.*?)(\s*)$`)
	splitRE := regexp.MustCompile(`(?m)^>`)
	cfg := tree.DefaultPrettyCfg()
	cfg.LineWidth = 80
	cfg.UseTabs = false
	cfg.TabWidth = 2

	ignorePaths := make(map[string]bool)
	for _, p := range []string{
		"bytes.md",
		"sql-constants.md",
	} {
		ignorePaths[p] = true
	}

	err := filepath.Walk(".", func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if info.IsDir() || !strings.HasSuffix(path, ".md") {
			return nil
		}
		if ignorePaths[filepath.Base(path)] {
			return nil
		}
		b, err := ioutil.ReadFile(path)
		if err != nil {
			return err
		}
		n := sqlRE.ReplaceAllFunc(b, func(found []byte) []byte {
			blockMatch := sqlRE.FindSubmatch(found)
			var buf bytes.Buffer
			buf.Write(blockMatch[1])
			exprs := splitRE.Split(string(blockMatch[2]), -1)
			for i, expr := range exprs {
				expr := []byte(expr)
				if i > 0 {
					buf.WriteByte('>')
				}
				if skip(expr) {
					buf.Write(expr)
					continue
				}

				exprMatch := exprRE.FindSubmatch(expr)
				s, err := parser.ParseOne(string(exprMatch[2]))
				if err != nil {
					buf.Write(expr)
					continue
				}
				buf.Write(exprMatch[1])
				buf.WriteString(cfg.Pretty(s))
				buf.WriteByte(';')
				buf.Write(exprMatch[3])
			}
			buf.Write(blockMatch[3])
			return buf.Bytes()
		})
		if bytes.Equal(b, n) {
			return nil
		}
		return ioutil.WriteFile(path, n, 0666)
	})
	if err != nil {
		fmt.Println(err)
	}
}

func skip(expr []byte) bool {
	for _, c := range expr {
		if c > 127 {
			return true
		}
	}

	expr = bytes.ToLower(expr)
	for _, contains := range [][]byte{
		[]byte("--"),
		[]byte("backup"),
		[]byte("begin"),
		[]byte("cancel"),
		[]byte("cluster setting"),
		[]byte("collate"),
		[]byte("commit"),
		[]byte("create view"),
		[]byte("partition"),
		[]byte("pause"),
		[]byte("reset"),
		[]byte("restore"),
		[]byte("resume"),
		[]byte("rollback"),
		[]byte("set database"),
		[]byte("show"),
		[]byte("transaction"),
		[]byte("using gin"),

		// https://github.com/mjibson/sqlfmt/issues/21
		[]byte("constraint"),
	} {
		if bytes.Contains(expr, contains) {
			return true
		}
	}
	return false
}
