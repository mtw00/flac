package flac

import (
	"encoding/hex"
	"os"
	"reflect"
	"testing"
)

func TestParseMetadata44k16Mono(t *testing.T) {
	f, err := os.Open("testdata/44100-16-mono.flac")
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()

	got := new(Metadata)
	if err := got.Read(f); err != nil {
		t.Fatal(err)
	}

	wantSi := Streaminfo{
		Header: &MetadataBlockHeader{
			Type:   MetadataStreaminfo,
			Length: 34,
			Last:   false,
		},
		Data: &StreaminfoBlock{
			MinBlockSize:  4096,
			MaxBlockSize:  4096,
			MinFrameSize:  11,
			MaxFrameSize:  14,
			SampleRate:    44100,
			Channels:      1,
			BitsPerSample: 16,
			TotalSamples:  1014300,
			MD5Signature:  "e5ccc967ced6c111530e5c79e33c969e",
		},
		IsPopulated: true,
	}
	if !reflect.DeepEqual(got.Streaminfo, wantSi) {
		t.Errorf("Streaminfo headers differ:\ngot:  %+v\nwant: %+v", got, wantSi)
	}

	wantVc := VorbisComment{
		Header: &MetadataBlockHeader{
			Type:   MetadataVorbisComment,
			Length: 57,
			Last:   false,
		},
		Data: &VorbisCommentBlock{
			Vendor:        "reference libFLAC 1.2.1 20070917",
			TotalComments: 1,
			Comments:      []string{"ARTIST=GoGoGo"},
		},
		IsPopulated: true,
	}
	if !reflect.DeepEqual(got.VorbisComment, wantVc) {
		t.Errorf("VorbisComment headers differ:\ngot:  %+v\nwant: %+v", got.VorbisComment, wantVc)
	}

	wantPad := Padding{
		Header: &MetadataBlockHeader{
			Type:   MetadataPadding,
			Length: 8175,
			Last:   true,
		},
		Data:        nil,
		IsPopulated: true,
	}
	if !reflect.DeepEqual(got.Padding, wantPad) {
		t.Errorf("Padding differs:\ngot:  %+v\nwant: %+v", got.Padding, wantPad)
	}

	wantSt := Seektable{
		Header: &MetadataBlockHeader{
			Type:   MetadataSeektable,
			Length: 54,
			Last:   false,
		},
		Data: []*SeekpointBlock{
			{SampleNumber: 0, Offset: 0, FrameSamples: 4096},
			{SampleNumber: 438272, Offset: 1177, FrameSamples: 4096},
			{SampleNumber: 880640, Offset: 2452, FrameSamples: 4096},
		},
		IsPopulated: true,
	}
	if !reflect.DeepEqual(got.Seektable, wantSt) {
		t.Errorf("Seektable differs:\ngot:  %+v\nwant: %+v", got.Seektable, wantSt)
	}
	if got, want := got.Seektable.TotalPoints(), wantSt.TotalPoints(); got != want {
		t.Errorf("Seektable TotalPoints differ: got %d, want %d", got, want)
	}
}

func TestParseMetadata44k16Stereo(t *testing.T) {
	f, err := os.Open("testdata/silence-44-s.flac")
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()

	got := new(Metadata)
	got.Read(f)

	wantSi := Streaminfo{
		Header: &MetadataBlockHeader{
			Type:   MetadataStreaminfo,
			Length: 34,
			Last:   false,
		},
		Data: &StreaminfoBlock{
			MinBlockSize:  4608,
			MaxBlockSize:  4608,
			MinFrameSize:  633,
			MaxFrameSize:  1323,
			SampleRate:    44100,
			Channels:      2,
			BitsPerSample: 16,
			TotalSamples:  162496,
			MD5Signature:  "6291dbd8dcb7dc480132e4c4ba154a17",
		},
		IsPopulated: true,
	}
	if !reflect.DeepEqual(got.Streaminfo, wantSi) {
		t.Errorf("Streaminfo differs:\ngot:  %+v\nwant: %+v", got.Streaminfo, wantSi)
	}

	wantVc := VorbisComment{
		Header: &MetadataBlockHeader{
			Type:   MetadataVorbisComment,
			Length: 169,
			Last:   false,
		},
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
				"title=Silence",
			},
		},
		IsPopulated: true,
	}
	if !reflect.DeepEqual(got.VorbisComment, wantVc) {
		t.Errorf("VorbisComment differs:\ngot:  %+v\nwant: %+v", got.VorbisComment, wantVc)
	}

	isrc := "1234567890123"
	for i := len(isrc); i < 128; i++ {
		isrc += "\x00"
	}

	emptyIRSC := "\x00\x00\x00\x00\x00\x00\x00\x00\x00\x00\x00\x00"

	wantCs := Cuesheet{
		Header: &MetadataBlockHeader{
			Type:   MetadataCuesheet,
			Length: 588,
			Last:   false,
		},
		Data: &CuesheetBlock{
			MediaCatalogNumber: isrc,
			LeadinSamples:      0x15888,
			IsCompactDisc:      true,
			TotalTracks:        0x4,
			Tracks: []*CuesheetTrack{{
				Offset:      0,
				Number:      uint8(1),
				ISRC:        "123456789012",
				Type:        0,
				PreEmphasis: false,
				IndexPoints: uint8(1),
				Indexes: []*TrackIndex{
					{SampleOffset: 0, IndexPoint: 1},
				},
			}, {
				Offset:      44100,
				Number:      2,
				ISRC:        emptyIRSC,
				Type:        1,
				PreEmphasis: true,
				IndexPoints: 2,
				Indexes: []*TrackIndex{
					{SampleOffset: 0, IndexPoint: 1},
					{SampleOffset: 588, IndexPoint: 2},
				},
			}, {
				Offset:      88200,
				Number:      3,
				ISRC:        emptyIRSC,
				Type:        0,
				PreEmphasis: false,
				IndexPoints: 1,
				Indexes: []*TrackIndex{
					{SampleOffset: 0, IndexPoint: 1},
				},
			}, {
				Offset:      162496,
				Number:      170,
				ISRC:        emptyIRSC,
				Type:        0,
				PreEmphasis: false,
				IndexPoints: 0,
				Indexes:     nil,
			}},
		},
		IsPopulated: true,
	}
	if !reflect.DeepEqual(got.Cuesheet, wantCs) {
		t.Errorf("Cuesheet differs:\ngot:  %+v\nwant: %+v", got.Cuesheet, wantCs)
	}

	wantSt := Seektable{
		Header: &MetadataBlockHeader{
			Type:   MetadataSeektable,
			Length: 108,
			Last:   false,
		},
		Data: []*SeekpointBlock{
			{SampleNumber: 0, Offset: 0, FrameSamples: 4608},
			{SampleNumber: 41472, Offset: 11852, FrameSamples: 4608},
			{SampleNumber: 50688, Offset: 14484, FrameSamples: 4608},
			{SampleNumber: 87552, Offset: 25022, FrameSamples: 4608},
			{SampleNumber: 105984, Offset: 30284, FrameSamples: 4608},
			{SampleNumber: 18446744073709551615, Offset: 0, FrameSamples: 0},
		},
		IsPopulated: true,
	}
	if !reflect.DeepEqual(got.Seektable, wantSt) {
		t.Errorf("Cuesheet differs:\ngot:  %+v\nwant: %+v", got.Seektable, wantSt)
	}

	blob, err := hex.DecodeString(`89504e470d0a1a0a0000000d4948445200000001000000010802000000907753de000000097048597300000b1300000b1301009a9c180000000774494d4507d60b1c0a360608443d320000001d74455874436f6d6d656e7400437265617465642077697468205468652047494d50ef64256e0000000c4944415408d763f8ffff3f0005fe02fedccc59e70000000049454e44ae426082`)
	if err != nil {
		t.Fatalf("failed to decode hex representation of picture: %v", err)
	}
	wantPic := &Picture{
		Header: &MetadataBlockHeader{
			Type:   MetadataPicture,
			Length: 199,
			Last:   false,
		},
		Data: &PictureBlock{
			PictureType: "Cover (front)",
			MimeType:    "image/png",
			Description: "A pixel.",
			Width:       1,
			Height:      1,
			ColorDepth:  24,
			NumColors:   0,
			Length:      150,
			PictureBlob: blob,
		},
		IsPopulated: true,
	}
	if !reflect.DeepEqual(got.Pictures[0], wantPic) {
		t.Errorf("Pictures differs:\ngot:  %+v\nwant: %+v", got.Pictures[0], wantPic)
	}

	wantPad := Padding{
		Header: &MetadataBlockHeader{
			Type:   MetadataPadding,
			Length: 3060,
			Last:   true,
		},
		IsPopulated: true,
	}
	if !reflect.DeepEqual(got.Padding, wantPad) {
		t.Errorf("Padding differs:\ngot:  %+v\nwant: %+v", got.Padding, wantPad)
	}
}
