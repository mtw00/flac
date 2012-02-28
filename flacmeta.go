// Package flacmeta provides an API to extract and print metadata from FLAC
// audio files.
package flacmeta

// TODO:
// 	FLACParseCuesheetBlock()
//	FLACParseCuesheetTrack()

import (
	"bytes"
	"encoding/binary"
	"encoding/hex"
	"fmt"
	"os"
	"strings"
)

// METADATA_BLOCK_TYPES enumerates the types of metadata blocks in a FLAC file.
type METADATA_BLOCK_TYPE uint32

const (
	STREAMINFO     METADATA_BLOCK_TYPE = iota // 0
	PADDING                                   // 1
	APPLICATION                               // 2
	SEEKTABLE                                 // 3
	VORBIS_COMMENT                            // 4
	CUESHEET                                  // 5
	PICTURE                                   // 6
	INVALID        METADATA_BLOCK_TYPE = 127

	METADATA_BLOCK_HEADER_LEN int = 32

	// Metadata field sizes, in bits.
	APPLICATION_ID_LEN int = 32

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
	SEEKPOINT_BLOCK_LEN		   int = (SEEKPOINT_SAMPLE_NUMBER_LEN +
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

// Begin convenience functions.

// LookupHeaderType returns a const representing the METADATA_BLOCK_TYPE or
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
	Data        []byte
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
	FLACCuesheet  // not implemented yet
	TotalBlocks   uint8
}

// Begin FLACParseX functions.

// FLACParseApplicationBlock parses the bits from an application block.
func (ab *FLACApplicationBlock) FLACParseApplicationBlock(block []byte) (bool, string) {
	buf := bytes.NewBuffer(block)

	ab.Id = binary.BigEndian.Uint32(buf.Next(APPLICATION_ID_LEN))
	if buf.Len()%8 != 0 {
		return true, "Malformed METADATA_BLOCK_APPLICATION: the data field length is not a mulitple of 8."
	}
	ab.Data = buf.Bytes()
	return false, ""
}

// FLACParseMetadataBlockHeader parses the bits from a FLAC metadata block header.
func (mbh *FLACMetadataBlockHeader) FLACParseMetadataBlockHeader(block []byte) (bool, string) {
	var LASTBLOCK uint32 = 0x80000000
	var BLOCKTYPE uint32 = 0x7F000000
	var BLOCKLEN uint32 = 0x00FFFFFF

	bits := binary.BigEndian.Uint32(block)

	blktype := BLOCKTYPE & bits >> 24
	mbh.Type = LookupHeaderType(blktype)
	if mbh.Type == INVALID {
		return true, fmt.Sprintf("FATAL: Encountered an invalid or unknown block type: %d.", blktype)
	}
	mbh.Length = BLOCKLEN & bits
	mbh.Last = false
	if (LASTBLOCK&bits)>>31 == 1 {
		mbh.Last = true
	}
	if mbh.Type == SEEKTABLE {
		if mbh.Length%18 != 0 {
			return true, "SEEKTABLE block length is not a multiple of 18."
		}
		mbh.SeekPoints = uint16(mbh.Length / 18)
	}
	return false, ""
}

// FLACParsePictureBlock parses the bits from a picture block.
func (pb *FLACPictureBlock) FLACParsePictureBlock(block []byte) {
	// bits   Field
	// -----+------
	// <32>   The picture type according to the ID3v2 APIC frame
	// <32>   The length of the MIME type string in bytes.
	// <n*8>  The MIME type string, in printable ASCII characters 0x20-0x7e. The MIME type may also be --> to signify that the data part is a URL of the picture instead of the picture data itself.
	// <32>   The length of the description string in bytes.
	// <n*8>  The description of the picture, in UTF-8.
	// <32>   The width of the picture in pixels.
	// <32>   The height of the picture in pixels.
	// <32>   The color depth of the picture in bits-per-pixel.
	// <32>   For indexed-color pictures (e.g. GIF), the number of colors used, or 0 for non-indexed pictures.
	// <32>   The length of the picture data in bytes.
	// <n*8>  The binary picture data.

	buf := bytes.NewBuffer(block)

	pb.PictureType = LookupPictureType(binary.BigEndian.Uint32(buf.Next(PICTURE_TYPE_LEN / 8)))

	mimeLen := int(binary.BigEndian.Uint32(buf.Next(PICTURE_MIME_LENGTH_LEN / 8)))
	pb.MimeType = string(buf.Next(mimeLen))

	descLen := int(binary.BigEndian.Uint32(buf.Next(PICTURE_DESCRIPTION_LENGTH_LEN / 8)))
	if descLen > 0 {
		pb.PictureDescription = string(binary.BigEndian.Uint32(buf.Next(descLen)))
	} else {
		pb.PictureDescription = ""
	}
	pb.Width = binary.BigEndian.Uint32(buf.Next(PICTURE_LENGTH_LEN / 8))
	pb.Height = binary.BigEndian.Uint32(buf.Next(PICTURE_HEIGHT_LEN / 8))
	pb.ColorDepth = binary.BigEndian.Uint32(buf.Next(PICTURE_COLOR_DEPTH_LEN / 8))
	pb.NumColors = binary.BigEndian.Uint32(buf.Next(PICTURE_NUMBER_OF_COLORS_LEN / 8))
	pb.Length = binary.BigEndian.Uint32(buf.Next(PICTURE_LENGTH_LEN / 8))
	pb.PictureBlob = hex.Dump(buf.Next(int(pb.Length)))
}

// FLACParseSeekpointBlock parses the bits from a FLAC seekpoint block.
func (spb *FLACSeekpointBlock) FLACParseSeekpointBlock(block []byte) {
	buf := bytes.NewBuffer(block)

	spb.SampleNumber = binary.BigEndian.Uint64(buf.Next(int(SEEKPOINT_SAMPLE_NUMBER_LEN / 8)))
	spb.Offset = binary.BigEndian.Uint64(buf.Next(SEEKPOINT_SAMPLE_OFFSET_LEN / 8))
	spb.FrameSamples = binary.BigEndian.Uint16(buf.Next(SEEKPOINT_TARGET_FRAME_SAMPLES_LEN / 8))
}

// FLACParseStreaminfoBlock parses the bits from a FLAC streaminfo block.
func (sib *FLACStreaminfoBlock) FLACParseStreaminfoBlock(block []byte) {
	// From: http://flac.sourceforge.net/format.html
	// The FLAC STREAMINFO block is structured thus:
	// <16>  - Minimum block size (in samples) used in the stream.
	// <16>  - Maximum block size (in samples) used in the stream.
	// <24>  - Minimum frame size (in bytes) used in the stream. 0 == Implied Unknown
	// <24>  - Maximum frame size (in bytes) used in the stream. 0 == Implied Unknown
	// <20>  - Sample rate (in Hz). Must be > 0 && < 655350
	// <3>   - Number of channels - 1. Why -1?
	// <5>   - Bits per sample - 1. Why -1?
	// <36>  - Total number of samples in the stream. 0 == Implied Unknown
	// <128> - MD5 signature of the unencoded audio data.
	//
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

	sib.MinBlockSize = binary.BigEndian.Uint16(buf.Next(STREAMINFO_MIN_BLOCK_SIZE_LEN / 8))

	bits = binary.BigEndian.Uint64(buf.Next((STREAMINFO_MAX_BLOCK_SIZE_LEN +
		STREAMINFO_MIN_FRAME_SIZE_LEN +
		STREAMINFO_MAX_FRAME_SIZE_LEN) / 8))
	sib.MaxBlockSize = uint16((minFSMask & bits) >> 48)
	sib.MinFrameSize = uint32((minFSMask & bits) >> 24)
	sib.MaxFrameSize = uint32(maxFSMask & bits)

	bits = binary.BigEndian.Uint64(buf.Next((STREAMINFO_SAMPLE_RATE_LEN +
		STREAMINFO_CHANNEL_COUNT_LEN +
		STREAMINFO_BITS_PER_SAMPLE_LEN +
		STREAMINFO_TOTAL_SAMPLES_LEN) / 8))
	sib.SampleRate = uint32((sampRateMask & bits) >> 44)
	sib.Channels = uint8((chMask&bits)>>41) + 1
	sib.BitsPerSample = uint8((bitsPerSampMask&bits)>>36) + 1
	sib.TotalSamples = bits & totSampMask

	sib.MD5Signature = fmt.Sprintf("%x", buf.Next(STREAMINFO_MD5_LEN/8))
}

// FLACParseVorbisCommentBlock parses the bits in a Vorbis comment block.
func (vcb *FLACVorbisCommentBlock) FLACParseVorbisCommentBlock(block []byte) {
	// http://www.xiph.org/vorbis/doc/v-comment.html
	// The comment header is decoded as follows:
	//
	//	1) [vendor_length] = read an unsigned integer of 32 bits
	//	2) [vendor_string] = read a UTF-8 vector as [vendor_length] octets
	//	3) [user_comment_list_length] = read an unsigned integer of 32 bits
	//	4) iterate [user_comment_list_length] times {
	//		5) [length] = read an unsigned integer of 32 bits
	//		6) this iteration's user comment = read a UTF-8 vector as [length] octets
	//	}
	//	7) done.

	buf := bytes.NewBuffer(block)

	vendorLen := int(binary.LittleEndian.Uint32(buf.Next(VORBIS_COMMENT_VENDOR_LEN / 8)))
	vcb.Vendor = string(buf.Next(vendorLen))

	vcb.TotalComments = binary.LittleEndian.Uint32(buf.Next(VORBIS_COMMENT_USER_COMMENT_LIST_LEN / 8))

	for tc := vcb.TotalComments; tc > 0; tc-- {
		commentLen := int(binary.LittleEndian.Uint32(buf.Next(VORBIS_COMMENT_COMMENT_LENGTH_LEN / 8)))
		comment := string(buf.Next(commentLen))
		vcb.Comments = append(vcb.Comments, comment)
	}
}

func (data *FLACApplication) String() string {
	var s string
	
	s += fmt.Sprintf("%s\n", data.Header)
	s += fmt.Sprintf("  app. id: %s\n", data.Data)

	return s
}


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
	for _, l := range strings.Split(data.PictureBlob, "\n") {
		s += fmt.Sprintf("    %s\n", l)
	}

	return s
}

func (data *FLACSeekpointBlock) String() string {
	var s string

	s += fmt.Sprintf("   sample: %8d offset: %8d frame samples: %8d\n", data.SampleNumber, data.Offset, data.FrameSamples)

	return s
}

func (data *FLACStreaminfoBlock) String() string {
	var s string
	
	s += fmt.Sprintf("  minimum blocksize: %d samples\n", data.MinBlockSize)
	s += fmt.Sprintf("  maximum blocksize: %d samples\n", data.MaxBlockSize)
	s += fmt.Sprintf("  minimum framesize: %d bytes\n", data.MinFrameSize)
	s += fmt.Sprintf("  maximum framesize: %d bytes\n", data.MaxFrameSize)
	s += fmt.Sprintf("  sample_rate: %d\n", data.SampleRate)
	s += fmt.Sprintf("  channels: %d\n", data.Channels)
	s += fmt.Sprintf("  bits-per-sample: %d\n", data.BitsPerSample)
	s += fmt.Sprintf("  total samples: %d\n", data.TotalSamples)
	s += fmt.Sprintf("  MD5 signature: %s\n", data.MD5Signature)

	return s
}

func (data *FLACVorbisCommentBlock) String() string {
	var s string

	s += fmt.Sprintf("   vendor: %s\n", data.Vendor)
	s += fmt.Sprintf("   comments: %d\n", data.TotalComments)
	for i, v := range data.Comments {
		s += fmt.Sprintf("   comment[%d]: %s\n", i, v)
	}

	return s
}

func (data *FLACVorbisComment) String() string {
	var s string

	s += fmt.Sprintf("%s\n", data.Header)
	s += fmt.Sprintf("%s\n", data.Data)

	return s
}

func (data *FLACMetadata) String() string {
	var s string

	s += fmt.Sprintf("%s", data.FLACStreaminfo.Header)
	s += fmt.Sprintf("%s", data.FLACStreaminfo.Data)

	s += fmt.Sprintf("%s", data.FLACVorbisComment.Header)
	s += fmt.Sprintf("%s", data.FLACVorbisComment.Data)

	for _, p := range data.FLACPictures {
		s += fmt.Sprintf("%s", p.Header)
		s += fmt.Sprintf("%s", p.Data)
	}

	if data.FLACSeektable.Header != nil {
		s += fmt.Sprintf("%s", data.FLACSeektable.Header)
		for _, sp := range data.FLACSeektable.Data {
			s += fmt.Sprintf("%s", sp)
		}
	}

	return s
}

// ReadFLACMetatada reads the metadata from a FLAC file and populates a FLACMetadata struct.
func (flacmetadata *FLACMetadata) ReadFLACMetadata(f *os.File) (bool, string) {
	// First 4 bytes of the file are the FLAC stream marker: 0x66, 0x4C, 0x61, 0x43
	// It's also the length of all metadata block headers so we'll resue it below.
	headerBuf := make([]byte, METADATA_BLOCK_HEADER_LEN/8)

	readlen, readerr := f.Read(headerBuf)
	if readerr != nil || readlen != int(METADATA_BLOCK_HEADER_LEN/8) {
		return true, fmt.Sprintf("FATAL: error reading FLAC signature from '%s': %s", f.Name(), readerr)
	}

	if string(headerBuf) != "fLaC" {
		return true, fmt.Sprintf("FATAL: FLAC signature not found in '%s'", f.Name())
	}

	for totalMBH := 0; ; totalMBH++ {
		// Next 4 bytes after the stream marker is the first metadata block header.
		readlen, readerr := f.Read(headerBuf)
		if readerr != nil || readlen != int(METADATA_BLOCK_HEADER_LEN/8) {
			return true, fmt.Sprintf("FATAL: error reading metadata block header from '%s': %s", f.Name(), readerr)
		}

		mbh := new(FLACMetadataBlockHeader)
		error, msg := mbh.FLACParseMetadataBlockHeader(headerBuf)
		if error {
			return true, msg
		}

		block := make([]byte, int(mbh.Length))
		readlen, readerr = f.Read(block)
		if readerr != nil || readlen != int(len(block)) {
			return true, fmt.Sprintf("FATAL: error reading %s metadata block from '%s': %s", mbh.Type, f.Name(), readerr)
		}

		switch mbh.Type {
		case STREAMINFO:
			if flacmetadata.FLACStreaminfo.IsPopulated {
				return true, fmt.Sprintf("FATAL: Two %s blocks encountered!\n", mbh.Type)
			}
			sib := new(FLACStreaminfoBlock)
			sib.FLACParseStreaminfoBlock(block)
			flacmetadata.FLACStreaminfo = FLACStreaminfo{mbh, sib, true}

		case VORBIS_COMMENT:
			if flacmetadata.FLACVorbisComment.IsPopulated {
				return true, fmt.Sprintf("FATAL: Two %s blocks encountered!\n", mbh.Type)
			}
			vcb := new(FLACVorbisCommentBlock)
			vcb.FLACParseVorbisCommentBlock(block)
			flacmetadata.FLACVorbisComment = FLACVorbisComment{mbh, vcb, true}

		case PICTURE:
			fpb := new(FLACPictureBlock)
			fpb.FLACParsePictureBlock(block)
			flacmetadata.FLACPictures = append(flacmetadata.FLACPictures, &FLACPicture{mbh, fpb, true})

		case PADDING:
			if flacmetadata.FLACPadding.IsPopulated {
				return true, fmt.Sprintf("FATAL: Two %s blocks encountered!\n", mbh.Type)
			}
			flacmetadata.FLACPadding = FLACPadding{mbh, nil, true}

		case APPLICATION:
			if flacmetadata.FLACApplication.IsPopulated {
				return true, fmt.Sprintf("FATAL: Two %s blocks encountered!\n", mbh.Type)
			}
			fab := new(FLACApplicationBlock)
			fab.FLACParseApplicationBlock(block)
			flacmetadata.FLACApplication = FLACApplication{mbh, fab, true}

		case SEEKTABLE:
			if flacmetadata.FLACSeektable.IsPopulated {
				return true, fmt.Sprintf("FATAL: Two %s block encountered!\n", mbh.Type)
			}
			if len(block) % (SEEKPOINT_BLOCK_LEN / 8) != 0 {
				return true, fmt.Sprintf("FATAL: %s block length is not a multiple of %d\n", mbh.Type, (SEEKPOINT_BLOCK_LEN/8))
			}
			
			flacmetadata.FLACSeektable.Header = mbh

			buf := bytes.NewBuffer(block)
			for i := 0 ; buf.Len() > 0 ; i++ {
				spb := new(FLACSeekpointBlock)
				spb.FLACParseSeekpointBlock(buf.Next(SEEKPOINT_BLOCK_LEN/8))
				flacmetadata.FLACSeektable.Data = append(flacmetadata.FLACSeektable.Data, spb)
			}
			
			flacmetadata.FLACSeektable.IsPopulated = true

		default:
			continue
		}

		if mbh.Last {
			break
		}
	}
	return false, ""
}
