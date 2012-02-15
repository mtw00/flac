package flacmeta

import (
	. "launchpad.net/gocheck"
	"testing"
	"os"
	"fmt"
)

func Test(t *testing.T) { TestingT(t) }
type S struct{}
var _ = Suite(&S{})

func (s *S) TestFLACParseMetadataBlockHeader1(c *C) {
	f, err := os.Open("testdata/44100-16-mono.flac")
	if err != nil {
		fmt.Println("FATAL:", err)
		os.Exit(-1)
	}
	defer f.Close()

	metadata := ReadFLACMetadata(f)

	streaminfo := FLACStreaminfo{
		FLACMetadataBlockHeader{0, 34, false},
		FLACStreaminfoBlock{4096, 4096, 11, 14, 44100, 1, 16, 1014300, "e5ccc967ced6c111530e5c79e33c969e"}}
	c.Check(streaminfo, Equals, metadata.FLACStreaminfo)

	comment := FLACVorbisComment{
		FLACMetadataBlockHeader{4, 57, false},
		FLACVorbisCommentBlock{"reference libFLAC 1.2.1 20070917", 1, []string{"ARTIST=GoGoGo"}}}
	c.Check(comment, Equals, metadata.FLACVorbisComment)

	pad := FLACPadding{ FLACMetadataBlockHeader{1, 8175, true}, nil}
	c.Check(pad, Equals, metadata.FLACPadding)
}
