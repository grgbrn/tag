package tag

import (
	"bytes"
	"errors"
	"fmt"
	"io"
)

type Chapter struct {
	ElementID string

	StartTime   uint32
	EndTime     uint32
	StartOffset uint32
	EndOffset   uint32

	EmbeddedTags map[string]interface{}
}

func (c Chapter) Title() string {
	val, ok := c.EmbeddedTags["TIT2"]
	if !ok {
		return ""
	}
	title, ok := val.(string)
	if !ok {
		return ""
	}
	return title
}

// TODO: accessors for urls, images, etc.

// TODO: does this need to be exported?
type ChapterTOC struct {
	ElementID string
	Entries   []string

	EmbeddedTags map[string]interface{}
}

// TODO: document this frame format in the same style as the others
func readChapFrame(b []byte, version Format) (Chapter, error) {
	var err error
	var result Chapter

	var bytesRead int // XXX this is a little messy

	if len(b) == 0 {
		return Chapter{}, nil
	}

	// look for the first null byte
	ix := bytes.Index(b, []byte{0})
	if ix == -1 {
		return Chapter{}, errors.New("no null terminator found for element id")
	}
	result.ElementID = string(b[:ix])
	bytesRead = ix + 1

	reader := bytes.NewReader(b[ix+1:])

	// now come 4 32bit integers
	result.StartTime, err = readChapterUint32(reader)
	if err != nil {
		return Chapter{}, fmt.Errorf("error reading start time: %v", err)
	}
	result.EndTime, err = readChapterUint32(reader)
	if err != nil {
		return Chapter{}, fmt.Errorf("error reading end time: %v", err)
	}
	result.StartOffset, err = readChapterUint32(reader)
	if err != nil {
		return Chapter{}, fmt.Errorf("error reading start offset: %v", err)
	}
	result.EndOffset, err = readChapterUint32(reader)
	if err != nil {
		return Chapter{}, fmt.Errorf("error reading end offset: %v", err)
	}

	bytesRead += 16

	// there may be additional embedded frames
	extraBytes := len(b) - bytesRead
	if extraBytes > 0 {
		result.EmbeddedTags = make(map[string]interface{}, 0)

		for extraBytes > 0 {
			frame, err := readID3v2Frame(reader, version)
			if err != nil {
				return Chapter{}, fmt.Errorf("error reading embedded tags: %v", err)
			}
			if frame == nil {
				return Chapter{}, fmt.Errorf("error reading embedded tags: %v", err)
			}

			result.EmbeddedTags[frame.Name] = frame.Data

			extraBytes -= int(frame.TotalFrameSize())
		}
	}

	return result, nil
}

// TODO: document this frame format in the same style as the others
func readCTOCFrame(b []byte, version Format) (ChapterTOC, error) {
	var err error
	var toc ChapterTOC

	var bytesRead int // XXX this is a little messy

	// look for the first null byte
	ix := bytes.Index(b, []byte{0})
	if ix == -1 {
		return ChapterTOC{}, errors.New("no null terminator found for element id")
	}
	toc.ElementID = string(b[:ix])

	bytesRead = ix + 1

	reader := bytes.NewReader(b[ix+1:])

	// skip flags for now
	_, err = readUint(reader, 1)
	if err != nil {
		return ChapterTOC{}, fmt.Errorf("error reading flags: %v", err)
	}
	bytesRead += 1

	// read entry count
	entryCount, err := readUint(reader, 1)
	if err != nil {
		return ChapterTOC{}, fmt.Errorf("error reading entry count: %v", err)
	}
	bytesRead += 1

	entries := make([]string, entryCount)
	for i := 0; i < int(entryCount); i++ {
		entries[i], err = readTerminatedString(reader)
		if err != nil {
			return ChapterTOC{}, fmt.Errorf("error reading entry %d: %v", i, err)
		}
		bytesRead += len(entries[i]) + 1
	}

	toc.Entries = entries

	// there may be additional embedded frames
	extraBytes := len(b) - bytesRead
	if extraBytes > 0 {
		toc.EmbeddedTags = make(map[string]interface{}, 0)

		for extraBytes > 0 {
			frame, err := readID3v2Frame(reader, version)
			if err != nil {
				return ChapterTOC{}, fmt.Errorf("error reading embedded tags: %v", err)
			}
			if frame == nil {
				return ChapterTOC{}, fmt.Errorf("error reading embedded tags: %v", err)
			}

			toc.EmbeddedTags[frame.Name] = frame.Data
			extraBytes -= int(frame.TotalFrameSize())
		}
	}

	return toc, nil
}

// read a uint32, replacing it with 0 if it's 0xFFFFFFFF
func readChapterUint32(r io.Reader) (uint32, error) {
	n, err := readUint(r, 4)
	if err != nil {
		return 0, err
	}
	if n == 0xFFFFFFFF {
		return 0, nil
	}
	return uint32(n), nil
}

// XXX is there not a stdlib function for this? readUntil?
func readTerminatedString(r io.Reader) (string, error) {
	buf := bytes.Buffer{}
	ch := make([]byte, 1)
	for {
		n, err := r.Read(ch)
		if err != nil {
			return "", err
		}
		if n == 0 {
			break // ERR?
		}
		if ch[0] == 0 {
			break
		}
		buf.Write(ch)
	}
	return buf.String(), nil
}
