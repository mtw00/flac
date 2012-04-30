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
	ok, readerr := metadata.Read(f)
	if !ok {
		fmt.Printf("Error reading test file: %s\n", readerr)
		os.Exit(-1)
	}

	streaminfo := FLACStreaminfo{
		Header: &FLACMetadataBlockHeader{
			Type:       STREAMINFO,
			Length:     34,
			Last:       false,
			SeekPoints: 0},
		Data: &FLACStreaminfoBlock{
			MinBlockSize:  4096,
			MaxBlockSize:  4096,
			MinFrameSize:  11,
			MaxFrameSize:  14,
			SampleRate:    44100,
			Channels:      1,
			BitsPerSample: 16,
			TotalSamples:  1014300,
			MD5Signature:  "e5ccc967ced6c111530e5c79e33c969e"},
		IsPopulated: true}
	c.Check(metadata.FLACStreaminfo.Header, DeepEquals, streaminfo.Header)
	c.Check(metadata.FLACStreaminfo.Data, DeepEquals, streaminfo.Data)
	c.Check(metadata.FLACStreaminfo.IsPopulated, DeepEquals, streaminfo.IsPopulated)

	comment := FLACVorbisComment{
		Header: &FLACMetadataBlockHeader{
			Type:       VORBIS_COMMENT,
			Length:     57,
			Last:       false,
			SeekPoints: 0},
		Data: &FLACVorbisCommentBlock{
			Vendor:        "reference libFLAC 1.2.1 20070917",
			TotalComments: 1,
			Comments: []string{
				"ARTIST=GoGoGo"}},
		IsPopulated: true}
	c.Check(metadata.FLACVorbisComment.Header, DeepEquals, comment.Header)
	c.Check(metadata.FLACVorbisComment.Data, DeepEquals, comment.Data)
	c.Check(metadata.FLACVorbisComment.IsPopulated, DeepEquals, comment.IsPopulated)

	pad := FLACPadding{
		Header: &FLACMetadataBlockHeader{
			Type:       PADDING,
			Length:     8175,
			Last:       true,
			SeekPoints: 0},
		Data:        nil,
		IsPopulated: true}
	c.Check(pad, DeepEquals, metadata.FLACPadding)

	stb := FLACSeektable{
		Header: &FLACMetadataBlockHeader{
			Type:       SEEKTABLE,
			Length:     54,
			Last:       false,
			SeekPoints: 3},
		Data: []*FLACSeekpointBlock{
			&FLACSeekpointBlock{
				SampleNumber: 0,
				Offset:       0,
				FrameSamples: 4096},
			&FLACSeekpointBlock{
				SampleNumber: 438272,
				Offset:       1177,
				FrameSamples: 4096},
			&FLACSeekpointBlock{
				SampleNumber: 880640,
				Offset:       2452,
				FrameSamples: 4096}},
		IsPopulated: true}
	c.Check(metadata.FLACSeektable.Header, DeepEquals, stb.Header)
	c.Check(metadata.FLACSeektable.Data, DeepEquals, stb.Data)
	c.Check(metadata.FLACSeektable.IsPopulated, DeepEquals, stb.IsPopulated)
}

func (s *S) TestFLACParseMetadataBlockHeader2(c *C) {
	f, err := os.Open("testdata/mutagen/silence-44-s.flac")
	if err != nil {
		fmt.Println("FATAL:", err)
		os.Exit(-1)
	}
	defer f.Close()

	metadata := new(FLACMetadata)
	metadata.Read(f)

	// Test Streaminfo Block
	streaminfo := FLACStreaminfo{
		Header: &FLACMetadataBlockHeader{
			Type:       STREAMINFO,
			Length:     34,
			Last:       false,
			SeekPoints: 0},
		Data: &FLACStreaminfoBlock{
			MinBlockSize:  4608,
			MaxBlockSize:  4608,
			MinFrameSize:  633,
			MaxFrameSize:  1323,
			SampleRate:    44100,
			Channels:      2,
			BitsPerSample: 16,
			TotalSamples:  162496,
			MD5Signature:  "6291dbd8dcb7dc480132e4c4ba154a17"},
		IsPopulated: true}
	c.Check(metadata.FLACStreaminfo.Header, DeepEquals, streaminfo.Header)
	c.Check(metadata.FLACStreaminfo.Data, DeepEquals, streaminfo.Data)
	c.Check(metadata.FLACStreaminfo.IsPopulated, DeepEquals, streaminfo.IsPopulated)

	// Test Vorbis Comments
	comment := FLACVorbisComment{
		Header: &FLACMetadataBlockHeader{
			Type:       VORBIS_COMMENT,
			Length:     169,
			Last:       false,
			SeekPoints: 0},
		Data: &FLACVorbisCommentBlock{
			Vendor:        "reference libFLAC 1.1.0 20030126",
			TotalComments: 7,
			Comments: []string{
				"album=Quod Libet Test Data",
				"artist=piman",
				"artist=jzig",
				"genre=Silence",
				"tracknumber=02/10",
				"date=2004",
				"title=Silence"}},
		IsPopulated: true}
	c.Check(metadata.FLACVorbisComment.Header, DeepEquals, comment.Header)
	c.Check(metadata.FLACVorbisComment.Data, DeepEquals, comment.Data)
	c.Check(metadata.FLACVorbisComment.IsPopulated, DeepEquals, comment.IsPopulated)

	// Test Cuesheet Block
	isrc := ""
	isrc = "1234567890123"
	for i := len(isrc); i < 128; i++ {
		isrc += "\x00"
	}

	// First bit in the Reserved field specifies if this is a CD cuesheet or not.
	cbReserved := make([]byte, 259)
	for i := range cbReserved {
		if i == 0 {
			cbReserved[i] = 0x80
			continue
		}
		cbReserved[i] = 0x0
	}

	cti0 := make([]*FLACCuesheetTrackIndexBlock, 1)
	cti0[0] = &FLACCuesheetTrackIndexBlock{
		SampleOffset: 0,
		IndexPoint:   1,
		Reserved:     []byte{0x00, 0x00, 0x00}}

	cti1 := make([]*FLACCuesheetTrackIndexBlock, 2)
	cti1[0] = &FLACCuesheetTrackIndexBlock{
		SampleOffset: 0,
		IndexPoint:   1,
		Reserved:     []byte{0x00, 0x00, 0x00}}
	cti1[1] = &FLACCuesheetTrackIndexBlock{
		SampleOffset: 588,
		IndexPoint:   2,
		Reserved:     []byte{0x00, 0x00, 0x00}}

	cti2 := make([]*FLACCuesheetTrackIndexBlock, 1)
	cti2[0] = &FLACCuesheetTrackIndexBlock{
		SampleOffset: 0,
		IndexPoint:   1,
		Reserved:     []byte{0x00, 0x00, 0x00}}

	res0 := []byte{0x00, 0x0, 0x0, 0x0, 0x0, 0x0, 0x0, 0x0, 0x0, 0x0, 0x0, 0x0, 0x0, 0x0}
	res1 := []byte{0xc0, 0x0, 0x0, 0x0, 0x0, 0x0, 0x0, 0x0, 0x0, 0x0, 0x0, 0x0, 0x0, 0x0}
	emptyIRSC := "\x00\x00\x00\x00\x00\x00\x00\x00\x00\x00\x00\x00"

	ctb := make([]*FLACCuesheetTrackBlock, 4)
	ctb[0] = &FLACCuesheetTrackBlock{
		TrackOffset:              0,
		TrackNumber:              uint8(1),
		TrackISRC:                "123456789012",
		TrackType:                0,
		PreEmphasis:              false,
		Reserved:                 res0,
		IndexPoints:              uint8(1),
		FLACCuesheetTrackIndexes: cti0}
	ctb[1] = &FLACCuesheetTrackBlock{
		TrackOffset:              44100,
		TrackNumber:              2,
		TrackISRC:                emptyIRSC,
		TrackType:                1,
		PreEmphasis:              true,
		Reserved:                 res1,
		IndexPoints:              2,
		FLACCuesheetTrackIndexes: cti1}
	ctb[2] = &FLACCuesheetTrackBlock{
		TrackOffset:              88200,
		TrackNumber:              3,
		TrackISRC:                emptyIRSC,
		TrackType:                0,
		PreEmphasis:              false,
		Reserved:                 res0,
		IndexPoints:              1,
		FLACCuesheetTrackIndexes: cti2}
	ctb[3] = &FLACCuesheetTrackBlock{
		TrackOffset:              162496,
		TrackNumber:              170,
		TrackISRC:                emptyIRSC,
		TrackType:                0,
		PreEmphasis:              false,
		Reserved:                 res0,
		IndexPoints:              0,
		FLACCuesheetTrackIndexes: nil}

	cb := FLACCuesheet{
		Header: &FLACMetadataBlockHeader{
			Type:       CUESHEET,
			Length:     588,
			Last:       false,
			SeekPoints: 0},
		Data: &FLACCuesheetBlock{
			MediaCatalogNumber: isrc,
			LeadinSamples:      0x15888,
			IsCompactDisc:      true,
			Reserved:           cbReserved,
			TotalTracks:        0x4,
			FLACCuesheetTracks: ctb},
		IsPopulated: true}
	c.Check(metadata.FLACCuesheet.Header, DeepEquals, cb.Header)
	c.Check(metadata.FLACCuesheet.IsPopulated, DeepEquals, cb.IsPopulated)
	for x := range ctb {
		c.Check(metadata.FLACCuesheet.Data.FLACCuesheetTracks[x], DeepEquals, ctb[x])
		for y := range metadata.FLACCuesheet.Data.FLACCuesheetTracks[x].FLACCuesheetTrackIndexes {
			c.Check(metadata.FLACCuesheet.Data.FLACCuesheetTracks[x].FLACCuesheetTrackIndexes[y], DeepEquals, ctb[x].FLACCuesheetTrackIndexes[y])
		}
	}

	// Test Seek Table
	stb := FLACSeektable{
		Header: &FLACMetadataBlockHeader{
			Type:       SEEKTABLE,
			Length:     108,
			Last:       false,
			SeekPoints: 6},
		Data: []*FLACSeekpointBlock{
			&FLACSeekpointBlock{
				SampleNumber: 0,
				Offset:       0,
				FrameSamples: 4608},
			&FLACSeekpointBlock{
				SampleNumber: 41472,
				Offset:       11852,
				FrameSamples: 4608},
			&FLACSeekpointBlock{
				SampleNumber: 50688,
				Offset:       14484,
				FrameSamples: 4608},
			&FLACSeekpointBlock{
				SampleNumber: 87552,
				Offset:       25022,
				FrameSamples: 4608},
			&FLACSeekpointBlock{
				SampleNumber: 105984,
				Offset:       30284,
				FrameSamples: 4608},
			&FLACSeekpointBlock{
				SampleNumber: 18446744073709551615,
				Offset:       0,
				FrameSamples: 0}},
		IsPopulated: true}
	c.Check(metadata.FLACSeektable.Header, DeepEquals, stb.Header)
	c.Check(metadata.FLACSeektable.Data, DeepEquals, stb.Data)
	c.Check(metadata.FLACSeektable.IsPopulated, DeepEquals, stb.IsPopulated)

	// Test Picture
	pb := FLACPicture{
		Header: &FLACMetadataBlockHeader{
			Type:       PICTURE,
			Length:     199,
			Last:       false,
			SeekPoints: 0},
		Data: &FLACPictureBlock{
			PictureType:        "Cover (front)",
			MimeType:           "image/png",
			PictureDescription: "A pixel.",
			Width:              1,
			Height:             1,
			ColorDepth:         24,
			NumColors:          0,
			Length:             150},
		IsPopulated: true}
	c.Check(metadata.FLACPictures[0].Header, DeepEquals, pb.Header)
	c.Check(metadata.FLACPictures[0].Data.PictureType, DeepEquals, pb.Data.PictureType)
	c.Check(metadata.FLACPictures[0].Data.MimeType, DeepEquals, pb.Data.MimeType)
	c.Check(metadata.FLACPictures[0].Data.PictureDescription, DeepEquals, pb.Data.PictureDescription)
	c.Check(metadata.FLACPictures[0].Data.Width, DeepEquals, pb.Data.Width)
	c.Check(metadata.FLACPictures[0].Data.Height, DeepEquals, pb.Data.Height)
	c.Check(metadata.FLACPictures[0].Data.ColorDepth, DeepEquals, pb.Data.ColorDepth)
	c.Check(metadata.FLACPictures[0].Data.NumColors, DeepEquals, pb.Data.NumColors)
	c.Check(metadata.FLACPictures[0].Data.Length, DeepEquals, pb.Data.Length)
	c.Check(metadata.FLACPictures[0].IsPopulated, DeepEquals, pb.IsPopulated)

	// Test Padding
	pad := FLACPadding{
		Header: &FLACMetadataBlockHeader{
			Type:       PADDING,
			Length:     3060,
			Last:       true,
			SeekPoints: 0},
		Data:        nil,
		IsPopulated: true}
	c.Check(metadata.FLACPadding.Header, DeepEquals, pad.Header)
	c.Check(metadata.FLACPadding.Data, DeepEquals, pad.Data)
	c.Check(metadata.FLACPadding.IsPopulated, DeepEquals, pad.IsPopulated)
}
