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

	metadata := new(FLACMetadata)
	metadata.ReadFLACMetadata(f)

	streaminfo := FLACStreaminfo{
		&FLACMetadataBlockHeader{0, 34, false, 0},
		&FLACStreaminfoBlock{4096, 4096, 11, 14, 44100, 1, 16, 1014300, "e5ccc967ced6c111530e5c79e33c969e"}, true}
	c.Check(streaminfo, Equals, metadata.FLACStreaminfo)

	comment := FLACVorbisComment{
		&FLACMetadataBlockHeader{4, 57, false, 0},
		&FLACVorbisCommentBlock{"reference libFLAC 1.2.1 20070917", 1, []string{"ARTIST=GoGoGo"}}, true}
	c.Check(comment, Equals, metadata.FLACVorbisComment)

	pad := FLACPadding{&FLACMetadataBlockHeader{1, 8175, true, 0}, nil, true}
	c.Check(pad, Equals, metadata.FLACPadding)

	seekpointBlocks := []FLACSeekpointBlock{}
	seekpointBlocks = append(seekpointBlocks, FLACSeekpointBlock{0, 0, 4096})
	seekpointBlocks = append(seekpointBlocks, FLACSeekpointBlock{438272, 1177, 4096})
	seekpointBlocks = append(seekpointBlocks, FLACSeekpointBlock{880640, 2452, 4096})
	
	stb := FLACSeektable{&FLACMetadataBlockHeader{3, 54, false, 3}, seekpointBlocks, true}
	c.Check(stb, Equals, metadata.FLACSeektable)
}
