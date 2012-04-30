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


// Package flacmeta provides an API to process metadata from FLAC audio files.
package flacmeta

// TODO: make NewZZZ functions to create Header+Data blocks
import (
	"bytes"
	"encoding/binary"
	"encoding/hex"
	"fmt"
	"io"
	"strings"
)

// METADATA_BLOCK_TYPES enumerates types of metadata blocks in a FLAC file.
type METADATA_BLOCK_TYPE uint32

const (
	FLAC_SIGNATURE string = "fLaC"

	STREAMINFO     METADATA_BLOCK_TYPE = iota // 0
	PADDING                                   // 1
	APPLICATION                               // 2
	SEEKTABLE                                 // 3
	VORBIS_COMMENT                            // 4
	CUESHEET                                  // 5
	PICTURE                                   // 6
	INVALID        METADATA_BLOCK_TYPE = 127

	// Metadata field sizes, in bits.
	METADATA_BLOCK_HEADER_LEN int = 32

	APPLICATION_ID_LEN int = 32

	CUESHEET_MEDIA_CATALOG_NUMBER_LEN int = 128 * 8
	CUESHEET_LEADIN_SAMPLES_LEN       int = 64
	CUESHEET_TYPE_LEN                 int = 1
	CUESHEET_RESERVED_LEN             int = CUESHEET_TYPE_LEN + 7 + 258*8
	CUESHEET_TOTAL_TRACKS_LEN         int = 8

	CUESHEET_TRACK_TRACK_OFFSET_LEN int = 64
	CUESHEET_TRACK_TRACK_NUMBER_LEN int = 8
	CUESHEET_TRACK_TRACK_ISRC_LEN   int = 12 * 8
	CUESHEET_TRACK_TRACK_TYPE_LEN   int = 1
	CUESHEET_TRACK_PREEMPHASIS_LEN  int = 1
	CUESHEET_TRACK_RESERVED_LEN     int = CUESHEET_TRACK_TRACK_TYPE_LEN + CUESHEET_TRACK_PREEMPHASIS_LEN + 6 + 13*8
	CUESHEET_TRACK_INDEX_POINTS_LEN int = 8
	CUESHEET_TRACK_BLOCK_LEN        int = (CUESHEET_TRACK_TRACK_OFFSET_LEN +
		CUESHEET_TRACK_TRACK_NUMBER_LEN +
		CUESHEET_TRACK_TRACK_ISRC_LEN +
		CUESHEET_TRACK_TRACK_TYPE_LEN +
		CUESHEET_TRACK_PREEMPHASIS_LEN +
		CUESHEET_TRACK_RESERVED_LEN +
		CUESHEET_TRACK_INDEX_POINTS_LEN)

	CUESHEET_TRACK_INDEX_SAMPLE_OFFSET_LEN int = 64
	CUESHEET_TRACK_INDEX_INDEX_POINT_LEN   int = 8
	CUESHEET_TRACK_INDEX_RESERVED_LEN      int = 3 * 8
	CUESHEET_TRACK_INDEX_BLOCK_LEN         int = (CUESHEET_TRACK_INDEX_SAMPLE_OFFSET_LEN +
		CUESHEET_TRACK_INDEX_INDEX_POINT_LEN +
		CUESHEET_TRACK_INDEX_RESERVED_LEN)

	PICTURE_TYPE_LEN               int = 32
	PICTURE_MIME_LENGTH_LEN        int = 32
	PICTURE_DESCRIPTION_LENGTH_LEN int = 32
	PICTURE_WIDTH_LEN              int = 32
	PICTURE_HEIGHT_LEN             int = 32
	PICTURE_COLOR_DEPTH_LEN        int = 32
	PICTURE_NUMBER_OF_COLORS_LEN   int = 32
	PICTURE_LENGTH_LEN             int = 32

	SEEKPOINT_SAMPLE_NUMBER_LEN        int = 64
	SEEKPOINT_SAMPLE_OFFSET_LEN        int = 64
	SEEKPOINT_TARGET_FRAME_SAMPLES_LEN int = 16
	SEEKPOINT_BLOCK_LEN                int = (SEEKPOINT_SAMPLE_NUMBER_LEN +
		SEEKPOINT_SAMPLE_OFFSET_LEN +
		SEEKPOINT_TARGET_FRAME_SAMPLES_LEN)

	STREAMINFO_MIN_BLOCK_SIZE_LEN  int = 16
	STREAMINFO_MAX_BLOCK_SIZE_LEN  int = 16
	STREAMINFO_MIN_FRAME_SIZE_LEN  int = 24
	STREAMINFO_MAX_FRAME_SIZE_LEN  int = 24
	STREAMINFO_SAMPLE_RATE_LEN     int = 20
	STREAMINFO_CHANNEL_COUNT_LEN   int = 3
	STREAMINFO_BITS_PER_SAMPLE_LEN int = 6
	STREAMINFO_TOTAL_SAMPLES_LEN   int = 36
	STREAMINFO_MD5_LEN             int = 128

	VORBIS_COMMENT_VENDOR_LEN            int = 32
	VORBIS_COMMENT_USER_COMMENT_LIST_LEN int = 32
	VORBIS_COMMENT_COMMENT_LENGTH_LEN    int = 32

	// Upper and lower bounds for metadata values, in bytes.
	STREAMINFO_MIN_BLOCK_SIZE_MINIMUM  uint32 = 16
	STREAMINFO_MAX_BLOCK_SIZE_MAXIMUM  uint32 = 1 << uint(STREAMINFO_MAX_BLOCK_SIZE_LEN)
	STREAMINFO_MIN_FRAME_SIZE_MINIMUM  uint32 = 1
	STREAMINFO_MAX_FRAME_SIZE_MAXIMUM  uint32 = 1 << uint(STREAMINFO_MAX_FRAME_SIZE_LEN)
	STREAMINFO_SAMPLE_RATE_MINIMUM     uint32 = 1
	STREAMINFO_SAMPLE_RATE_MAXIMUM     uint32 = 1 << uint(STREAMINFO_SAMPLE_RATE_LEN)
	STREAMINFO_CHANNEL_COUNT_MINIMUM   uint8  = 1
	STREAMINFO_CHANNEL_COUNT_MAXIMUM   uint8  = 1 << uint(STREAMINFO_CHANNEL_COUNT_LEN)
	STREAMINFO_BITS_PER_SAMPLE_MINIMUM uint8  = 4
	STREAMINFO_BITS_PER_SAMPLE_MAXIMUM uint8  = 1 << uint(STREAMINFO_BITS_PER_SAMPLE_LEN)
	STREAMINFO_TOTAL_SAMPLES_MAXIMUM   uint64 = 1 << uint(STREAMINFO_TOTAL_SAMPLES_LEN)
)

// PictureTypeMap enumerates the types of pictures in a FLACPictureBlock.
var PictureTypeMap = map[uint32]string{
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

// LookupHeaderType returns a const representing a METADATA_BLOCK_TYPE or
// INVALID for unknown/undefined block type.
func LookupHeaderType(i uint32) METADATA_BLOCK_TYPE {
	switch i {
	case 0:
		return STREAMINFO
	case 1:
		return PADDING
	case 2:
		return APPLICATION
	case 3:
		return SEEKTABLE
	case 4:
		return VORBIS_COMMENT
	case 5:
		return CUESHEET
	case 6:
		return PICTURE
	case 127:
		return INVALID
	}
	return INVALID
}

// LookupPictureType looks up the type of a picture based on numeric id.
func LookupPictureType(k uint32) string {
	t := PictureTypeMap[k]
	switch t {
	case "":
		return "UNKNOWN"
	}
	return t
}

// Implement fmt.Stringer() to print the string representation of METADATA_BLOCK_TYPEs.
func (mbt METADATA_BLOCK_TYPE) String() string {
	switch {
	case mbt == STREAMINFO:
		return "STREAMINFO"
	case mbt == PADDING:
		return "PADDING"
	case mbt == APPLICATION:
		return "APPLICATION"
	case mbt == SEEKTABLE:
		return "SEEKTABLE"
	case mbt == VORBIS_COMMENT:
		return "VORBIS_COMMENT"
	case mbt == CUESHEET:
		return "CUESHEET"
	case mbt == PICTURE:
		return "PICTURE"
	}
	return "INVALID"
}

// Implement fmt.GoStringer() to print the string and integer representation of METADATA_BLOCK_TYPEs.
func (mbt METADATA_BLOCK_TYPE) GoString() string {
	switch {
	case mbt == STREAMINFO:
		return fmt.Sprintf("%d (%s)", int(STREAMINFO), STREAMINFO)
	case mbt == PADDING:
		return fmt.Sprintf("%d (%s)", int(PADDING), PADDING)
	case mbt == APPLICATION:
		return fmt.Sprintf("%d (%s)", int(APPLICATION), APPLICATION)
	case mbt == SEEKTABLE:
		return fmt.Sprintf("%d (%s)", int(SEEKTABLE), SEEKTABLE)
	case mbt == VORBIS_COMMENT:
		return fmt.Sprintf("%d (%s)", int(VORBIS_COMMENT), VORBIS_COMMENT)
	case mbt == CUESHEET:
		return fmt.Sprintf("%d (%s)", int(CUESHEET), CUESHEET)
	case mbt == PICTURE:
		return fmt.Sprintf("%d (%s)", int(PICTURE), PICTURE)
	}
	return fmt.Sprintf("%d (%s)", int(INVALID), PICTURE)
}

// Begin base metadata block types.

// FLACApplicationBlock contains the ID and binary data of an embedded executable.
// Only one ApplicationBlock is allowed per file.
type FLACApplicationBlock struct {
	Id   uint32
	Data []byte
}

type FLACCuesheetBlock struct {
	MediaCatalogNumber string
	LeadinSamples      uint64
	IsCompactDisc      bool
	Reserved           []byte
	TotalTracks        uint8 // > 1 && < 100 (for CD-DA)
	FLACCuesheetTracks []*FLACCuesheetTrackBlock
}

type FLACCuesheetTrackBlock struct {
	TrackOffset              uint64
	TrackNumber              uint8
	TrackISRC                string
	TrackType                uint8
	PreEmphasis              bool
	Reserved                 []byte // 6 + 13 * 8
	IndexPoints              uint8
	FLACCuesheetTrackIndexes []*FLACCuesheetTrackIndexBlock
}

type FLACCuesheetTrackIndexBlock struct {
	SampleOffset uint64
	IndexPoint   uint8
	Reserved     []byte // 3 * 8 bits. All bits must be set to zero.
}

// FLACMetadataBlockHeader is the common element for every metadata block in a
// FLAC file. It describes the metadata block type, its length (in bytes), the
// number of seek points if the block is a seektable, and if it is the last
// metadata block before the start of the audio stream.
type FLACMetadataBlockHeader struct {
	Type       METADATA_BLOCK_TYPE
	Length     uint32
	Last       bool
	SeekPoints uint16
}

// FLACPictureBlock contains information and binary data about pictures that
// are embedded in the FLAC file. Muitiple PictureBlocks are allow per file.
type FLACPictureBlock struct {
	PictureType        string
	MimeType           string
	PictureDescription string
	Width              uint32
	Height             uint32
	ColorDepth         uint32
	NumColors          uint32
	Length             uint32
	PictureBlob        string
}

// FLACSeekpointBlock contains locations within the FLAC file that allow
// an application to quickly jump to pre-defined locations in the audio stream.
type FLACSeekpointBlock struct {
	SampleNumber uint64
	Offset       uint64
	FrameSamples uint16
}

// FLACStreaminfoBlock contains information about the audio stream.
// Only one StreaminfoBlock is allowed per file. It is also the only required block.
type FLACStreaminfoBlock struct {
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

// FLACVorbisCommentBlock contains general information about the song/audio stream.
// Common fields are Artist, Song Title and Album.
// Only one VorbisCommentBlock is allowed per file.
type FLACVorbisCommentBlock struct {
	Vendor        string
	TotalComments uint32
	Comments      []string
}

// Here start the complete metadata blocks: a struct containing a
// FLACMetadataBlockHeader and the corresponding metadata block. Structs with
// an IsPopulated field imply that only one of this type of block is allowed in
// a FLAC file.

// FLACApplication is a full Application block (header + data).
type FLACApplication struct {
	Header      *FLACMetadataBlockHeader
	Data        *FLACApplicationBlock
	IsPopulated bool
}

// FLACCuesheet describes a "Cue sheet".
type FLACCuesheet struct {
	Header      *FLACMetadataBlockHeader
	Data        *FLACCuesheetBlock
	IsPopulated bool
}

// FLACPadding is a full Padding block (header + data).
type FLACPadding struct {
	Header      *FLACMetadataBlockHeader
	Data        []byte
	IsPopulated bool
}

// FLACPicture is a full Picture block (header + data).
type FLACPicture struct {
	Header      *FLACMetadataBlockHeader
	Data        *FLACPictureBlock
	IsPopulated bool
}

// FLACSeektable is a full Seek Table block (header + data).
type FLACSeektable struct {
	Header      *FLACMetadataBlockHeader
	Data        []*FLACSeekpointBlock
	IsPopulated bool
}

// FLACStreaminfo is a full streaminfo block (header + data).
type FLACStreaminfo struct {
	Header      *FLACMetadataBlockHeader
	Data        *FLACStreaminfoBlock
	IsPopulated bool
}

// FLACVorbisComment is a full Vorbis Comment block (header + data).
type FLACVorbisComment struct {
	Header      *FLACMetadataBlockHeader
	Data        *FLACVorbisCommentBlock
	IsPopulated bool
}

// FLACMetadata represents all metadata present in a FLAC file.
type FLACMetadata struct {
	FLACStreaminfo
	FLACApplication
	FLACVorbisComment
	FLACPictures []*FLACPicture
	FLACPadding
	FLACSeektable
	FLACCuesheet
	TotalBlocks uint8
}

// Begin FLACParseX functions.

// FLACParseApplicationBlock parses the bits of a FLAC Application block.
func (ab *FLACApplicationBlock) FLACParseApplicationBlock(block []byte) (bool, string) {
	// http://flac.sourceforge.net/format.html#metadata_block_application
	// Field Len  | Data
	// -----------+--------------------------------------------------------
	// 32         | Registered application ID.
	//            |
	// n          | Application data (n must be a multiple of 8)

	buf := bytes.NewBuffer(block)

	ab.Id = binary.BigEndian.Uint32(buf.Next(APPLICATION_ID_LEN))
	if buf.Len()%8 != 0 {
		return false, fmt.Sprintf("Malformed APPLICATON_METADATA_BLOCK: the data field length is not a mulitple of 8.")
	}
	ab.Data = buf.Bytes()
	return true, ""
}

// FLACParseCuesheetBlock parses the bits of a FLAC Cue Sheet block.
func (cb *FLACCuesheetBlock) FLACParseCuesheetBlock(block []byte) (bool, string) {
	// http://flac.sourceforge.net/format.html#metadata_block_cuesheet
	// Field Len  | Data
	// -----------+--------------------------------------------------------
	// 128 * 8    | Media catalog number, in ASCII printable characters. In general,
	//            | the media catalog number may be 0 to 128 bytes long; any unused
	//	      | characters should be right-padded with NUL characters. For CD-DA,
	//	      | this is a thirteen digit number, followed by 115 NUL bytes.
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

	var TRACKTYPE uint8 = 0x01
	buf := bytes.NewBuffer(block)

	cb.MediaCatalogNumber = string(buf.Next(CUESHEET_MEDIA_CATALOG_NUMBER_LEN / 8))
	if len(cb.MediaCatalogNumber) != CUESHEET_MEDIA_CATALOG_NUMBER_LEN/8 {
		return false, fmt.Sprintf("FATAL: read %d bytes for a MediaCatalogNumber; expected %d.", len(cb.MediaCatalogNumber), CUESHEET_MEDIA_CATALOG_NUMBER_LEN/8)
	}

	samplesVal := buf.Next(CUESHEET_LEADIN_SAMPLES_LEN / 8)
	if len(samplesVal) != CUESHEET_LEADIN_SAMPLES_LEN/8 {
		return false, fmt.Sprintf("FATAL: read %d bytes for a LeadinSamples field; expected %d.", len(samplesVal), CUESHEET_LEADIN_SAMPLES_LEN/8)
	}

	cb.LeadinSamples = uint64(binary.BigEndian.Uint64(samplesVal))

	reservedVal := buf.Next(CUESHEET_RESERVED_LEN / 8)
	if len(reservedVal) != CUESHEET_RESERVED_LEN/8 {
		return false, fmt.Sprintf("FATAL: read %d bytes for a Reserved field; expected %d.", len(reservedVal), CUESHEET_RESERVED_LEN/8)
	}

	cb.Reserved = reservedVal

	isCD := reservedVal[0] >> 7 & TRACKTYPE
	if isCD == 1 {
		cb.IsCompactDisc = true
	}

	tracksVal := buf.Next(CUESHEET_TOTAL_TRACKS_LEN / 8)
	if len(tracksVal) != CUESHEET_TOTAL_TRACKS_LEN/8 {
		return false, fmt.Sprintf("FATAL: read %d bytes for a TotalTracks field; expected %d.", len(reservedVal), CUESHEET_TOTAL_TRACKS_LEN/8)
	}

	cb.TotalTracks = uint8(tracksVal[0])
	if cb.TotalTracks < 1 {
		return false, fmt.Sprintf("FATAL: FLACCuesheetBlock.TotalTracks value must be greater than >= 1.")
	}

	for i := 0; i < int(cb.TotalTracks); i++ {
		cb.FLACParseCuesheetTrackBlock(buf.Next((CUESHEET_TRACK_BLOCK_LEN / 8)))
		for j := 0; j < int(cb.FLACCuesheetTracks[i].IndexPoints); j++ {
			cb.FLACCuesheetTracks[i].FLACParseCuesheetTrackIndexBlock(buf.Next(CUESHEET_TRACK_INDEX_BLOCK_LEN / 8))
		}
	}

	return true, ""
}

// FLACParseCuesheetTrackBlock parses the bits of a FLAC Cue Sheet Track block.
func (cb *FLACCuesheetBlock) FLACParseCuesheetTrackBlock(block []byte) (bool, string) {
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

	var TRACKTYPE uint8 = 0x01
	buf := bytes.NewBuffer(block)

	ctb := new(FLACCuesheetTrackBlock)

	offsetVal := buf.Next(CUESHEET_TRACK_TRACK_OFFSET_LEN / 8)
	if len(offsetVal) != CUESHEET_TRACK_TRACK_OFFSET_LEN/8 {
		return false, fmt.Sprintf("FATAL: read %d bytes for a TrackOffset field; expected %d.", len(offsetVal), CUESHEET_TOTAL_TRACKS_LEN/8)
	}
	ctb.TrackOffset = uint64(binary.BigEndian.Uint64(offsetVal))

	tnumberVal := buf.Next(CUESHEET_TRACK_TRACK_NUMBER_LEN / 8)
	if len(tnumberVal) != CUESHEET_TRACK_TRACK_NUMBER_LEN/8 {
		return false, fmt.Sprintf("FATAL: read %d bytes for a TrackNumber field; expected %d.", len(tnumberVal), CUESHEET_TRACK_TRACK_NUMBER_LEN/8)
	}
	ctb.TrackNumber = uint8(tnumberVal[0])

	isrcVal := buf.Next(CUESHEET_TRACK_TRACK_ISRC_LEN / 8)
	if len(isrcVal) != CUESHEET_TRACK_TRACK_ISRC_LEN/8 {
		return false, fmt.Sprintf("FATAL: read %d bytes for a TrackNumber field; expected %d.", len(tnumberVal), CUESHEET_TRACK_TRACK_ISRC_LEN/8)
	}
	ctb.TrackISRC = string(isrcVal)

	ctb.Reserved = buf.Next(CUESHEET_TRACK_RESERVED_LEN / 8)

	ctb.TrackType = uint8(ctb.Reserved[0] >> 7 & TRACKTYPE)

	if uint8(ctb.Reserved[0]>>6&TRACKTYPE) == 1 {
		ctb.PreEmphasis = true
	}

	pointsVal := buf.Next(CUESHEET_TRACK_INDEX_POINTS_LEN / 8)
	ctb.IndexPoints = uint8(pointsVal[0])
	cb.FLACCuesheetTracks = append(cb.FLACCuesheetTracks, ctb)

	return true, ""
}

// FLACParseCuesheetTrackIndexBlock parses the bits of a Cue Sheet Track Index block.
func (ctb *FLACCuesheetTrackBlock) FLACParseCuesheetTrackIndexBlock(block []byte) (bool, string) {
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

	buf := bytes.NewBuffer(block)
	oneByte := make([]byte, 1)

	cti := new(FLACCuesheetTrackIndexBlock)

	cti.SampleOffset = binary.BigEndian.Uint64(buf.Next(CUESHEET_TRACK_INDEX_SAMPLE_OFFSET_LEN / 8))

	oneByte = buf.Next(CUESHEET_TRACK_INDEX_INDEX_POINT_LEN / 8)
	cti.IndexPoint = uint8(oneByte[0])
	cti.Reserved = buf.Next(CUESHEET_TRACK_INDEX_RESERVED_LEN / 8)

	ctb.FLACCuesheetTrackIndexes = append(ctb.FLACCuesheetTrackIndexes, cti)

	return true, ""
}

// FLACParseMetadataBlockHeader parses the bits of a FLAC Metadata Block Header.
func (mbh *FLACMetadataBlockHeader) FLACParseMetadataBlockHeader(block []byte) (bool, string) {
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

	var LASTBLOCK uint32 = 0x80000000
	var BLOCKTYPE uint32 = 0x7F000000
	var BLOCKLEN uint32 = 0x00FFFFFF

	bits := binary.BigEndian.Uint32(block)

	blktype := BLOCKTYPE & bits >> 24
	mbh.Type = LookupHeaderType(blktype)
	if mbh.Type == INVALID {
		return false, fmt.Sprintf("FATAL: Encountered an invalid or unknown block type: %d.", blktype)
	}
	mbh.Length = BLOCKLEN & bits
	mbh.Last = false
	if (LASTBLOCK&bits)>>31 == 1 {
		mbh.Last = true
	}
	if mbh.Type == SEEKTABLE {
		if mbh.Length%uint32(SEEKPOINT_BLOCK_LEN/8) != 0 {
			return false, fmt.Sprintf("SEEKTABLE block length is not a multiple of %d.", SEEKPOINT_BLOCK_LEN/8)
		}
		mbh.SeekPoints = uint16(mbh.Length / uint32(SEEKPOINT_BLOCK_LEN/8))
	}
	return true, ""
}

// FLACParsePictureBlock parses the bits of a FLAC picture block.
func (pb *FLACPictureBlock) FLACParsePictureBlock(block []byte) (bool, string) {
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

	buf := bytes.NewBuffer(block)

	pb.PictureType = LookupPictureType(binary.BigEndian.Uint32(buf.Next(PICTURE_TYPE_LEN / 8)))

	mimeLen := int(binary.BigEndian.Uint32(buf.Next(PICTURE_MIME_LENGTH_LEN / 8)))
	pb.MimeType = string(buf.Next(mimeLen))

	descLen := int(binary.BigEndian.Uint32(buf.Next(PICTURE_DESCRIPTION_LENGTH_LEN / 8)))
	if descLen > 0 {
		pb.PictureDescription = string(buf.Next(descLen))
	} else {
		pb.PictureDescription = ""
	}
	pb.Width = binary.BigEndian.Uint32(buf.Next(PICTURE_LENGTH_LEN / 8))
	pb.Height = binary.BigEndian.Uint32(buf.Next(PICTURE_HEIGHT_LEN / 8))
	pb.ColorDepth = binary.BigEndian.Uint32(buf.Next(PICTURE_COLOR_DEPTH_LEN / 8))
	pb.NumColors = binary.BigEndian.Uint32(buf.Next(PICTURE_NUMBER_OF_COLORS_LEN / 8))
	pb.Length = binary.BigEndian.Uint32(buf.Next(PICTURE_LENGTH_LEN / 8))
	pb.PictureBlob = hex.Dump(buf.Next(int(pb.Length)))

	return true, ""
}

// FLACParseSeekpointBlock parses the bits of a FLAC seekpoint block.
func (stb *FLACSeektable) FLACParseSeekpointBlock(block []byte) (bool, string) {
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

	buf := bytes.NewBuffer(block)

	for i := 0; buf.Len() > 0; i++ {
		spb := new(FLACSeekpointBlock)
		binary.Read(buf, binary.BigEndian, spb)

		// These got replaced by binary.Read
		//spb.SampleNumber = binary.BigEndian.Uint64(buf.Next(SEEKPOINT_SAMPLE_NUMBER_LEN / 8))
		//spb.Offset = binary.BigEndian.Uint64(buf.Next(SEEKPOINT_SAMPLE_OFFSET_LEN / 8))
		//spb.FrameSamples = binary.BigEndian.Uint16(buf.Next(SEEKPOINT_TARGET_FRAME_SAMPLES_LEN / 8))

		stb.Data = append(stb.Data, spb)
	}

	return true, ""
}

// FLACParseStreaminfoBlock parses the bits of a FLAC streaminfo block.
func (sib *FLACStreaminfoBlock) FLACParseStreaminfoBlock(block []byte) (bool, string) {
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
	// 3          | Number of channels - 1. Why -1?
	//            |
	// 5          | Bits per sample - 1. Why -1?
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

	buf := bytes.NewBuffer(block)

	var (
		bits            uint64
		minFSMask       uint64 = 0xFFFFFFFFFFFFFFFF
		maxFSMask       uint64 = 0xFFFFFF
		sampRateMask    uint64 = 0xFFFFF00000000000
		bitsPerSampMask uint64 = 0x1F000000000
		chMask          uint64 = 0xE0000000000
		totSampMask     uint64 = 0xFFFFFFFFF
	)

	mbs := buf.Next(STREAMINFO_MIN_BLOCK_SIZE_LEN / 8)
	if len(mbs) != STREAMINFO_MIN_BLOCK_SIZE_LEN/8 {
		return false, fmt.Sprintf("FATAL: error reading MinBlockSize field. Expected %d byte(s), got %d.", STREAMINFO_MIN_BLOCK_SIZE_LEN/8, len(mbs))
	}
	sib.MinBlockSize = binary.BigEndian.Uint16(mbs)
	if sib.MinBlockSize > 0 && sib.MinBlockSize < 16 {
		return false, fmt.Sprintf("FATAL: invalid MinBlockSize %d. Must be >= 16.", sib.MinBlockSize)
	}

	bfsLen := (STREAMINFO_MAX_BLOCK_SIZE_LEN + STREAMINFO_MIN_FRAME_SIZE_LEN + STREAMINFO_MAX_FRAME_SIZE_LEN) / 8
	bfs := buf.Next(bfsLen)
	if len(bfs) != bfsLen {
		return false, fmt.Sprintf("FATAL: error reading MaxBlockSize, MinFrameSize and MaxFrameSize fields. Expected %d byte(s), got %d.", bfsLen, len(bfs))
	}

	bits = binary.BigEndian.Uint64(bfs)
	sib.MaxBlockSize = uint16((minFSMask & bits) >> 48)
	if sib.MaxBlockSize > 0 && sib.MaxBlockSize < 16 {
		return false, fmt.Sprintf("FATAL: invalid MaxBlockSize %d. Must be > 0 and > 16.", sib.MaxBlockSize)
	}
	sib.MinFrameSize = uint32((minFSMask & bits) >> 24)
	sib.MaxFrameSize = uint32(maxFSMask & bits)

	bits = binary.BigEndian.Uint64(buf.Next((STREAMINFO_SAMPLE_RATE_LEN +
		STREAMINFO_CHANNEL_COUNT_LEN +
		STREAMINFO_BITS_PER_SAMPLE_LEN +
		STREAMINFO_TOTAL_SAMPLES_LEN) / 8))
	sib.SampleRate = uint32((sampRateMask & bits) >> 44)
	if sib.SampleRate == 0 || sib.SampleRate >= 655350 {
		return false, fmt.Sprintf("FATAL: invalid SampleRate: %d. Must be > 0 and < 655350.", sib.SampleRate)
	}
	sib.Channels = uint8((chMask&bits)>>41) + 1
	sib.BitsPerSample = uint8((bitsPerSampMask&bits)>>36) + 1
	sib.TotalSamples = bits & totSampMask

	sig := buf.Next(STREAMINFO_MD5_LEN / 8)
	if len(sig) != STREAMINFO_MD5_LEN/8 {
		return false, fmt.Sprintf("FATAL: error reading MD5Signature. Expected %d byte(s), got %d.", STREAMINFO_MD5_LEN/8, len(sig))
	}
	sib.MD5Signature = fmt.Sprintf("%x", sig)

	return true, ""
}

// FLACParseVorbisCommentBlock parses the bits in a Vorbis comment block.
func (vcb *FLACVorbisCommentBlock) FLACParseVorbisCommentBlock(block []byte) (bool, string) {
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

	buf := bytes.NewBuffer(block)

	vendorLen := int(binary.LittleEndian.Uint32(buf.Next(VORBIS_COMMENT_VENDOR_LEN / 8)))
	vcb.Vendor = string(buf.Next(vendorLen))

	vcb.TotalComments = binary.LittleEndian.Uint32(buf.Next(VORBIS_COMMENT_USER_COMMENT_LIST_LEN / 8))

	for tc := vcb.TotalComments; tc > 0; tc-- {
		commentLen := int(binary.LittleEndian.Uint32(buf.Next(VORBIS_COMMENT_COMMENT_LENGTH_LEN / 8)))
		comment := string(buf.Next(commentLen))
		vcb.Comments = append(vcb.Comments, comment)
	}
	return true, ""
}

// Implement fmt.String() for a FLAC Aplication block.
func (data *FLACApplication) String() string {
	var s string

	s += fmt.Sprintf("%s\n", data.Header)
	s += fmt.Sprintf("  app. id: %s\n", data.Data)

	return s
}

// Implement fmt.String() for a FLAC Cue Sheet block.
func (data *FLACCuesheetBlock) String() string {
	var s string
	var catNumber string

	for _, v := range data.MediaCatalogNumber {
		if string(v) != "\x00" {
			catNumber += string(v)
		}
	}

	s += fmt.Sprintf("  media catalog number: %s\n", catNumber)
	s += fmt.Sprintf("  lead-in: %d\n", data.LeadinSamples)
	s += fmt.Sprintf("  is CD: %t\n", data.IsCompactDisc)
	s += fmt.Sprintf("  total tracks: %d\n", data.TotalTracks)
	for i, v := range data.FLACCuesheetTracks {
		s += fmt.Sprintf("    track[%d]\n", i)
		s += fmt.Sprintf("%s", v)
	}
	return s
}

// Implement fmt.String() for a FLAC Cue Sheet Track block.
func (data *FLACCuesheetTrackBlock) String() string {
	var (
		s     string
		ttype string
		isrc  string
	)

	for _, v := range data.TrackISRC {
		if string(v) != "\x00" {
			isrc += string(v)
		}
	}

	if data.TrackType == 0 {
		ttype = "AUDIO"
	} else {
		ttype = "NON-AUDIO"
	}

	s += fmt.Sprintf("      offset: %d\n", data.TrackOffset)
	s += fmt.Sprintf("      number: %d\n", data.TrackNumber)
	s += fmt.Sprintf("      ISRC: %s\n", isrc)
	s += fmt.Sprintf("      type: %s\n", ttype)
	s += fmt.Sprintf("      pre-emphasis: %t\n", data.PreEmphasis)
	s += fmt.Sprintf("      number of index points: %d\n", data.IndexPoints)
	for i, v := range data.FLACCuesheetTrackIndexes {
		s += fmt.Sprintf("      index[%d]\n", i)
		s += fmt.Sprintf("%s", v)
	}
	return s
}

// Implement fmt.String for a FLAC Cue Sheet Track Index block.
func (data *FLACCuesheetTrackIndexBlock) String() string {
	var s string

	s += fmt.Sprintf("        offset: %d\n", data.SampleOffset)
	s += fmt.Sprintf("        number: %d\n", data.IndexPoint)

	return s
}

// Implement fmt.String() for a FLAC Metadata Block Header block.
func (header *FLACMetadataBlockHeader) String() string {
	var s string

	s += fmt.Sprintf("type: %#v\n", header.Type)
	s += fmt.Sprintf("ls last: %t\n", header.Last)
	s += fmt.Sprintf("length: %d\n", header.Length)
	if header.SeekPoints != 0 {
		s += fmt.Sprintf("  seekpoints: %d\n", header.SeekPoints)
	}

	return s
}

// Implement fmt.String() for a FLAC Picture block.
func (data *FLACPictureBlock) String() string {
	var s string

	s += fmt.Sprintf("  type: %s\n", data.PictureType)
	s += fmt.Sprintf("  MIME type: %s\n", data.MimeType)
	s += fmt.Sprintf("  description: %s\n", data.PictureDescription)
	s += fmt.Sprintf("  width: %d\n", data.Width)
	s += fmt.Sprintf("  height: %d\n", data.Height)
	s += fmt.Sprintf("  depth: %d\n", data.ColorDepth)
	s += fmt.Sprintf("  colors: %d\n", data.NumColors)
	s += fmt.Sprintf("  data length: %d\n", data.Length)
	s += fmt.Sprintf("  data:\n")
	for _, v := range strings.Split(data.PictureBlob, "\n") {
		s += fmt.Sprintf("    %s\n", v)
	}

	return s
}

// Implement fmt.String() for a FLAC Seekpoint block.
func (data *FLACSeekpointBlock) String() string {
	var s string

	s += fmt.Sprintf("   sample: %8d offset: %8d frame samples: %8d\n", data.SampleNumber, data.Offset, data.FrameSamples)

	return s
}

// Implement fmt.String() for a FLAC Streaminfo block.
func (data *FLACStreaminfoBlock) String() string {
	var s string

	if data.MinBlockSize == 0 {
		s += fmt.Sprintf("  minimum blocksize: %s\n", "unknown")
	} else {
		s += fmt.Sprintf("  minimum blocksize: %d samples\n", data.MinBlockSize)
	}
	if data.MaxBlockSize == 0 {
		s += fmt.Sprintf("  maximum blocksize: %s\n", "unknown")
	} else {
		s += fmt.Sprintf("  maximum blocksize: %d samples\n", data.MaxBlockSize)
	}
	if data.MinFrameSize == 0 {
		s += fmt.Sprintf("  minimum framesize: %s\n", "unknown")
	} else {
		s += fmt.Sprintf("  minimum framesize: %d bytes\n", data.MinFrameSize)
	}
	if data.MaxFrameSize == 0 {
		s += fmt.Sprintf("  maximum framesize: %s\n", "unknown")
	} else {
		s += fmt.Sprintf("  maximum framesize: %d bytes\n", data.MaxFrameSize)
	}
	s += fmt.Sprintf("  sample_rate: %d\n", data.SampleRate)
	s += fmt.Sprintf("  channels: %d\n", data.Channels)
	s += fmt.Sprintf("  bits-per-sample: %d\n", data.BitsPerSample)
	s += fmt.Sprintf("  total samples: %d\n", data.TotalSamples)
	s += fmt.Sprintf("  MD5 signature: %s\n", data.MD5Signature)

	return s
}

// Implement fmt.String() for a FLAC Vorbis Comment Block block.
func (data *FLACVorbisCommentBlock) String() string {
	var s string

	s += fmt.Sprintf("   vendor: %s\n", data.Vendor)
	s += fmt.Sprintf("   comments: %d\n", data.TotalComments)
	for i, v := range data.Comments {
		s += fmt.Sprintf("   comment[%d]: %s\n", i, v)
	}

	return s
}

// Implement fmt.String() for a FLAC Vorbis Comment block.
func (data *FLACVorbisComment) String() string {
	var s string

	s += fmt.Sprintf("%s\n", data.Header)
	s += fmt.Sprintf("%s\n", data.Data)

	return s
}

// Implement fmt.String for a full FLACMetadata struct.
func (data *FLACMetadata) String() string {
	var s string

	if data.FLACStreaminfo.Header != nil && data.FLACStreaminfo.Data != nil {
		s += fmt.Sprintf("%s", data.FLACStreaminfo.Header)
		s += fmt.Sprintf("%s", data.FLACStreaminfo.Data)
	}

	if data.FLACVorbisComment.Header != nil && data.FLACVorbisComment.Data != nil {
		s += fmt.Sprintf("%s", data.FLACVorbisComment.Header)
		s += fmt.Sprintf("%s", data.FLACVorbisComment.Data)
	}

	if data.FLACCuesheet.Header != nil && data.FLACCuesheet.Data != nil {
		s += fmt.Sprintf("%s", data.FLACCuesheet.Header)
		s += fmt.Sprintf("%s", data.FLACCuesheet.Data)
	}

	for _, p := range data.FLACPictures {
		s += fmt.Sprintf("%s", p.Header)
		s += fmt.Sprintf("%s", p.Data)
	}

	if data.FLACSeektable.Header != nil && data.FLACSeektable.Data != nil {
		s += fmt.Sprintf("%s", data.FLACSeektable.Header)
		for _, sp := range data.FLACSeektable.Data {
			s += fmt.Sprintf("%s", sp)
		}
	}

	return s
}

// ReadFLACMetatada reads the metadata from a FLAC file and populates a FLACMetadata struct.
func (flacmetadata *FLACMetadata) Read(f io.Reader) (bool, string) {
	// First 4 bytes of the file are the FLAC stream marker: 0x66, 0x4C, 0x61, 0x43
	// It's also the length of all metadata block headers so we'll resue it below.
	headerBuf := make([]byte, METADATA_BLOCK_HEADER_LEN/8)

	readlen, readerr := f.Read(headerBuf)
	if readerr != nil || readlen != int(METADATA_BLOCK_HEADER_LEN/8) {
		return false, fmt.Sprintf("FATAL: error reading FLAC signature: %s", readerr)
	}

	if string(headerBuf) != FLAC_SIGNATURE {
		return false, fmt.Sprintf("FATAL: FLAC signature not found")
	}

	for totalMBH := 0; ; totalMBH++ {
		// Next 4 bytes after the stream marker is the first metadata block header.
		readlen, readerr := f.Read(headerBuf)
		if readerr != nil || readlen != int(METADATA_BLOCK_HEADER_LEN/8) {
			return false, fmt.Sprintf("FATAL: error reading metadata block header from: %s", readerr)
		}

		mbh := new(FLACMetadataBlockHeader)
		ok, err := mbh.FLACParseMetadataBlockHeader(headerBuf)
		if !ok {
			return false, err
		}

		block := make([]byte, mbh.Length)
		readlen, readerr = f.Read(block)
		if readerr != nil || readlen != int(len(block)) {
			return false, fmt.Sprintf("FATAL: only read %d of %d bytes for %s metadata block: %s", readlen, mbh.Length, mbh.Type, readerr)
		}

		switch mbh.Type {
		case STREAMINFO:
			if flacmetadata.FLACStreaminfo.IsPopulated {
				return false, fmt.Sprintf("FATAL: Two %s blocks encountered.", mbh.Type)
			}

			sib := new(FLACStreaminfoBlock)
			ok, err := sib.FLACParseStreaminfoBlock(block)
			if !ok {
				return false, err
			}

			flacmetadata.FLACStreaminfo = FLACStreaminfo{mbh, sib, true}

		case VORBIS_COMMENT:
			if flacmetadata.FLACVorbisComment.IsPopulated {
				return false, fmt.Sprintf("FATAL: Two %s blocks encountered.", mbh.Type)
			}

			vcb := new(FLACVorbisCommentBlock)
			ok, err := vcb.FLACParseVorbisCommentBlock(block)
			if !ok {
				return false, err
			}

			flacmetadata.FLACVorbisComment = FLACVorbisComment{mbh, vcb, true}

		case PICTURE:
			fpb := new(FLACPictureBlock)
			ok, err := fpb.FLACParsePictureBlock(block)
			if !ok {
				return false, err
			}
			flacmetadata.FLACPictures = append(flacmetadata.FLACPictures, &FLACPicture{mbh, fpb, true})

		case PADDING:
			if flacmetadata.FLACPadding.IsPopulated {
				return false, fmt.Sprintf("FATAL: Two %s blocks encountered.", mbh.Type)
			}
			flacmetadata.FLACPadding = FLACPadding{mbh, nil, true}

		case APPLICATION:
			if flacmetadata.FLACApplication.IsPopulated {
				return false, fmt.Sprintf("FATAL: Two %s blocks encountered.", mbh.Type)
			}

			fab := new(FLACApplicationBlock)
			ok, err := fab.FLACParseApplicationBlock(block)
			if !ok {
				return false, err
			}
			flacmetadata.FLACApplication = FLACApplication{mbh, fab, true}

		case SEEKTABLE:
			if flacmetadata.FLACSeektable.IsPopulated {
				return false, fmt.Sprintf("FATAL: Two %s blocks encountered.", mbh.Type)
			}
			if len(block)%(SEEKPOINT_BLOCK_LEN/8) != 0 {
				return false, fmt.Sprintf("FATAL: %s block length is not a multiple of %d.", mbh.Type, (SEEKPOINT_BLOCK_LEN / 8))
			}

			flacmetadata.FLACSeektable.FLACParseSeekpointBlock(block)
			flacmetadata.FLACSeektable.Header = mbh
			flacmetadata.FLACSeektable.IsPopulated = true

		case CUESHEET:
			if flacmetadata.FLACCuesheet.IsPopulated {
				return false, fmt.Sprintf("FATAL: Two %s blocks encountered.", mbh.Type)
			}

			csb := new(FLACCuesheetBlock)
			ok, err := csb.FLACParseCuesheetBlock(block)
			if !ok {
				return false, err
			}
			flacmetadata.FLACCuesheet = FLACCuesheet{mbh, csb, true}

			//flacmetadata.FLACCuesheet.Header = mbh
			//flacmetadata.FLACCuesheet.Data = csb
			//lacmetadata.FLACCuesheet.IsPopulated = true

		default:
			continue
		}

		if mbh.Last {
			break
		}
	}
	return true, ""
}
