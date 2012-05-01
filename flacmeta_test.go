package flac

import (
	"fmt"
	. "launchpad.net/gocheck"
	"os"
	"testing"
)

func Test(t *testing.T) { TestingT(t) }

type S struct{}

var _ = Suite(&S{})

func (s *S) TestParseMetadataBlockHeader1(c *C) {
	f, err := os.Open("testdata/44100-16-mono.flac")
	if err != nil {
		fmt.Println("FATAL:", err)
		os.Exit(-1)
	}
	defer f.Close()

	metadata := new(Metadata)
	err = metadata.Read(f); if err != nil {
		fmt.Printf("Error reading test file: %s\n", err)
		os.Exit(-1)
	}

	streaminfo := Streaminfo{
		Header: &MetadataBlockHeader{
			Type:       STREAMINFO,
			Length:     34,
			Last:       false,
			SeekPoints: 0},
		Data: &StreaminfoBlock{
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
	c.Check(metadata.Streaminfo.Header, DeepEquals, streaminfo.Header)
	c.Check(metadata.Streaminfo.Data, DeepEquals, streaminfo.Data)
	c.Check(metadata.Streaminfo.IsPopulated, DeepEquals, streaminfo.IsPopulated)

	comment := VorbisComment{
		Header: &MetadataBlockHeader{
			Type:       VORBIS_COMMENT,
			Length:     57,
			Last:       false,
			SeekPoints: 0},
		Data: &VorbisCommentBlock{
			Vendor:        "reference libFLAC 1.2.1 20070917",
			TotalComments: 1,
			Comments: []string{
				"ARTIST=GoGoGo"}},
		IsPopulated: true}
	c.Check(metadata.VorbisComment.Header, DeepEquals, comment.Header)
	c.Check(metadata.VorbisComment.Data, DeepEquals, comment.Data)
	c.Check(metadata.VorbisComment.IsPopulated, DeepEquals, comment.IsPopulated)

	pad := Padding{
		Header: &MetadataBlockHeader{
			Type:       PADDING,
			Length:     8175,
			Last:       true,
			SeekPoints: 0},
		Data:        nil,
		IsPopulated: true}
	c.Check(pad, DeepEquals, metadata.Padding)
	c.Check(metadata.Padding, DeepEquals, pad)

	stb := Seektable{
		Header: &MetadataBlockHeader{
			Type:       SEEKTABLE,
			Length:     54,
			Last:       false,
			SeekPoints: 3},
		Data: []*SeekpointBlock{
			&SeekpointBlock{
				SampleNumber: 0,
				Offset:       0,
				FrameSamples: 4096},
			&SeekpointBlock{
				SampleNumber: 438272,
				Offset:       1177,
				FrameSamples: 4096},
			&SeekpointBlock{
				SampleNumber: 880640,
				Offset:       2452,
				FrameSamples: 4096}},
		IsPopulated: true}
	c.Check(metadata.Seektable.Header, DeepEquals, stb.Header)
	c.Check(metadata.Seektable.Data, DeepEquals, stb.Data)
	c.Check(metadata.Seektable.IsPopulated, DeepEquals, stb.IsPopulated)
}

func (s *S) TestParseMetadataBlockHeader2(c *C) {
	f, err := os.Open("testdata/mutagen/silence-44-s.flac")
	if err != nil {
		fmt.Println("FATAL:", err)
		os.Exit(-1)
	}
	defer f.Close()

	metadata := new(Metadata)
	metadata.Read(f)

	// Test Streaminfo Block
	streaminfo := Streaminfo{
		Header: &MetadataBlockHeader{
			Type:       STREAMINFO,
			Length:     34,
			Last:       false,
			SeekPoints: 0},
		Data: &StreaminfoBlock{
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
	c.Check(metadata.Streaminfo.Header, DeepEquals, streaminfo.Header)
	c.Check(metadata.Streaminfo.Data, DeepEquals, streaminfo.Data)
	c.Check(metadata.Streaminfo.IsPopulated, DeepEquals, streaminfo.IsPopulated)

	// Test Vorbis Comments
	comment := VorbisComment{
		Header: &MetadataBlockHeader{
			Type:       VORBIS_COMMENT,
			Length:     169,
			Last:       false,
			SeekPoints: 0},
		Data: &VorbisCommentBlock{
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
	c.Check(metadata.VorbisComment.Header, DeepEquals, comment.Header)
	c.Check(metadata.VorbisComment.Data, DeepEquals, comment.Data)
	c.Check(metadata.VorbisComment.IsPopulated, DeepEquals, comment.IsPopulated)

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

	cti0 := make([]*CuesheetTrackIndexBlock, 1)
	cti0[0] = &CuesheetTrackIndexBlock{
		SampleOffset: 0,
		IndexPoint:   1,
		Reserved:     []byte{0x00, 0x00, 0x00}}

	cti1 := make([]*CuesheetTrackIndexBlock, 2)
	cti1[0] = &CuesheetTrackIndexBlock{
		SampleOffset: 0,
		IndexPoint:   1,
		Reserved:     []byte{0x00, 0x00, 0x00}}
	cti1[1] = &CuesheetTrackIndexBlock{
		SampleOffset: 588,
		IndexPoint:   2,
		Reserved:     []byte{0x00, 0x00, 0x00}}

	cti2 := make([]*CuesheetTrackIndexBlock, 1)
	cti2[0] = &CuesheetTrackIndexBlock{
		SampleOffset: 0,
		IndexPoint:   1,
		Reserved:     []byte{0x00, 0x00, 0x00}}

	res0 := []byte{0x00, 0x0, 0x0, 0x0, 0x0, 0x0, 0x0, 0x0, 0x0, 0x0, 0x0, 0x0, 0x0, 0x0}
	res1 := []byte{0xc0, 0x0, 0x0, 0x0, 0x0, 0x0, 0x0, 0x0, 0x0, 0x0, 0x0, 0x0, 0x0, 0x0}
	emptyIRSC := "\x00\x00\x00\x00\x00\x00\x00\x00\x00\x00\x00\x00"

	ctb := make([]*CuesheetTrackBlock, 4)
	ctb[0] = &CuesheetTrackBlock{
		TrackOffset:          0,
		TrackNumber:          uint8(1),
		TrackISRC:            "123456789012",
		TrackType:            0,
		PreEmphasis:          false,
		Reserved:             res0,
		IndexPoints:          uint8(1),
		CuesheetTrackIndexes: cti0}
	ctb[1] = &CuesheetTrackBlock{
		TrackOffset:          44100,
		TrackNumber:          2,
		TrackISRC:            emptyIRSC,
		TrackType:            1,
		PreEmphasis:          true,
		Reserved:             res1,
		IndexPoints:          2,
		CuesheetTrackIndexes: cti1}
	ctb[2] = &CuesheetTrackBlock{
		TrackOffset:          88200,
		TrackNumber:          3,
		TrackISRC:            emptyIRSC,
		TrackType:            0,
		PreEmphasis:          false,
		Reserved:             res0,
		IndexPoints:          1,
		CuesheetTrackIndexes: cti2}
	ctb[3] = &CuesheetTrackBlock{
		TrackOffset:          162496,
		TrackNumber:          170,
		TrackISRC:            emptyIRSC,
		TrackType:            0,
		PreEmphasis:          false,
		Reserved:             res0,
		IndexPoints:          0,
		CuesheetTrackIndexes: nil}

	cb := Cuesheet{
		Header: &MetadataBlockHeader{
			Type:       CUESHEET,
			Length:     588,
			Last:       false,
			SeekPoints: 0},
		Data: &CuesheetBlock{
			MediaCatalogNumber: isrc,
			LeadinSamples:      0x15888,
			IsCompactDisc:      true,
			Reserved:           cbReserved,
			TotalTracks:        0x4,
			CuesheetTracks:     ctb},
		IsPopulated: true}
	c.Check(metadata.Cuesheet.Header, DeepEquals, cb.Header)
	c.Check(metadata.Cuesheet.IsPopulated, DeepEquals, cb.IsPopulated)
	for x := range ctb {
		c.Check(metadata.Cuesheet.Data.CuesheetTracks[x], DeepEquals, ctb[x])
		for y := range metadata.Cuesheet.Data.CuesheetTracks[x].CuesheetTrackIndexes {
			c.Check(metadata.Cuesheet.Data.CuesheetTracks[x].CuesheetTrackIndexes[y], DeepEquals, ctb[x].CuesheetTrackIndexes[y])
		}
	}

	// Test Seek Table
	stb := Seektable{
		Header: &MetadataBlockHeader{
			Type:       SEEKTABLE,
			Length:     108,
			Last:       false,
			SeekPoints: 6},
		Data: []*SeekpointBlock{
			&SeekpointBlock{
				SampleNumber: 0,
				Offset:       0,
				FrameSamples: 4608},
			&SeekpointBlock{
				SampleNumber: 41472,
				Offset:       11852,
				FrameSamples: 4608},
			&SeekpointBlock{
				SampleNumber: 50688,
				Offset:       14484,
				FrameSamples: 4608},
			&SeekpointBlock{
				SampleNumber: 87552,
				Offset:       25022,
				FrameSamples: 4608},
			&SeekpointBlock{
				SampleNumber: 105984,
				Offset:       30284,
				FrameSamples: 4608},
			&SeekpointBlock{
				SampleNumber: 18446744073709551615,
				Offset:       0,
				FrameSamples: 0}},
		IsPopulated: true}
	c.Check(metadata.Seektable.Header, DeepEquals, stb.Header)
	c.Check(metadata.Seektable.Data, DeepEquals, stb.Data)
	c.Check(metadata.Seektable.IsPopulated, DeepEquals, stb.IsPopulated)

	// Test Picture
	pb := Picture{
		Header: &MetadataBlockHeader{
			Type:       PICTURE,
			Length:     199,
			Last:       false,
			SeekPoints: 0},
		Data: &PictureBlock{
			PictureType:        "Cover (front)",
			MimeType:           "image/png",
			PictureDescription: "A pixel.",
			Width:              1,
			Height:             1,
			ColorDepth:         24,
			NumColors:          0,
			Length:             150},
		IsPopulated: true}
	c.Check(metadata.Pictures[0].Header, DeepEquals, pb.Header)
	c.Check(metadata.Pictures[0].Data.PictureType, DeepEquals, pb.Data.PictureType)
	c.Check(metadata.Pictures[0].Data.MimeType, DeepEquals, pb.Data.MimeType)
	c.Check(metadata.Pictures[0].Data.PictureDescription, DeepEquals, pb.Data.PictureDescription)
	c.Check(metadata.Pictures[0].Data.Width, DeepEquals, pb.Data.Width)
	c.Check(metadata.Pictures[0].Data.Height, DeepEquals, pb.Data.Height)
	c.Check(metadata.Pictures[0].Data.ColorDepth, DeepEquals, pb.Data.ColorDepth)
	c.Check(metadata.Pictures[0].Data.NumColors, DeepEquals, pb.Data.NumColors)
	c.Check(metadata.Pictures[0].Data.Length, DeepEquals, pb.Data.Length)
	c.Check(metadata.Pictures[0].IsPopulated, DeepEquals, pb.IsPopulated)

	// Test Padding
	pad := Padding{
		Header: &MetadataBlockHeader{
			Type:       PADDING,
			Length:     3060,
			Last:       true,
			SeekPoints: 0},
		Data:        nil,
		IsPopulated: true}
	c.Check(metadata.Padding.Header, DeepEquals, pad.Header)
	c.Check(metadata.Padding.Data, DeepEquals, pad.Data)
	c.Check(metadata.Padding.IsPopulated, DeepEquals, pad.IsPopulated)
}
