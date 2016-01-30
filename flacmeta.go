// flacmeta.go - A library to process FLAC metadata.
// Copyright (C) 2012 Matthew White <mtw@vne.net>
//
// This program is free software; you can redistribute it and/or modify
// it under the terms of the GNU General Public License as published by
// the Free Software Foundation; either version 2 of the License, or (at
// your option) any later version.
//
// This program is distributed in the hope that it will be useful, but
// WITHOUT ANY WARRANTY; without even the implied warranty of MERCHANTABILITY
// or FITNESS FOR A PARTICULAR PURPOSE. See the GNU General Public License
// for more details.

// Package flac provides an API to process metadata from FLAC audio files.
package flac

import (
	"bytes"
	"encoding/binary"
	"fmt"
	"io"
)

// MetadataBlockTypes enumerates types of metadata blocks in a FLAC file.
type MetadataBlockType uint32

const (
	FlacSignature = "fLaC"

	MetadataStreaminfo MetadataBlockType = iota
	MetadataPadding
	MetadataApplication
	MetadataSeektable
	MetadataVorbisComment
	MetadataCuesheet
	MetadataPicture
	MetadataInvalid MetadataBlockType = 127

	// Metadata field sizes, in bits.
	ApplicationIdLen = 32

	CuesheetMediaCatalogNumberLen = 128 * 8
	CuesheetLeadinSamplesLen      = 64
	CuesheetTypeLen               = 1
	CuesheetReservedLen           = CuesheetTypeLen + 7 + 258*8
	CuesheetTotalTracksLen        = 8

	CuesheetTrackTrackOffsetLen = 64
	CuesheetTrackTrackNumberLen = 8
	CuesheetTrackTrackISRCLen   = 12 * 8
	CuesheetTrackTrackTypeLen   = 1
	CuesheetTrackPreemphasisLen = 1
	CuesheetTrackReservedLen    = CuesheetTrackTrackTypeLen + CuesheetTrackPreemphasisLen + 6 + 13*8
	CuesheetTrackIndexPointsLen = 8
	CuesheetTrackBlockLen       = (CuesheetTrackTrackOffsetLen +
		CuesheetTrackTrackNumberLen +
		CuesheetTrackTrackISRCLen +
		CuesheetTrackTrackTypeLen +
		CuesheetTrackPreemphasisLen +
		CuesheetTrackReservedLen +
		CuesheetTrackIndexPointsLen)

	CuesheetTrackIndexSampleOffsetLen = 64
	CuesheetTrackIndexPointLen        = 8
	CuesheetTrackIndexReservedLen     = 3 * 8
	CuesheetTrackIndexBlockLen        = (CuesheetTrackIndexSampleOffsetLen +
		CuesheetTrackIndexPointLen +
		CuesheetTrackIndexReservedLen)

	MetadataBlockHeaderLen = 32

	PictureTypeLen              = 32
	PictureMimeLengthLen        = 32
	PictureDescriptionLengthLen = 32
	PictureWidthLen             = 32
	PictureHeightLen            = 32
	PictureColorDepthLen        = 32
	PictureNumberOfColorsLen    = 32
	PictureLengthLen            = 32

	SeekpointSampleLen             = 64
	SeekpointSampleOffsetLen       = 64
	SeekpointTargetFrameSamplesLen = 16
	SeekpointBlockLen              = (SeekpointSampleLen +
		SeekpointSampleOffsetLen +
		SeekpointTargetFrameSamplesLen)

	StreaminfoMinBlockSizeLen  = 16
	StreaminfoMaxBlockSizeLen  = 16
	StreaminfoMinFrameSizeLen  = 24
	StreaminfoMaxFrameSizeLen  = 24
	StreaminfoSampleRateLen    = 20
	StreaminfoChannelCountLen  = 3
	StreaminfoBitsPerSampleLen = 6
	StreaminfoTotalSamplesLen  = 36
	StreaminfoMD5Len           = 128

	VorbisCommentVendorLen        = 32
	VorbisCommentUserCommentLen   = 32
	VorbisCommentCommentLengthLen = 32

	// Upper and lower bounds for metadata values, in bytes.
	StreaminfoMinBlockSizeMinimum  = 16
	StreaminfoMaxBlocKSizeMaximum  = 1 << StreaminfoMaxBlockSizeLen
	StreaminfoMinFrameSizeMinimum  = 1
	StreaminfoMaxFrameSizeMaximum  = 1 << StreaminfoMaxFrameSizeLen
	StreaminfoSampleRateMinimum    = 1
	StreaminfoSampleRateMaximum    = 1 << StreaminfoSampleRateLen
	StreaminfoChannelCountMinimum  = 1
	StreaminfoChannelCountMaximum  = 1 << StreaminfoChannelCountLen
	StreaminfoBitsPerSampleMinimum = 4
	StreaminfoBitsPerSampleMaximum = 1 << StreaminfoBitsPerSampleLen
	StreaminfoTotalSamplesMaximum  = 1 << StreaminfoTotalSamplesLen
)

// PictureTypes enumerates the types of pictures in a PictureBlock.
var PictureTypes = map[uint32]string{
	0:  "Other",
	1:  "File Icon",
	2:  "Other File Icon",
	3:  "Cover (front)",
	4:  "Cover (back)",
	5:  "Leaflet Page",
	6:  "Media",
	7:  "Lead Artist/Lead Performer/Soloist",
	8:  "Artist/Performer",
	9:  "Conductor",
	10: "Band/Orchestra",
	11: "Composer",
	12: "Lyricist/Text Writer",
	13: "Recording Location",
	14: "During Recording",
	15: "During Performance",
	16: "Movie/Video Screen Capture",
	17: "A Bright Coloured Fish",
	18: "Illustration",
	19: "Band/Artist Logotype",
	20: "Publisher/Studio Logotype",
}

// HeaderType returns a const representing a MetadataBlockType or Invalid for an unknown/undefined block type.
func HeaderType(i uint32) MetadataBlockType {
	switch i {
	case 0:
		return MetadataStreaminfo
	case 1:
		return MetadataPadding
	case 2:
		return MetadataApplication
	case 3:
		return MetadataSeektable
	case 4:
		return MetadataVorbisComment
	case 5:
		return MetadataCuesheet
	case 6:
		return MetadataPicture
	case 127:
		return MetadataInvalid
	}
	return MetadataInvalid
}

// PictureType looks up the type of a picture based on numeric id.
func PictureType(k uint32) string {
	t := PictureTypes[k]
	if t == "" {
		return "UNKNOWN"
	}
	return t
}

// String implements the Stringer interface for MetadataBlockTypes.
func (mbt MetadataBlockType) String() string {
	switch mbt {
	case MetadataStreaminfo:
		return "STREAMINFO"
	case MetadataPadding:
		return "PADDING"
	case MetadataApplication:
		return "APPLICATION"
	case MetadataSeektable:
		return "SEEKTABLE"
	case MetadataVorbisComment:
		return "VORBIS_COMMENT"
	case MetadataCuesheet:
		return "CUESHEET"
	case MetadataPicture:
		return "PICTURE"
	}
	return "INVALID"
}

// Begin base metadata block types.

// Application contains the ID and binary data of an embedded executable. Only one ApplicationBlock is allowed per file.
type ApplicationBlock struct {
	Id   uint32
	Data []byte
}

// Cuesheet contains information about an embedded cue sheet.
type CuesheetBlock struct {
	MediaCatalogNumber string
	LeadinSamples      uint64
	IsCompactDisc      bool
	TotalTracks        uint8
	Tracks             []*CuesheetTrack
	// Reserved           []byte
}

// CuesheetTrack represents an individual cue sheet track.
type CuesheetTrack struct {
	Offset      uint64
	Number      uint8
	ISRC        string
	Type        uint8
	PreEmphasis bool
	// Reserved             []byte // 6 + 13 * 8
	IndexPoints uint8
	Indexes     []*TrackIndex
}

// CuesheetTrackIndex represents the position of a track within the file.
type TrackIndex struct {
	SampleOffset uint64
	IndexPoint   uint8
	// Reserved     []byte // 3 * 8 bits. All bits must be set to zero.
}

// MetadataBlockHeader is the common element for every metadata block in a
// FLAC file. It describes the metadata block type, its length (in bytes), the
// number of seek points if the block is a seektable, and if it is the last
// metadata block before the start of the audio stream.
type MetadataBlockHeader struct {
	Type   MetadataBlockType
	Length uint32
	Last   bool
	// SeekPoints uint16
}

// Picture contains information and binary data about pictures that are embedded in the FLAC file. Muitiple Picture blocks are allow per file.
type PictureBlock struct {
	PictureType string
	MimeType    string
	Description string
	Width       uint32
	Height      uint32
	ColorDepth  uint32
	NumColors   uint32
	Length      uint32
	PictureBlob []byte
}

// Seekpoint contains locations within the FLAC file that allow an application to quickly jump to pre-defined locations in the audio stream.
type SeekpointBlock struct {
	SampleNumber uint64
	Offset       uint64
	FrameSamples uint16
}

// Streaminfo contains information about the audio stream. Only one StreaminfoBlock is allowed per file; it is also the only required block.
type StreaminfoBlock struct {
	MinBlockSize  uint16
	MaxBlockSize  uint16
	MinFrameSize  uint32
	MaxFrameSize  uint32
	SampleRate    uint32
	Channels      uint8
	BitsPerSample uint8
	TotalSamples  uint64
	MD5Signature  string
}

// VorbisComment contains information about the song/audio stream, such as Artist, Song Title, and Album. Only one VorbisCommentBlock is allowed per file.
type VorbisCommentBlock struct {
	Vendor        string
	TotalComments uint32
	Comments      []string
}

// Here start the complete metadata blocks: a struct containing a
// MetadataBlockHeader and the corresponding metadata block. Structs with
// an IsPopulated field imply that only one of this type of block is allowed in
// a FLAC file.

// Application is a full Application block (header + data).
type Application struct {
	Header      *MetadataBlockHeader
	Data        *ApplicationBlock
	IsPopulated bool
}

// Cuesheet is a cue sheet; details on multiple tracks within a single file.
type Cuesheet struct {
	Header      *MetadataBlockHeader
	Data        *CuesheetBlock
	IsPopulated bool
}

// Padding is a full Padding block (header + data).
type Padding struct {
	Header      *MetadataBlockHeader
	Data        []byte
	IsPopulated bool
}

// Picture is a full Picture block (header + data).
type Picture struct {
	Header      *MetadataBlockHeader
	Data        *PictureBlock
	IsPopulated bool
}

// Seektable is a full Seek Table block (header + data).
type Seektable struct {
	Header      *MetadataBlockHeader
	Data        []*SeekpointBlock
	IsPopulated bool
}

// Streaminfo is a full streaminfo block (header + data).
type Streaminfo struct {
	Header      *MetadataBlockHeader
	Data        *StreaminfoBlock
	IsPopulated bool
}

// VorbisComment is a full Vorbis Comment block (header + data).
type VorbisComment struct {
	Header      *MetadataBlockHeader
	Data        *VorbisCommentBlock
	IsPopulated bool
}

// Metadata represents all metadata present in a FLAC file.
type Metadata struct {
	Streaminfo
	Application
	VorbisComment
	Pictures []*Picture
	Padding
	Seektable
	Cuesheet
	TotalBlocks uint8
}

// MarshalApplicationBlock marshals b into an ApplicationBlock.
func MarshalApplicationBlock(b []byte) (*ApplicationBlock, error) {
	// http://flac.sourceforge.net/format.html#metadata_block_application
	// Field Len  | Data
	// -----------+--------------------------------------------------------
	// 32         | Registered application ID.
	//            |
	// n          | Application data (n must be a multiple of 8)

	buf := bytes.NewBuffer(b)
	blk := &ApplicationBlock{}

	blk.Id = binary.BigEndian.Uint32(buf.Next(ApplicationIdLen))
	if buf.Len()%8 != 0 {
		return nil, fmt.Errorf("malformed ApplictionBlock; field length %d not a multiple of 8", buf.Len())
	}
	blk.Data = buf.Bytes()
	return blk, nil
}

// MarshalCuesheetBlock marshals b into a CuesheetBlock.
func MarshalCuesheetBlock(b []byte) (*CuesheetBlock, error) {
	// http://flac.sourceforge.net/format.html#metadata_block_cuesheet
	// Field Len  | Data
	// -----------+--------------------------------------------------------
	// 128 * 8    | Media catalog number, in ASCII printable characters. In general,
	//            | the media catalog number may be 0 to 128 bytes long; any unused
	//   	      | characters should be right-padded with NUL characters. For CD-DA,
	//	          | this is a thirteen digit number, followed by 115 NUL bytes.
	//            |
	// 64         | The number of lead-in samples. This field has meaning only for CD-DA
	//            | cuesheets; for other uses it should be 0. For CD-DA, the lead-in is
	//            | the TRACK 00 area where the table of contents is stored; more precisely,
	//            | it is the number of samples from the first sample of the media to the
	//            | first sample of the first index point of the first track. According to
	//            | the Red Book, the lead-in must be silence and CD grabbing software
	//            | does not usually store it; additionally, the lead-in must be at least
	//            | two seconds but may be longer. For these reasons the lead-in length is
	//            | stored here so that the absolute position of the first track can be
	//            | computed. Note that the lead-in stored here is the number of samples
	//            | up to the first index point of the first track, not necessarily to INDEX
	//            | 01 of the first track; even the first track may have INDEX 00 data.
	//            |
	// 1          | 1 if the CUESHEET corresponds to a Compact Disc, else 0.
	//            |
	// 7 + 258 * 8| Reserved. All bits must be set to zero.
	//            |
	// 8          | The number of tracks. Must be at least 1 (because of the requisite
	//            | lead-out track). For CD-DA, this number must be no more than 100 (99 regular
	//            | tracks and one lead-out track).

	const trackType = 0x01
	buf := bytes.NewBuffer(b)
	blk := &CuesheetBlock{}

	blk.MediaCatalogNumber = string(buf.Next(CuesheetMediaCatalogNumberLen / 8))
	blk.LeadinSamples = binary.BigEndian.Uint64(buf.Next(CuesheetLeadinSamplesLen / 8))

	res := buf.Next(CuesheetReservedLen / 8)

	blk.IsCompactDisc = res[0]>>7&trackType == 1
	blk.TotalTracks = uint8(buf.Next(CuesheetTotalTracksLen / 8)[0])
	if blk.TotalTracks == 0 {
		return nil, fmt.Errorf("TotalTracks value must be greater than >= 1")
	}

	for i := 0; i < int(blk.TotalTracks); i++ {
		track, err := MarshalCuesheetTrack(buf.Next(CuesheetTrackBlockLen / 8))
		if err != nil {
			return nil, err
		}
		for j := 0; j < int(track.IndexPoints); j++ {
			index, err := MarshalCuesheetTrackIndex(buf.Next(CuesheetTrackIndexBlockLen / 8))
			if err != nil {
				return nil, err
			}
			track.Indexes = append(track.Indexes, index)
		}
		blk.Tracks = append(blk.Tracks, track)
	}
	return blk, nil
}

// MarshalCuesheetTrack marshals b into a CuesheetTrack.
func MarshalCuesheetTrack(b []byte) (*CuesheetTrack, error) {
	// http://flac.sourceforge.net/format.html#cuesheet_track
	// Field Len  | Data
	// -----------+--------------------------------------------------------
	// 64         | Track offset in samples, relative to the beginning of the FLAC audio stream.
	//            | It is the offset to the first index point of the track. (Note how this
	//            | differs from CD-DA, where the track's offset in the TOC is that of the track's
	//            | INDEX 01 even if there is an INDEX 00.) For CD-DA, the offset must be evenly
	//            | divisible by 588 samples (588 samples = 44100 samples/sec * 1/75th of a sec).
	//            |
	// 8          | Track number. A track number of 0 is not allowed to avoid
	//            | conflicting with the CD-DA spec, which reserves this for the
	//            | lead-in. For CD-DA the number must be 1-99, or 170 for the
	//            | lead-out; for non-CD-DA, the track number must for 255 for the
	//            | lead-out. It is not required but encouraged to start with track
	//            | 1 and increase sequentially. Track numbers must be unique within a CUESHEET.
	//            |
	// 12 * 8     | Track ISRC. This is a 12-digit alphanumeric code; see here[1] and here[2].
	//            | A value of 12 ASCII NUL characters may be used to denote absence of an ISRC.
	//            |
	// 1          | The track type: 0 for audio, 1 for non-audio. This corresponds to the
	//            | CD-DA Q-channel control bit 3.
	//            |
	// 1          | The pre-emphasis flag: 0 for no pre-emphasis, 1 for pre-emphasis. This
	//            | corresponds to the CD-DA Q-channel control bit 5; see here[3].
	//            |
	// 6 + 13 * 8 | Reserved. All bits must be set to zero.
	//            |
	// 8          | The number of track index points.  There must be at least one index in
	//            | every track in a CUESHEET except for the lead-out track, which must have
	//            | zero. For CD-DA, this number may be no more than 100.
	//            |
	//            | [1] http://www.ifpi.org/isrc/isrc_handbook.html
	//            | [2] http://en.wikipedia.org/wiki/International_Standard_Recording_Code
	//            | [3] http://www.chipchapin.com/CDMedia/cdda9.php3

	const trackType = 0x01
	buf := bytes.NewBuffer(b)

	blk := &CuesheetTrack{}

	blk.Offset = binary.BigEndian.Uint64(buf.Next(CuesheetTrackTrackOffsetLen / 8))

	if blk.Number = uint8(buf.Next(CuesheetTrackTrackNumberLen / 8)[0]); blk.Number == 0 {
		return nil, fmt.Errorf("cuesheet track value of 0 is not allowed")
	}
	blk.ISRC = string(buf.Next(CuesheetTrackTrackISRCLen / 8))

	// The first byte of the reserved field contain the flags for track type and preemphasis.
	res := buf.Next(CuesheetTrackReservedLen / 8)[0]
	blk.Type = uint8(res >> 7 & trackType)
	blk.PreEmphasis = res>>6&trackType == 1
	blk.IndexPoints = uint8(buf.Next(CuesheetTrackIndexPointsLen / 8)[0])

	return blk, nil
}

// MarshalCuesheetTrackIndex marshals b into a CuesheetTrackIndex.
func MarshalCuesheetTrackIndex(b []byte) (*TrackIndex, error) {
	// http://flac.sourceforge.net/format.html#cuesheet_track_index
	// Field Len  | Data
	// -----------+--------------------------------------------------------
	// 64         | Offset in samples, relative to the track offset, of the
	//            | index point. For CD-DA, the offset must be evenly divisible
	//            | by 588 samples (588 samples = 44100 samples/sec * 1/75th
	//            | of a sec). Note that the offset is from the beginning of
	//            | the track, not the beginning of the audio data.
	//            |
	// 8          | The index point number. For CD-DA, an index number of 0
	//            | corresponds to the track pre-gap. The first index in a track
	//            | must have a number of 0 or 1, and subsequently, index numbers
	//            | must increase by 1. Index numbers must be unique within a track.
	//            |
	// 3 * 8      | Reserved. All bits must be set to zero.

	buf := bytes.NewBuffer(b)
	blk := &TrackIndex{}

	blk.SampleOffset = binary.BigEndian.Uint64(buf.Next(CuesheetTrackIndexSampleOffsetLen / 8))
	if blk.SampleOffset%588 != 0 {
		return nil, fmt.Errorf("invalid value %d for Cuesheet Track Index Sample Offset: must be divisible by 588.", blk.SampleOffset)
	}
	blk.IndexPoint = uint8(buf.Next(CuesheetTrackIndexPointLen / 8)[0])
	return blk, nil
}

// MarshalMetadataBlockHeader marshals b into a MetadataBlockHeader.
func MarshalMetadataBlockHeader(b []byte) (*MetadataBlockHeader, error) {
	// http://flac.sourceforge.net/format.html#metadata_block_header
	// Field Len  | Data
	// -----------+--------------------------------------------------------
	// 1          | Last-metadata-block flag: '1' if this block is the last
	//            | metadata block before the audio blocks, '0' otherwise.
	//            |
	// 7          | BLOCK_TYPE
	//            |
	// 24         | Length (in bytes) of metadata to follow (does not include
	//            | the size of the METADATA_BLOCK_HEADER)

	hdr := &MetadataBlockHeader{}

	const (
		lastBlock = 0x80000000
		blockType = 0x7F000000
		blockLen  = 0x00FFFFFF
	)

	bits := binary.BigEndian.Uint32(b)

	hdr.Last = (lastBlock&bits)>>31 == 1
	hdr.Length = blockLen & bits

	bt := blockType & bits >> 24
	hdr.Type = HeaderType(bt)
	if hdr.Type == MetadataInvalid {
		return nil, fmt.Errorf("invalid or unknown block type: %d", bt)
	}

	return hdr, nil
}

// MarshalPictureBlock marshals b into a PictureBlock.
func MarshalPictureBlock(b []byte) *PictureBlock {
	// http://flac.sourceforge.net/format.html#metadata_block_picture
	// Field Len  | Data
	// -----------+--------------------------------------------------------
	// 32         | The picture type according to the ID3v2 APIC frame
	//            |
	// 32         | The length of the MIME type string in bytes.
	//            |
	// n * 8      | The MIME type string, in printable ASCII characters 0x20-0x7e.
	//            | The MIME type may also be --> to signify that the data part is
	//            | a URL of the picture instead of the picture data itself.
	//            |
	// 32         | The length of the description string in bytes.
	//            |
	// n * 8      | The description of the picture, in UTF-8.
	//            |
	// 32         | The width of the picture in pixels.
	//            |
	// 32         | The height of the picture in pixels.
	//            |
	// 32         | The color depth of the picture in bits-per-pixel.
	//            |
	// 32         | For indexed-color pictures (e.g. GIF), the number of colors used,
	//            | or 0 for non-indexed pictures.
	//            |
	// 32         | The length of the picture data in bytes.
	//            |
	// n * 8      | The binary picture data.

	buf := bytes.NewBuffer(b)
	blk := &PictureBlock{}

	blk.PictureType = PictureType(binary.BigEndian.Uint32(buf.Next(PictureTypeLen / 8)))

	picLength := int(binary.BigEndian.Uint32(buf.Next(PictureMimeLengthLen / 8)))
	blk.MimeType = string(buf.Next(picLength))

	picLength = int(binary.BigEndian.Uint32(buf.Next(PictureDescriptionLengthLen / 8)))
	if picLength > 0 {
		blk.Description = string(buf.Next(picLength))
	}
	blk.Width = binary.BigEndian.Uint32(buf.Next(PictureWidthLen / 8))
	blk.Height = binary.BigEndian.Uint32(buf.Next(PictureHeightLen / 8))
	blk.ColorDepth = binary.BigEndian.Uint32(buf.Next(PictureColorDepthLen / 8))
	blk.NumColors = binary.BigEndian.Uint32(buf.Next(PictureNumberOfColorsLen / 8))
	blk.Length = binary.BigEndian.Uint32(buf.Next(PictureLengthLen / 8))

	blk.PictureBlob = buf.Next(int(blk.Length))

	return blk
}

// TotalPoints returns the number of seek points in this Seektable.
func (s *Seektable) TotalPoints() int {
	return len(s.Data)
}

// MarshalSeekpointBlock marshals the contents b into a SeektableBlock.
func MarshalSeekpointBlock(b []byte) []*SeekpointBlock {
	// http://flac.sourceforge.net/format.html#seekpoint
	// Field Len  | Data
	// -----------+--------------------------------------------------------
	// 64         | Sample number of first sample in the target frame,
	//            | or 0xFFFFFFFFFFFFFFFF for a placeholder point.
	//            |
	// 64         | Offset (in bytes) from the first byte of the first frame
	//            | header to the first byte of the target frame's header.
	//            |
	// 16         | Number of samples in the target frame.
	// --------------------------------------------------------------------
	// Notes:
	//  - For placeholder points, the second and third field values are undefined.
	//  - Seek points within a table must be sorted in ascending order by sample number.
	//  - Seek points within a table must be unique by sample number, with the exception
	//    of placeholder points.
	//  - The previous two notes imply that there may be any number of placeholder points,
	//    but they must all occur at the end of the table.

	buf := bytes.NewBuffer(b)
	var ret []*SeekpointBlock

	for i := 0; buf.Len() > 0; i++ {
		spb := &SeekpointBlock{}
		binary.Read(buf, binary.BigEndian, spb)
		ret = append(ret, spb)
	}
	return ret
}

// MarshalStreaminfoBlock marshals b into a StreaminfoBlock.
func MarshalStreaminfoBlock(b []byte) (*StreaminfoBlock, error) {
	// http://flac.sourceforge.net/format.html#metadata_block_streaminfo
	// Field Len  | Data
	// -----------+--------------------------------------------------------
	// 16         | Minimum block size (in samples) used in the stream.
	//            |
	// 16         | Maximum block size (in samples) used in the stream.
	//            |
	// 24         | Minimum frame size (in bytes) used in the stream. 0 == Implied Unknown
	//            |
	// 24         | Maximum frame size (in bytes) used in the stream. 0 == Implied Unknown
	//            |
	// 20         | Sample rate (in Hz). Must be > 0 && < 655350
	//            |
	// 3          | Number of channels - 1.
	//            |
	// 5          | Bits per sample - 1.
	//            |
	// 36         | Total number of samples in the stream. 0 == Implied Unknown
	//            |
	// 128        | MD5 signature of the unencoded audio data.
	// --------------------------------------------------------------------
	// In order to keep everything on powers-of-2 boundaries, reads from the
	// block are grouped thus:
	//
	// 	MinBlockSize = 16 bits
	// 	MaxBlockSize + minFrameSize + maxFrameSize = 64 bits
	// 	SampleRate + channels + bitsPerSample + TotalSamples = 64 bits
	// 	md5Signature = 128 bits

	const (
		minFSMask       = 0xFFFFFFFFFFFFFFFF
		maxFSMask       = 0xFFFFFF
		sampRateMask    = 0xFFFFF00000000000
		bitsPerSampMask = 0x1F000000000
		chMask          = 0xE0000000000
		totSampMask     = 0xFFFFFFFFF
	)
	var bits uint64

	blk := &StreaminfoBlock{}
	buf := bytes.NewBuffer(b)

	blk.MinBlockSize = binary.BigEndian.Uint16(buf.Next(StreaminfoMinBlockSizeLen / 8))
	if blk.MinBlockSize > 0 && blk.MinBlockSize < 16 {
		return nil, fmt.Errorf("invalid MinBlockSize '%d'; must be >= 16", blk.MinBlockSize)
	}

	bfsLen := (StreaminfoMaxBlockSizeLen + StreaminfoMinFrameSizeLen + StreaminfoMaxFrameSizeLen) / 8
	bits = binary.BigEndian.Uint64(buf.Next(bfsLen))
	blk.MaxBlockSize = uint16((minFSMask & bits) >> 48)
	if blk.MaxBlockSize < 16 {
		return nil, fmt.Errorf("invalid MaxBlockSize '%d'; must be > 16", blk.MaxBlockSize)
	}

	blk.MinFrameSize = uint32((minFSMask & bits) >> 24)
	blk.MaxFrameSize = uint32(maxFSMask & bits)

	bits = binary.BigEndian.Uint64(buf.Next((StreaminfoSampleRateLen +
		StreaminfoChannelCountLen +
		StreaminfoBitsPerSampleLen +
		StreaminfoTotalSamplesLen) / 8))

	blk.SampleRate = uint32((sampRateMask & bits) >> 44)
	if blk.SampleRate == 0 || blk.SampleRate >= 655350 {
		return nil, fmt.Errorf("invalid SampleRate '%d'; must be > 0 and < 655350", blk.SampleRate)
	}
	blk.Channels = uint8((chMask&bits)>>41) + 1
	blk.BitsPerSample = uint8((bitsPerSampMask&bits)>>36) + 1
	blk.TotalSamples = bits & totSampMask
	blk.MD5Signature = fmt.Sprintf("%x", buf.Next(StreaminfoMD5Len/8))

	return blk, nil
}

// MarshalVorbisCommentBlock marshals b into a VorbisCommentBlock.
func MarshalVorbisCommentBlock(b []byte) *VorbisCommentBlock {
	// http://www.xiph.org/vorbis/doc/v-comment.html
	// The comment header is decoded as follows:
	//
	// 1) [vendor_length] = read an unsigned integer of 32 bits
	// 2) [vendor_string] = read a UTF-8 vector as [vendor_length] octets
	// 3) [user_comment_list_length] = read an unsigned integer of 32 bits
	// 4) iterate [user_comment_list_length] times {
	//      5) [length] = read an unsigned integer of 32 bits
	//      6) this iteration's user comment = read a UTF-8 vector as [length] octets
	//    }
	// 7) done.

	blk := &VorbisCommentBlock{}
	buf := bytes.NewBuffer(b)

	l := int(binary.LittleEndian.Uint32(buf.Next(VorbisCommentVendorLen / 8)))
	blk.Vendor = string(buf.Next(l))
	blk.TotalComments = binary.LittleEndian.Uint32(buf.Next(VorbisCommentUserCommentLen / 8))

	for tc := blk.TotalComments; tc > 0; tc-- {
		l := int(binary.LittleEndian.Uint32(buf.Next(VorbisCommentCommentLengthLen / 8)))
		comment := string(buf.Next(l))
		blk.Comments = append(blk.Comments, comment)
	}
	return blk
}

// Read reads the metadata from a FLAC file and populates a Metadata struct.
func (m *Metadata) Read(f io.Reader) error {
	// First 4 bytes of the file are the FLAC stream marker: 0x66, 0x4C, 0x61, 0x43
	// It's also the length of all metadata block headers so we'll resue it below.
	h := make([]byte, MetadataBlockHeaderLen/8)

	n, err := io.ReadFull(f, h)
	if err != nil || n != int(MetadataBlockHeaderLen/8) {
		return fmt.Errorf("error reading FLAC signature: %v", err)
	}

	if string(h) != FlacSignature {
		return fmt.Errorf("%q is not a valid FLAC signature", h)
	}

	for totalMBH := 0; ; totalMBH++ {
		// Next 4 bytes after the stream marker is the first metadata block header.
		n, err := io.ReadFull(f, h)
		if err != nil || n != int(MetadataBlockHeaderLen/8) {
			return fmt.Errorf("error reading metadata block header: %v", err)
		}

		mbh, err := MarshalMetadataBlockHeader(h)
		if err != nil {
			return fmt.Errorf("failed to marshalMetadataBlockHeader: %v", err)
		}

		block := make([]byte, mbh.Length)
		n, err = io.ReadFull(f, block)
		if err != nil {
			return fmt.Errorf("failed to read %d of %d bytes for %s metadata block: %v", n, mbh.Length, mbh.Type, err)
		}

		switch mbh.Type {
		case MetadataStreaminfo:
			if m.Streaminfo.IsPopulated {
				return fmt.Errorf("two %s blocks encountered", mbh.Type)
			}
			sib, err := MarshalStreaminfoBlock(block)
			if err != nil {
				return err
			}
			m.Streaminfo = Streaminfo{mbh, sib, true}

		case MetadataVorbisComment:
			if m.VorbisComment.IsPopulated {
				return fmt.Errorf("two %s blocks encountered", mbh.Type)
			}
			vcb := MarshalVorbisCommentBlock(block)
			m.VorbisComment = VorbisComment{mbh, vcb, true}

		case MetadataPicture:
			fpb := MarshalPictureBlock(block)
			m.Pictures = append(m.Pictures, &Picture{mbh, fpb, true})

		case MetadataPadding:
			if m.Padding.IsPopulated {
				return fmt.Errorf("two %s blocks encountered", mbh.Type)
			}
			m.Padding = Padding{mbh, nil, true}

		case MetadataApplication:
			if m.Application.IsPopulated {
				return fmt.Errorf("two %s blocks encountered", mbh.Type)
			}
			ab, err := MarshalApplicationBlock(block)
			if err != nil {
				return err
			}
			m.Application = Application{mbh, ab, true}

		case MetadataSeektable:
			if m.Seektable.IsPopulated {
				return fmt.Errorf("two %s blocks encountered", mbh.Type)
			}
			if len(block)%(SeekpointBlockLen/8) != 0 {
				return fmt.Errorf("%s block length is not a multiple of %d", mbh.Type, (SeekpointBlockLen / 8))
			}

			st := MarshalSeekpointBlock(block)
			m.Seektable = Seektable{mbh, st, true}

		case MetadataCuesheet:
			if m.Cuesheet.IsPopulated {
				return fmt.Errorf("two %s blocks encountered", mbh.Type)
			}

			cb, err := MarshalCuesheetBlock(block)
			if err != nil {
				return err
			}
			m.Cuesheet = Cuesheet{mbh, cb, true}

		default:
			continue
		}

		if mbh.Last {
			break
		}
	}
	return nil
}
