package custom

import (
	"bytes"
	"encoding/hex"
	"io/ioutil"
	"os"
	"path"
	"path/filepath"
	"strings"

	"github.com/UnnoTed/fileb0x/compression"
	"github.com/UnnoTed/fileb0x/dir"
	"github.com/UnnoTed/fileb0x/file"
	"github.com/UnnoTed/fileb0x/utils"
	"github.com/bmatcuk/doublestar"
)

// SharedConfig holds needed data from config package
// without causing import cycle
type SharedConfig struct {
	Output      string
	Compression *compression.Gzip
}

// Custom is a set of files with dedicaTed customization
type Custom struct {
	Files  []string
	Base   string
	Prefix string

	Exclude []string
	Replace []Replacer
}

// Parse the files transforming them into a byte string and inserting the file
// into a map of files
func (c *Custom) Parse(files *map[string]*file.File, dirs **dir.Dir, config *SharedConfig) error {
	to := *files
	dirList := *dirs

	for _, customFile := range c.Files {
		customFile = utils.FixPath(customFile)
		walkErr := filepath.Walk(customFile, func(fpath string, info os.FileInfo, err error) error {
			if err != nil {
				return err
			}

			// only files will be processed
			if info.IsDir() {
				return nil
			}

			var fixedPath string
			if c.Base != "" {
				// remove base and inserts prefix
				fixedPath = strings.Replace(
					utils.FixPath(fpath),
					utils.FixPath(c.Base),
					utils.FixPath(c.Prefix),
					1) // once
			} else {
				fixedPath = utils.FixPath(fpath)
			}

			// check for excluded files
			for _, excludedFile := range c.Exclude {
				m, err := doublestar.Match(c.Prefix+excludedFile, fixedPath)
				if err != nil {
					return err
				}

				if m {
					return nil
				}
			}

			// FIXME
			// prevent including itself (destination file or dir)
			if info.Name() == config.Output {
				return nil
			}
			/*if info.Name() == cfg.Output { ||
			  info.Name() == path.Base(path.Dir(jsonFile)) {
			  return nil
			}*/

			// get file's content
			content, err := ioutil.ReadFile(fpath)
			if err != nil {
				return err
			}

			// loop through replace list
			for _, r := range c.Replace {
				// check if path matches the pattern from property: file
				matched, err := doublestar.Match(c.Prefix+r.File, fixedPath)
				if err != nil {
					return err
				}

				if matched {
					for pattern, word := range r.Replace {
						content = []byte(strings.Replace(string(content), pattern, word, -1))
					}
				}
			}

			// it's way faster to use a buffer as string than use string
			var buf bytes.Buffer
			buf.WriteString(`[]byte("`)
			f := file.NewFile()

			// compress the content
			if config.Compression.Options != nil {
				content, err = config.Compression.Compress(content)
				if err != nil {
					return err
				}
			}

			// it's way faster to loop and slice a string than a byte array
			h := hex.EncodeToString(content)

			// loop through hex string, at each 2 chars
			// it's added into a byte array -> []byte{0x61 ,...}
			for i := 0; i < len(h); i += 2 {
				buf.WriteString(`\x` + h[i:i+2])
			}

			f.Data = buf.String() + `")`
			f.Name = info.Name()
			f.Path = fixedPath

			// insert dir to dirlist so it can be created on b0x's init()
			dirList.Insert(path.Dir(fixedPath))

			// insert file into file list
			to[fixedPath] = f
			return nil
		})

		if walkErr != nil {
			return walkErr
		}
	}

	return nil
}
