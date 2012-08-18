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

// TODO: make NewZZZ functions to create Header+Data blocks
import (
	"bytes"
	"encoding/binary"
	"encoding/hex"
	"fmt"
	"io"
)

// METADATA_BLOCK_TYPES enumerates types of metadata blocks in a FLAC file.
type MetadataBlockType uint32

const (
	FlacSignature = "fLaC"

	MetadataStreaminfo    MetadataBlockType = iota // 0
	MetadataPadding                                // 1
	MetadataApplication                            // 2
	MetadataSeektable                              // 3
	MetadataVorbisComment                          // 4
	MetadataCuesheet                               // 5
	MetadataPicture                                // 6
	MetadataInvalid       MetadataBlockType = 127

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

// PictureTypeMap enumerates the types of pictures in a PictureBlock.
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
func LookupHeaderType(i uint32) MetadataBlockType {
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

// Implement fmt.GoStringer() to print the string and integer representation of METADATA_BLOCK_TYPEs.
func (mbt MetadataBlockType) GoString() string {
	return fmt.Sprintf("%d (%s)", int(mbt), mbt)
}

// Begin base metadata block types.

// ApplicationBlock contains the ID and binary data of an embedded executable.
// Only one ApplicationBlock is allowed per file.
type ApplicationBlock struct {
	Id   uint32
	Data []byte
}

type CuesheetBlock struct {
	MediaCatalogNumber string
	LeadinSamples      uint64
	IsCompactDisc      bool
	//Reserved           []byte
	TotalTracks    uint8 // > 1 && < 100 (for CD-DA)
	CuesheetTracks []*CuesheetTrackBlock
}

type CuesheetTrackBlock struct {
	TrackOffset uint64
	TrackNumber uint8
	TrackISRC   string
	TrackType   uint8
	PreEmphasis bool
	// Reserved             []byte // 6 + 13 * 8
	IndexPoints          uint8
	CuesheetTrackIndexes []*CuesheetTrackIndexBlock
}

type CuesheetTrackIndexBlock struct {
	SampleOffset uint64
	IndexPoint   uint8
	//Reserved     []byte // 3 * 8 bits. All bits must be set to zero.
}

// MetadataBlockHeader is the common element for every metadata block in a
// FLAC file. It describes the metadata block type, its length (in bytes), the
// number of seek points if the block is a seektable, and if it is the last
// metadata block before the start of the audio stream.
type MetadataBlockHeader struct {
	Type       MetadataBlockType
	Length     uint32
	Last       bool
	SeekPoints uint16
}

// PictureBlock contains information and binary data about pictures that
// are embedded in the FLAC file. Muitiple PictureBlocks are allow per file.
type PictureBlock struct {
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

// SeekpointBlock contains locations within the FLAC file that allow
// an application to quickly jump to pre-defined locations in the audio stream.
type SeekpointBlock struct {
	SampleNumber uint64
	Offset       uint64
	FrameSamples uint16
}

// StreaminfoBlock contains information about the audio stream.
// Only one StreaminfoBlock is allowed per file. It is also the only required block.
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

// VorbisCommentBlock contains general information about the song/audio stream.
// Common fields are Artist, Song Title and Album.
// Only one VorbisCommentBlock is allowed per file.
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

// Cuesheet describes a "Cue sheet".
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

// Begin ParseX functions.

// Parse parses the bits of a FLAC Application block.
func (ab *ApplicationBlock) Parse(block []byte) error {
	// http://flac.sourceforge.net/format.html#metadata_block_application
	// Field Len  | Data
	// -----------+--------------------------------------------------------
	// 32         | Registered application ID.
	//            |
	// n          | Application data (n must be a multiple of 8)

	buf := bytes.NewBuffer(block)

	ab.Id = binary.BigEndian.Uint32(buf.Next(ApplicationIdLen))
	if buf.Len()%8 != 0 {
		return fmt.Errorf("Malformed ApplictionBlock: the data field length must be a mulitple of 8.")
	}
	ab.Data = buf.Bytes()
	return nil
}

// Parse parses the bits of a FLAC Cue Sheet block.
func (cb *CuesheetBlock) Parse(block []byte) error {
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

	const trackType = 0x01
	buf := bytes.NewBuffer(block)

	cb.MediaCatalogNumber = string(buf.Next(CuesheetMediaCatalogNumberLen / 8))
	cb.LeadinSamples = binary.BigEndian.Uint64(buf.Next(CuesheetLeadinSamplesLen / 8))

	res := buf.Next(CuesheetReservedLen / 8)

	if res[0]>>7&trackType == 1 {
		cb.IsCompactDisc = true
	}

	cb.TotalTracks = uint8(buf.Next(CuesheetTotalTracksLen / 8)[0])

	if cb.TotalTracks < 1 {
		return fmt.Errorf("FATAL: CuesheetBlock.TotalTracks value must be greater than >= 1.")
	}

	for i := 0; i < int(cb.TotalTracks); i++ {
		cb.ParseTrack(buf.Next(CuesheetTrackBlockLen / 8))
		for j := 0; j < int(cb.CuesheetTracks[i].IndexPoints); j++ {
			cb.CuesheetTracks[i].ParseIndex(buf.Next(CuesheetTrackIndexBlockLen / 8))
		}
	}

	return nil
}

// Parse parses the bits of a FLAC Cue Sheet Track block.
func (cb *CuesheetBlock) ParseTrack(block []byte) error {
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

	// TODO Implement error checking here.
	const trackType = 0x01
	buf := bytes.NewBuffer(block)

	ctb := new(CuesheetTrackBlock)

	ctb.TrackOffset = binary.BigEndian.Uint64(buf.Next(CuesheetTrackTrackOffsetLen / 8))

	ctb.TrackNumber = uint8(buf.Next(CuesheetTrackTrackNumberLen / 8)[0])
	if ctb.TrackNumber == 0 {
		return fmt.Errorf("FATAL: Cuesheet track value of 0 is not allowed.")
	}

	ctb.TrackISRC = string(buf.Next(CuesheetTrackTrackISRCLen / 8))

	// The first byte of the reserved field contain the flags for track type and preemphasis.
	res := buf.Next(CuesheetTrackReservedLen / 8)[0]

	ctb.TrackType = uint8(res >> 7 & trackType)

	if res>>6&trackType == 1 {
		ctb.PreEmphasis = true
	}

	ctb.IndexPoints = uint8(buf.Next(CuesheetTrackIndexPointsLen / 8)[0])

	cb.CuesheetTracks = append(cb.CuesheetTracks, ctb)

	return nil
}

// Parse parses the bits of a Cue Sheet Track Index block.
func (ctb *CuesheetTrackBlock) ParseIndex(block []byte) error {
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

	cti := new(CuesheetTrackIndexBlock)

	cti.SampleOffset = binary.BigEndian.Uint64(buf.Next(CuesheetTrackIndexSampleOffsetLen / 8))
	if cti.SampleOffset%588 != 0 {
		return fmt.Errorf("Invalid value '%d' for Cuesheet Track Index Sample Offset: must be divisible by 588.", cti.SampleOffset)
	}

	cti.IndexPoint = uint8(buf.Next(CuesheetTrackIndexPointLen / 8)[0])
	ctb.CuesheetTrackIndexes = append(ctb.CuesheetTrackIndexes, cti)

	return nil
}

// Parse parses the bits of a FLAC Metadata Block Header.
func (mbh *MetadataBlockHeader) Parse(block []byte) error {
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

	const (
		lastBlock = 0x80000000
		blockType = 0x7F000000
		blockLen  = 0x00FFFFFF
	)

	bits := binary.BigEndian.Uint32(block)

	if (lastBlock&bits)>>31 == 1 {
		mbh.Last = true
	}

	bt := blockType & bits >> 24
	mbh.Type = LookupHeaderType(bt)
	if mbh.Type == MetadataInvalid {
		return fmt.Errorf("FATAL: Encountered an invalid or unknown block type: %d.", bt)
	}
	mbh.Length = blockLen & bits

	if mbh.Type == MetadataSeektable {
		if mbh.Length%(SeekpointBlockLen/8) != 0 {
			return fmt.Errorf("FATAL: Seektable block length is not a multiple of %d.", SeekpointBlockLen/8)
		}
		mbh.SeekPoints = uint16(mbh.Length / (SeekpointBlockLen / 8))
	}
	return nil
}

// Parse parses the bits of a FLAC picture block.
func (pb *PictureBlock) Parse(block []byte) error {
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

	// TODO: Add error checking here.
	buf := bytes.NewBuffer(block)

	pb.PictureType = LookupPictureType(binary.BigEndian.Uint32(buf.Next(PictureTypeLen / 8)))

	len := binary.BigEndian.Uint32(buf.Next(PictureMimeLengthLen / 8))
	pb.MimeType = string(buf.Next(int(len)))

	len = binary.BigEndian.Uint32(buf.Next(PictureDescriptionLengthLen / 8))
	if len > 0 {
		pb.PictureDescription = string(buf.Next(int(len)))
	} else {
		pb.PictureDescription = ""
	}
	pb.Width = binary.BigEndian.Uint32(buf.Next(PictureWidthLen / 8))
	pb.Height = binary.BigEndian.Uint32(buf.Next(PictureHeightLen / 8))
	pb.ColorDepth = binary.BigEndian.Uint32(buf.Next(PictureColorDepthLen / 8))
	pb.NumColors = binary.BigEndian.Uint32(buf.Next(PictureNumberOfColorsLen / 8))
	pb.Length = binary.BigEndian.Uint32(buf.Next(PictureLengthLen / 8))
	pb.PictureBlob = hex.Dump(buf.Next(int(pb.Length)))

	return nil
}

// Parse parses the bits of a FLAC seekpoint block.
func (stb *Seektable) Parse(block []byte) error {
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

	// TODO: Add error checking here.
	buf := bytes.NewBuffer(block)

	for i := 0; buf.Len() > 0; i++ {
		spb := new(SeekpointBlock)
		binary.Read(buf, binary.BigEndian, spb)

		// These got replaced by binary.Read
		//spb.SampleNumber = binary.BigEndian.Uint64(buf.Next(SeekpointSampleLen / 8))
		//spb.Offset = binary.BigEndian.Uint64(buf.Next(SeekpointSampleOffsetLen / 8))
		//spb.FrameSamples = binary.BigEndian.Uint16(buf.Next(SeekpointTargetFrameSamplesLen / 8))

		stb.Data = append(stb.Data, spb)
	}
	return nil
}

// Parse parses the bits of a FLAC streaminfo block.
func (sib *StreaminfoBlock) Parse(block []byte) error {
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

	const (
		minFSMask       = 0xFFFFFFFFFFFFFFFF
		maxFSMask       = 0xFFFFFF
		sampRateMask    = 0xFFFFF00000000000
		bitsPerSampMask = 0x1F000000000
		chMask          = 0xE0000000000
		totSampMask     = 0xFFFFFFFFF
	)
	var bits uint64

	buf := bytes.NewBuffer(block)

	mbs := buf.Next(StreaminfoMinBlockSizeLen / 8)
	if len(mbs) != StreaminfoMinBlockSizeLen/8 {
		return fmt.Errorf("FATAL: error reading MinBlockSize field. Expected %d byte(s), got %d.", StreaminfoMinBlockSizeLen/8, len(mbs))
	}
	sib.MinBlockSize = binary.BigEndian.Uint16(mbs)
	if sib.MinBlockSize > 0 && sib.MinBlockSize < 16 {
		return fmt.Errorf("FATAL: invalid MinBlockSize '%d'. Must be >= 16.", sib.MinBlockSize)
	}

	bfsLen := (StreaminfoMaxBlockSizeLen + StreaminfoMinFrameSizeLen + StreaminfoMaxFrameSizeLen) / 8

	bits = binary.BigEndian.Uint64(buf.Next(bfsLen))
	sib.MaxBlockSize = uint16((minFSMask & bits) >> 48)
	if sib.MaxBlockSize < 16 {
		return fmt.Errorf("FATAL: invalid MaxBlockSize '%d'. Must be > 16.", sib.MaxBlockSize)
	}
	sib.MinFrameSize = uint32((minFSMask & bits) >> 24)
	sib.MaxFrameSize = uint32(maxFSMask & bits)

	bits = binary.BigEndian.Uint64(buf.Next((StreaminfoSampleRateLen +
		StreaminfoChannelCountLen +
		StreaminfoBitsPerSampleLen +
		StreaminfoTotalSamplesLen) / 8))

	sib.SampleRate = uint32((sampRateMask & bits) >> 44)
	if sib.SampleRate == 0 || sib.SampleRate >= 655350 {
		return fmt.Errorf("FATAL: invalid SampleRate: %d. Must be > 0 and < 655350.", sib.SampleRate)
	}
	sib.Channels = uint8((chMask&bits)>>41) + 1
	sib.BitsPerSample = uint8((bitsPerSampMask&bits)>>36) + 1
	sib.TotalSamples = bits & totSampMask

	sig := buf.Next(StreaminfoMD5Len / 8)
	sib.MD5Signature = fmt.Sprintf("%x", sig)

	return nil
}

// Parse parses the bits in a Vorbis comment block.
func (vcb *VorbisCommentBlock) Parse(block []byte) error {
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

	// TODO: Add error checking here
	buf := bytes.NewBuffer(block)

	len := binary.LittleEndian.Uint32(buf.Next(VorbisCommentVendorLen / 8))
	vcb.Vendor = string(buf.Next(int(len)))

	vcb.TotalComments = binary.LittleEndian.Uint32(buf.Next(VorbisCommentUserCommentLen / 8))

	for tc := vcb.TotalComments; tc > 0; tc-- {
		len := binary.LittleEndian.Uint32(buf.Next(VorbisCommentCommentLengthLen / 8))
		comment := string(buf.Next(int(len)))
		vcb.Comments = append(vcb.Comments, comment)
	}
	return nil
}

// ReadFLACMetatada reads the metadata from a FLAC file and populates a Metadata struct.
func (meta *Metadata) Read(f io.Reader) error {
	// First 4 bytes of the file are the FLAC stream marker: 0x66, 0x4C, 0x61, 0x43
	// It's also the length of all metadata block headers so we'll resue it below.
	h := make([]byte, MetadataBlockHeaderLen/8)

	n, err := io.ReadFull(f, h)
	if err != nil || n != int(MetadataBlockHeaderLen/8) {
		return fmt.Errorf("FATAL: error reading FLAC signature: %s", err)
	}

	if string(h) != FlacSignature {
		return fmt.Errorf("FATAL: '%s' is not a valid FLAC signature.", string(h))
	}

	for totalMBH := 0; ; totalMBH++ {
		// Next 4 bytes after the stream marker is the first metadata block header.
		n, err := io.ReadFull(f, h)
		if err != nil || n != int(MetadataBlockHeaderLen/8) {
			return fmt.Errorf("FATAL: error reading metadata block header: %s", err)
		}

		mbh := new(MetadataBlockHeader)
		err = mbh.Parse(h)
		if err != nil {
			return err
		}

		block := make([]byte, mbh.Length)
		n, err = io.ReadFull(f, block)
		if err != nil || n != int(len(block)) {
			return fmt.Errorf("FATAL: read %d of %d bytes for %s metadata block: %s", n, mbh.Length, mbh.Type, err)
		}

		switch mbh.Type {
		case MetadataStreaminfo:
			if meta.Streaminfo.IsPopulated {
				return fmt.Errorf("FATAL: Two %s blocks encountered.", mbh.Type)
			}

			sib := new(StreaminfoBlock)
			err := sib.Parse(block)
			if err != nil {
				return err
			}

			meta.Streaminfo = Streaminfo{mbh, sib, true}

		case MetadataVorbisComment:
			if meta.VorbisComment.IsPopulated {
				return fmt.Errorf("FATAL: Two %s blocks encountered.", mbh.Type)
			}

			vcb := new(VorbisCommentBlock)
			err := vcb.Parse(block)
			if err != nil {
				return err
			}

			meta.VorbisComment = VorbisComment{mbh, vcb, true}

		case MetadataPicture:
			fpb := new(PictureBlock)
			err := fpb.Parse(block)
			if err != nil {
				return err
			}
			meta.Pictures = append(meta.Pictures, &Picture{mbh, fpb, true})

		case MetadataPadding:
			if meta.Padding.IsPopulated {
				return fmt.Errorf("FATAL: Two %s blocks encountered.", mbh.Type)
			}
			meta.Padding = Padding{mbh, nil, true}

		case MetadataApplication:
			if meta.Application.IsPopulated {
				return fmt.Errorf("FATAL: Two %s blocks encountered.", mbh.Type)
			}

			fab := new(ApplicationBlock)
			err := fab.Parse(block)
			if err != nil {
				return err
			}
			meta.Application = Application{mbh, fab, true}

		case MetadataSeektable:
			if meta.Seektable.IsPopulated {
				return fmt.Errorf("FATAL: Two %s blocks encountered.", mbh.Type)
			}
			if len(block)%(SeekpointBlockLen/8) != 0 {
				return fmt.Errorf("FATAL: %s block length is not a multiple of %d.", mbh.Type, (SeekpointBlockLen / 8))
			}

			err := meta.Seektable.Parse(block)
			if err != nil {
				return err
			}
			meta.Seektable.Header = mbh
			meta.Seektable.IsPopulated = true

		case MetadataCuesheet:
			if meta.Cuesheet.IsPopulated {
				return fmt.Errorf("FATAL: Two %s blocks encountered.", mbh.Type)
			}

			csb := new(CuesheetBlock)
			err := csb.Parse(block)
			if err != nil {
				return err
			}
			meta.Cuesheet = Cuesheet{mbh, csb, true}

		default:
			continue
		}

		if mbh.Last {
			break
		}
	}
	return nil
}
